// Package metrics 极简 Prometheus 指标登记器（零外部依赖）。
//
// 只做三件事：计数器（HTTP 请求/业务错误码/定时任务）、HTTP 时延直方图、
// 以及抓取时的运行时/依赖探针指标。输出遵循 Prometheus text exposition format 0.0.4，
// 直接挂到 GET /metrics 即可被 Prometheus / VictoriaMetrics / 夜莺 抓取。
//
// 不引入 prometheus/client_golang 的原因：脚手架希望"零依赖可跑"，
// 且这里的指标规模（几十个序列）远小于库带来的复杂度成本。
package metrics

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// latencyBuckets 直方图分桶（秒），覆盖 5ms ~ 10s。
var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// maxSeries 序列上限：路由是白名单（gin FullPath），正常不会超；超了归并到 __other__ 防内存爆。
const maxSeries = 500

type httpSeries struct {
	method string
	route  string
	status string
	count  int64
	sum    float64
	bucket []int64
}

// Registry 指标登记器。
type Registry struct {
	mu       sync.Mutex
	start    time.Time
	http     map[string]*httpSeries
	codes    map[string]int64 // 业务码 -> 次数
	cron     map[string]int64 // handler|result -> 次数
	cronLast map[string]int64 // handler -> 最近一次耗时(ms)
	overflow int64
}

// Default 全局默认登记器。
var Default = New()

// New 创建登记器。
func New() *Registry {
	return &Registry{
		start:    time.Now(),
		http:     make(map[string]*httpSeries, 64),
		codes:    make(map[string]int64, 16),
		cron:     make(map[string]int64, 16),
		cronLast: make(map[string]int64, 16),
	}
}

// ObserveHTTP 记录一次 HTTP 请求。
func (r *Registry) ObserveHTTP(method, route, status string, seconds float64) {
	if r == nil {
		return
	}
	if route == "" {
		route = "unmatched"
	}
	key := method + "|" + route + "|" + status
	r.mu.Lock()
	s, ok := r.http[key]
	if !ok {
		if len(r.http) >= maxSeries {
			r.overflow++
			r.mu.Unlock()
			return
		}
		s = &httpSeries{method: method, route: route, status: status, bucket: make([]int64, len(latencyBuckets))}
		r.http[key] = s
	}
	s.count++
	s.sum += seconds
	for i, b := range latencyBuckets {
		if seconds <= b {
			s.bucket[i]++
		}
	}
	r.mu.Unlock()
}

// IncCode 记录业务响应码分布（code=0 表示成功）。
func (r *Registry) IncCode(code string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.codes[code]++
	r.mu.Unlock()
}

// ObserveCron 记录定时任务执行结果（result: success|failed|skipped）。
func (r *Registry) ObserveCron(handler, result string, cost time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.cron[handler+"|"+result]++
	r.cronLast[handler] = cost.Milliseconds()
	r.mu.Unlock()
}

// Extras 抓取时追加的额外指标（由调用方提供依赖探针）。
type Extras struct {
	Build map[string]string // 版本信息
	MySQL map[string]float64
	Redis float64 // 1 可用 / 0 不可用
}

// Render 输出 Prometheus 文本格式。
func (r *Registry) Render(w io.Writer, ex Extras) {
	if r == nil {
		return
	}
	r.mu.Lock()
	httpList := make([]*httpSeries, 0, len(r.http))
	for _, s := range r.http {
		httpList = append(httpList, s)
	}
	sort.Slice(httpList, func(i, j int) bool {
		if httpList[i].route != httpList[j].route {
			return httpList[i].route < httpList[j].route
		}
		if httpList[i].method != httpList[j].method {
			return httpList[i].method < httpList[j].method
		}
		return httpList[i].status < httpList[j].status
	})
	codes := make(map[string]int64, len(r.codes))
	for k, v := range r.codes {
		codes[k] = v
	}
	cron := make(map[string]int64, len(r.cron))
	for k, v := range r.cron {
		cron[k] = v
	}
	cronLast := make(map[string]int64, len(r.cronLast))
	for k, v := range r.cronLast {
		cronLast[k] = v
	}
	overflow := r.overflow
	r.mu.Unlock()

	// ---------- 构建信息 ----------
	fmt.Fprintf(w, "# HELP web_basic_build_info 构建信息（固定值 1，标签携带版本）\n")
	fmt.Fprintf(w, "# TYPE web_basic_build_info gauge\n")
	fmt.Fprintf(w, "web_basic_build_info{version=%q,commit=%q,build_time=%q} 1\n",
		ex.Build["version"], ex.Build["commit"], ex.Build["buildTime"])

	fmt.Fprintf(w, "# HELP web_basic_uptime_seconds 进程已运行秒数\n# TYPE web_basic_uptime_seconds gauge\n")
	fmt.Fprintf(w, "web_basic_uptime_seconds %d\n", int64(time.Since(r.start).Seconds()))

	// ---------- HTTP ----------
	fmt.Fprintf(w, "# HELP web_basic_http_requests_total HTTP 请求总数\n# TYPE web_basic_http_requests_total counter\n")
	for _, s := range httpList {
		fmt.Fprintf(w, "web_basic_http_requests_total{method=%q,route=%q,status=%q} %d\n", s.method, s.route, s.status, s.count)
	}
	if overflow > 0 {
		fmt.Fprintf(w, "web_basic_http_requests_total{method=\"-\",route=\"__other__\",status=\"-\"} %d\n", overflow)
	}

	fmt.Fprintf(w, "# HELP web_basic_http_request_duration_seconds HTTP 请求耗时\n# TYPE web_basic_http_request_duration_seconds histogram\n")
	for _, s := range httpList {
		for i, b := range latencyBuckets {
			fmt.Fprintf(w, "web_basic_http_request_duration_seconds_bucket{method=%q,route=%q,le=%q} %d\n",
				s.method, s.route, trimFloat(b), s.bucket[i])
		}
		fmt.Fprintf(w, "web_basic_http_request_duration_seconds_bucket{method=%q,route=%q,le=\"+Inf\"} %d\n", s.method, s.route, s.count)
		fmt.Fprintf(w, "web_basic_http_request_duration_seconds_sum{method=%q,route=%q} %s\n", s.method, s.route, trimFloat(s.sum))
		fmt.Fprintf(w, "web_basic_http_request_duration_seconds_count{method=%q,route=%q} %d\n", s.method, s.route, s.count)
	}

	// ---------- 业务码 ----------
	fmt.Fprintf(w, "# HELP web_basic_response_codes_total 接口响应码分布（0=成功）\n# TYPE web_basic_response_codes_total counter\n")
	codeKeys := make([]string, 0, len(codes))
	for k := range codes {
		codeKeys = append(codeKeys, k)
	}
	sort.Strings(codeKeys)
	for _, k := range codeKeys {
		fmt.Fprintf(w, "web_basic_response_codes_total{code=%q} %d\n", k, codes[k])
	}

	// ---------- 定时任务 ----------
	fmt.Fprintf(w, "# HELP web_basic_cron_runs_total 定时任务执行次数\n# TYPE web_basic_cron_runs_total counter\n")
	cronKeys := make([]string, 0, len(cron))
	for k := range cron {
		cronKeys = append(cronKeys, k)
	}
	sort.Strings(cronKeys)
	for _, k := range cronKeys {
		parts := strings.SplitN(k, "|", 2)
		fmt.Fprintf(w, "web_basic_cron_runs_total{handler=%q,result=%q} %d\n", parts[0], parts[1], cron[k])
	}
	fmt.Fprintf(w, "# HELP web_basic_cron_last_duration_ms 定时任务最近一次耗时（毫秒）\n# TYPE web_basic_cron_last_duration_ms gauge\n")
	lastKeys := make([]string, 0, len(cronLast))
	for k := range cronLast {
		lastKeys = append(lastKeys, k)
	}
	sort.Strings(lastKeys)
	for _, k := range lastKeys {
		fmt.Fprintf(w, "web_basic_cron_last_duration_ms{handler=%q} %d\n", k, cronLast[k])
	}

	// ---------- 依赖探针 ----------
	if ex.MySQL != nil {
		fmt.Fprintf(w, "# HELP web_basic_db_connections 数据库连接池状态\n# TYPE web_basic_db_connections gauge\n")
		for _, st := range []string{"open", "in_use", "idle"} {
			fmt.Fprintf(w, "web_basic_db_connections{state=%q} %s\n", st, trimFloat(ex.MySQL[st]))
		}
	}
	fmt.Fprintf(w, "# HELP web_basic_redis_up Redis 可用性（1/0）\n# TYPE web_basic_redis_up gauge\n")
	fmt.Fprintf(w, "web_basic_redis_up %s\n", trimFloat(ex.Redis))

	// ---------- 运行时 ----------
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "# HELP web_basic_goroutines 当前 goroutine 数\n# TYPE web_basic_goroutines gauge\n")
	fmt.Fprintf(w, "web_basic_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "# HELP web_basic_memory_alloc_bytes 已分配堆内存字节数\n# TYPE web_basic_memory_alloc_bytes gauge\n")
	fmt.Fprintf(w, "web_basic_memory_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(w, "# HELP web_basic_memory_sys_bytes 进程向系统申请的内存字节数\n# TYPE web_basic_memory_sys_bytes gauge\n")
	fmt.Fprintf(w, "web_basic_memory_sys_bytes %d\n", ms.Sys)
	fmt.Fprintf(w, "# HELP web_basic_gc_total GC 次数\n# TYPE web_basic_gc_total counter\n")
	fmt.Fprintf(w, "web_basic_gc_total %d\n", ms.NumGC)
}

// Reset 清空计数器（测试用）。
func (r *Registry) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.http = make(map[string]*httpSeries, 64)
	r.codes = make(map[string]int64, 16)
	r.cron = make(map[string]int64, 16)
	r.cronLast = make(map[string]int64, 16)
	r.overflow = 0
	r.mu.Unlock()
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// Package sysinfo 采集主机硬件与系统状态（CPU / 内存 / 磁盘 / 网络 / 温度 / 进程）。
//
// 统一用 gopsutil 跨平台实现：设备上的 linux/arm64 与开发机的 windows/amd64 都能跑，
// 缺失的指标（例如虚拟机没有温度传感器）返回 0 而不是报错，保证页面始终有数据可显示。
package sysinfo

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Snapshot 一次完整的主机快照。
type Snapshot struct {
	CollectedAt time.Time   `json:"collectedAt"`
	Host        HostInfo    `json:"host"`
	CPU         CPUInfo     `json:"cpu"`
	Memory      MemoryInfo  `json:"memory"`
	Disks       []DiskInfo  `json:"disks"`
	Net         NetInfo     `json:"net"`
	NetIfaces   []IfaceInfo `json:"netIfaces"`
	Load        LoadInfo    `json:"load"`
	Temperature []float64   `json:"temperature"`
}

// HostInfo 主机基础信息。
type HostInfo struct {
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	Platform      string `json:"platform"`
	PlatformVer   string `json:"platformVersion"`
	KernelArch    string `json:"kernelArch"`
	KernelVersion string `json:"kernelVersion"`
	Uptime        uint64 `json:"uptime"`
	BootTime      uint64 `json:"bootTime"`
	Virtualization string `json:"virtualization"`
	GoVersion     string `json:"goVersion"`
	NumCPU        int    `json:"numCpu"`
}

// CPUInfo CPU 使用情况。
type CPUInfo struct {
	Model       string    `json:"model"`
	Cores       int       `json:"cores"`
	Logical     int       `json:"logical"`
	UsedPercent float64   `json:"usedPercent"`
	PerCore     []float64 `json:"perCore"`
	Mhz         float64   `json:"mhz"`
}

// MemoryInfo 内存与交换分区。
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"usedPercent"`
	SwapTotal   uint64  `json:"swapTotal"`
	SwapUsed    uint64  `json:"swapUsed"`
}

// DiskInfo 磁盘分区使用情况。
type DiskInfo struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}

// NetInfo 网络累计流量与速率。
type NetInfo struct {
	BytesSent      uint64  `json:"bytesSent"`
	BytesRecv      uint64  `json:"bytesRecv"`
	PacketsSent    uint64  `json:"packetsSent"`
	PacketsRecv    uint64  `json:"packetsRecv"`
	SendRateBps    float64 `json:"sendRateBps"`
	RecvRateBps    float64 `json:"recvRateBps"`
	ErrIn          uint64  `json:"errIn"`
	ErrOut         uint64  `json:"errOut"`
	DropIn         uint64  `json:"dropIn"`
	DropOut        uint64  `json:"dropOut"`
}

// IfaceInfo 网卡状态。
type IfaceInfo struct {
	Name        string   `json:"name"`
	Mac         string   `json:"mac"`
	MTU         int      `json:"mtu"`
	Up          bool     `json:"up"`
	Loopback    bool     `json:"loopback"`
	Flags       []string `json:"flags"`
	Addrs       []string `json:"addrs"`
	BytesSent   uint64   `json:"bytesSent"`
	BytesRecv   uint64   `json:"bytesRecv"`
	SpeedMbps   int      `json:"speedMbps"`
}

// LoadInfo 平均负载与进程数。
type LoadInfo struct {
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
	ProcessCount int     `json:"processCount"`
}

// Collector 采集器：缓存的 CPU 采样与网卡速率上一次读数。
type Collector struct {
	lastNetAt   time.Time
	lastSent    uint64
	lastRecv    uint64
	cpuModel    string
	cpuCores    int
	cpuLogical  int
	cpuMhz      float64
}

// NewCollector 创建采集器。
func NewCollector() *Collector {
	c := &Collector{}
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		c.cpuModel = strings.TrimSpace(infos[0].ModelName)
		c.cpuMhz = infos[0].Mhz
	}
	if n, err := cpu.Counts(true); err == nil {
		c.cpuLogical = n
	}
	if n, err := cpu.Counts(false); err == nil {
		c.cpuCores = n
	}
	return c
}

// Collect 采集一次快照。ctx 用于限制耗时。
func (c *Collector) Collect(ctx context.Context) (*Snapshot, error) {
	now := time.Now()
	s := &Snapshot{CollectedAt: now}

	// 主机信息
	if hi, err := host.InfoWithContext(ctx); err == nil {
		s.Host = HostInfo{
			Hostname:       hi.Hostname,
			OS:             hi.OS,
			Platform:       hi.Platform,
			PlatformVer:    hi.PlatformVersion,
			KernelArch:     hi.KernelArch,
			KernelVersion:  hi.KernelVersion,
			Uptime:         hi.Uptime,
			BootTime:       hi.BootTime,
			Virtualization: hi.VirtualizationSystem,
			GoVersion:      runtime.Version(),
			NumCPU:         runtime.NumCPU(),
		}
	} else if name, herr := os.Hostname(); herr == nil {
		s.Host.Hostname = name
	}

	// CPU（短采样 200ms，避免接口变慢）
	perCore, _ := cpu.PercentWithContext(ctx, 200*time.Millisecond, true)
	total, _ := cpu.PercentWithContext(ctx, 0, false)
	s.CPU = CPUInfo{
		Model:   c.cpuModel,
		Cores:   c.cpuCores,
		Logical: c.cpuLogical,
		PerCore: perCore,
		Mhz:     c.cpuMhz,
	}
	if len(total) > 0 {
		s.CPU.UsedPercent = round2(total[0])
	}
	if s.CPU.Logical == 0 {
		s.CPU.Logical = runtime.NumCPU()
	}

	// 内存
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.Memory.Total = vm.Total
		s.Memory.Used = vm.Used
		s.Memory.Available = vm.Available
		s.Memory.UsedPercent = round2(vm.UsedPercent)
	}
	if sw, err := mem.SwapMemoryWithContext(ctx); err == nil {
		s.Memory.SwapTotal = sw.Total
		s.Memory.SwapUsed = sw.Used
	}

	// 磁盘（只取真实分区，跳过容器/tmpfs 噪音）
	if parts, err := disk.PartitionsWithContext(ctx, false); err == nil {
		for _, p := range parts {
			if skipFstype(p.Fstype) {
				continue
			}
			usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
			if err != nil || usage.Total == 0 {
				continue
			}
			s.Disks = append(s.Disks, DiskInfo{
				Device:      p.Device,
				Mountpoint:  p.Mountpoint,
				Fstype:      p.Fstype,
				Total:       usage.Total,
				Used:        usage.Used,
				Free:        usage.Free,
				UsedPercent: round2(usage.UsedPercent),
			})
		}
	}

	// 网络累计流量
	if counters, err := net.IOCountersWithContext(ctx, false); err == nil && len(counters) > 0 {
		io := counters[0]
		s.Net.BytesSent = io.BytesSent
		s.Net.BytesRecv = io.BytesRecv
		s.Net.PacketsSent = io.PacketsSent
		s.Net.PacketsRecv = io.PacketsRecv
		s.Net.ErrIn = io.Errin
		s.Net.ErrOut = io.Errout
		s.Net.DropIn = io.Dropin
		s.Net.DropOut = io.Dropout
		if !c.lastNetAt.IsZero() {
			elapsed := now.Sub(c.lastNetAt).Seconds()
			if elapsed > 0 {
				if io.BytesSent >= c.lastSent {
					s.Net.SendRateBps = round2(float64(io.BytesSent-c.lastSent) * 8 / elapsed)
				}
				if io.BytesRecv >= c.lastRecv {
					s.Net.RecvRateBps = round2(float64(io.BytesRecv-c.lastRecv) * 8 / elapsed)
				}
			}
		}
		c.lastNetAt, c.lastSent, c.lastRecv = now, io.BytesSent, io.BytesRecv
	}

	// 网卡列表
	s.NetIfaces = c.Interfaces(ctx)

	// 负载
	if avg, err := load.AvgWithContext(ctx); err == nil {
		s.Load.Load1 = round2(avg.Load1)
		s.Load.Load5 = round2(avg.Load5)
		s.Load.Load15 = round2(avg.Load15)
	}
	if pids, err := process.PidsWithContext(ctx); err == nil {
		s.Load.ProcessCount = len(pids)
	}

	// 温度（虚拟机/容器通常没有，返回空即可）
	if temps, err := sensors.TemperaturesWithContext(ctx); err == nil {
		for _, t := range temps {
			if t.Temperature > 0 {
				s.Temperature = append(s.Temperature, round2(t.Temperature))
			}
		}
	}

	return s, nil
}

// Interfaces 收集网卡详细信息。
func (c *Collector) Interfaces(ctx context.Context) []IfaceInfo {
	addrs := map[string][]string{}
	if list, err := net.InterfacesWithContext(ctx); err == nil {
		for _, a := range list {
			if len(a.Addrs) == 0 {
				continue
			}
			list := make([]string, 0, len(a.Addrs))
			for _, ad := range a.Addrs {
				list = append(list, ad.Addr)
			}
			addrs[a.Name] = list
		}
	}
	counters := map[string]net.IOCountersStat{}
	if list, err := net.IOCountersWithContext(ctx, true); err == nil {
		for _, io := range list {
			counters[io.Name] = io
		}
	}

	out := make([]IfaceInfo, 0, 8)
	list, err := net.InterfacesWithContext(ctx)
	if err != nil {
		return out
	}
	for _, it := range list {
		info := IfaceInfo{
			Name:     it.Name,
			Mac:      it.HardwareAddr,
			MTU:      it.MTU,
			Up:       hasFlag(it.Flags, "up"),
			Loopback: hasFlag(it.Flags, "loopback"),
			Flags:    it.Flags,
			Addrs:    addrs[it.Name],
		}
		if io, ok := counters[it.Name]; ok {
			info.BytesSent = io.BytesSent
			info.BytesRecv = io.BytesRecv
		}
		out = append(out, info)
	}
	return out
}

// Summary 轻量摘要，供告警判定与卡片展示（避免整表读磁盘）。
type Summary struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemPercent    float64 `json:"memPercent"`
	DiskPercent   float64 `json:"diskPercent"`
	NetInBps      float64 `json:"netInBps"`
	NetOutBps     float64 `json:"netOutBps"`
	Uptime        uint64  `json:"uptime"`
	Hostname      string  `json:"hostname"`
	Platform      string  `json:"platform"`
	PlatformVer   string  `json:"platformVersion"`
	KernelArch    string  `json:"kernelArch"`
	NumCPU        int     `json:"numCpu"`
	MemTotal      uint64  `json:"memTotal"`
	DiskTotal     uint64  `json:"diskTotal"`
}

// Summarize 从快照提取摘要（磁盘取使用率最高的分区）。
func Summarize(s *Snapshot) Summary {
	sum := Summary{
		CPUPercent:  s.CPU.UsedPercent,
		MemPercent:  s.Memory.UsedPercent,
		NetInBps:    s.Net.RecvRateBps,
		NetOutBps:   s.Net.SendRateBps,
		Uptime:      s.Host.Uptime,
		Hostname:    s.Host.Hostname,
		Platform:    s.Host.Platform,
		PlatformVer: s.Host.PlatformVer,
		KernelArch:  s.Host.KernelArch,
		NumCPU:      s.CPU.Logical,
		MemTotal:    s.Memory.Total,
	}
	for _, d := range s.Disks {
		sum.DiskTotal += d.Total
		if d.UsedPercent > sum.DiskPercent {
			sum.DiskPercent = d.UsedPercent
		}
	}
	return sum
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if strings.EqualFold(f, want) {
			return true
		}
	}
	return false
}

func skipFstype(fstype string) bool {
	switch strings.ToLower(fstype) {
	case "tmpfs", "devtmpfs", "devfs", "overlay", "squashfs", "ramfs", "proc", "sysfs", "cgroup", "cgroup2", "nsfs":
		return true
	}
	return false
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

// FormatBytes 人类可读的字节数（日志与告警文案用）。
func FormatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

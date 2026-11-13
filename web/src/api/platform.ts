/**
 * 设备平台接口（主机 / 网络 / 设备 / 安全 / 日志 / 插件）
 *
 * 约定与脚手架一致：全部直接返回后端领域结构，
 * 成功提示文案写在这里（页面不再重复弹窗）。
 *
 * @module api/platform
 */
import request from '@/utils/http'

/** 分页参数：current/size → page/pageSize */
export function toPage(params: Record<string, any> = {}) {
  const { current, size, ...rest } = params
  const query: Record<string, any> = { ...rest }
  if (current) query.page = current
  if (size) query.pageSize = size
  Object.keys(query).forEach((k) => {
    if (query[k] === undefined || query[k] === null || query[k] === '') delete query[k]
  })
  return query
}

/** 后端下发结果（逐项） */
export interface ApplyResult {
  item: string
  ok: boolean
  message: string
}

/** 策略类保存返回：记录 id + 冲突告警 */
export interface SaveWithWarnings {
  id: string
  warnings: string[]
}

/* ------------------------------- 主机 ------------------------------- */

export function fetchHostOverview() {
  return request.get<Record<string, any>>({ url: '/api/v1/host/overview' })
}

export function fetchHostSnapshot() {
  return request.get<Record<string, any>>({ url: '/api/v1/host/snapshot' })
}

export function fetchHostMetrics(minutes = 60) {
  return request.get<any[]>({ url: '/api/v1/host/metrics', params: { minutes } })
}

/* ------------------------------- 告警 ------------------------------- */

export function fetchAlertRules() {
  return request.get<any[]>({ url: '/api/v1/device/alert/rules' })
}

export function saveAlertRule(data: Record<string, any>) {
  if (data.id) {
    return request.put({ url: '/api/v1/device/alert/rules', params: data, successMessage: '保存成功', showSuccessMessage: true })
  }
  return request.post<{ id: string }>({
    url: '/api/v1/device/alert/rules',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteAlertRule(id: string) {
  return request.del({ url: `/api/v1/device/alert/rules/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

export function fetchAlertRecords(params: Record<string, any> = {}) {
  return request.get<WebBasic.Page<any>>({ url: '/api/v1/device/alert/records', params: toPage(params) })
}

export function ackAlertRecord(id: string) {
  return request.put({ url: `/api/v1/device/alert/records/${id}/ack`, successMessage: '已确认', showSuccessMessage: true })
}

/* ------------------------------- 日志 ------------------------------- */

export function fetchAccessLogs(params: Record<string, any> = {}) {
  return request.get<WebBasic.Page<any>>({ url: '/api/v1/logs/access', params: toPage(params) })
}

export function fetchTrafficLogs(params: Record<string, any> = {}) {
  return request.get<WebBasic.Page<any>>({ url: '/api/v1/logs/traffic', params: toPage(params) })
}

export function fetchSystemLog(lines = 500) {
  return request.get<Record<string, any>>({ url: '/api/v1/logs/system', params: { lines } })
}

/* ------------------------------- 设备 ------------------------------- */

export function fetchDeviceSettings() {
  return request.get<Record<string, any>>({ url: '/api/v1/device/settings' })
}

export function updateDeviceSettings(data: Record<string, any>) {
  return request.put<{ results: ApplyResult[] }>({
    url: '/api/v1/device/settings',
    params: data,
    successMessage: '设置已保存',
    showSuccessMessage: true
  })
}

export function fetchDeviceTime() {
  return request.get<Record<string, any>>({ url: '/api/v1/device/time' })
}

export function syncDeviceTime() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/device/time/sync',
    successMessage: '已触发校时',
    showSuccessMessage: true
  })
}

export function fetchPlatformAbout() {
  return request.get<Record<string, any>>({ url: '/api/v1/device/about' })
}

export function fetchToolMeta() {
  return request.get<Record<string, any>>({ url: '/api/v1/device/tools/meta' })
}

export function runTool(data: Record<string, any>) {
  return request.post<Record<string, any>>({ url: '/api/v1/device/tools/run', params: data })
}

/* ------------------------------- 网络 ------------------------------- */

export function fetchNetworkStatus() {
  return request.get<Record<string, any>>({ url: '/api/v1/network/status' })
}

export function fetchInterfaces() {
  return request.get<any[]>({ url: '/api/v1/network/interfaces' })
}

export function fetchStaticRoutes() {
  return request.get<any[]>({ url: '/api/v1/network/routes' })
}

export function saveStaticRoute(data: Record<string, any>) {
  if (data.id) {
    return request.put({ url: '/api/v1/network/routes', params: data, successMessage: '保存成功', showSuccessMessage: true })
  }
  return request.post<{ id: string }>({
    url: '/api/v1/network/routes',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteStaticRoute(id: string) {
  return request.del({ url: `/api/v1/network/routes/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

export function applyStaticRoutes() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/routes/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function fetchDNSConfig() {
  return request.get<Record<string, any>>({ url: '/api/v1/network/dns' })
}

export function updateDNSConfig(servers: string[]) {
  return request.put<{ results: ApplyResult[] }>({
    url: '/api/v1/network/dns',
    params: { servers },
    successMessage: 'DNS 已保存',
    showSuccessMessage: true
  })
}

/* ------------------------------- 安全策略 ------------------------------- */

export function fetchFirewallStatus() {
  return request.get<Record<string, any>>({ url: '/api/v1/security/firewall' })
}

export function installFirewalld() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/security/firewall/install',
    successMessage: '安装指令已执行',
    showSuccessMessage: true
  })
}

export function fetchPolicies() {
  return request.get<any[]>({ url: '/api/v1/security/policies' })
}

export function savePolicy(data: Record<string, any>) {
  if (data.id) {
    return request.put({ url: '/api/v1/security/policies', params: data, successMessage: '保存成功', showSuccessMessage: true })
  }
  return request.post<SaveWithWarnings>({
    url: '/api/v1/security/policies',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deletePolicy(id: string) {
  return request.del({ url: `/api/v1/security/policies/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

export function applyPolicies() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/security/policies/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function fetchNATRules() {
  return request.get<any[]>({ url: '/api/v1/security/nat' })
}

export function saveNATRule(data: Record<string, any>) {
  if (data.id) {
    return request.put({ url: '/api/v1/security/nat', params: data, successMessage: '保存成功', showSuccessMessage: true })
  }
  return request.post<SaveWithWarnings>({
    url: '/api/v1/security/nat',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteNATRule(id: string) {
  return request.del({ url: `/api/v1/security/nat/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

export function applyNATRules() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/security/nat/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function fetchQosRules() {
  return request.get<any[]>({ url: '/api/v1/security/qos' })
}

export function saveQosRule(data: Record<string, any>) {
  if (data.id) {
    return request.put({ url: '/api/v1/security/qos', params: data, successMessage: '保存成功', showSuccessMessage: true })
  }
  return request.post<SaveWithWarnings>({
    url: '/api/v1/security/qos',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteQosRule(id: string) {
  return request.del({ url: `/api/v1/security/qos/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

export function applyQosRules() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/security/qos/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

/* ------------------------------- 插件市场 ------------------------------- */

export function fetchPlugins() {
  return request.get<any[]>({ url: '/api/v1/plugins' })
}

export function fetchPlugin(key: string) {
  return request.get<any>({ url: `/api/v1/plugins/${key}` })
}

export function fetchPluginMeta() {
  return request.get<Record<string, any>>({ url: '/api/v1/plugins/meta' })
}

export function fetchBundledPlugins() {
  return request.get<any[]>({ url: '/api/v1/plugins/bundled' })
}

export function installBundledPlugin(key: string) {
  return request.post({
    url: `/api/v1/plugins/bundled/${key}/install`,
    successMessage: '安装成功',
    showSuccessMessage: true
  })
}

export function uploadPlugin(file: File) {
  const form = new FormData()
  form.append('file', file)
  return request.post({
    url: '/api/v1/plugins/install',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    successMessage: '安装成功',
    showSuccessMessage: true
  })
}

export function pluginAction(key: string, action: string, value?: boolean) {
  const labels: Record<string, string> = {
    start: '插件已启动',
    stop: '插件已停止',
    restart: '插件已重启',
    enable: '插件已启用',
    disable: '插件已禁用',
    autostart: value ? '已开启自启' : '已关闭自启'
  }
  return request.post({
    url: `/api/v1/plugins/${key}/${action}`,
    params: value === undefined ? {} : { value: String(value) },
    successMessage: labels[action] || '操作成功',
    showSuccessMessage: true
  })
}

export function uninstallPlugin(key: string, purge = false) {
  return request.del({
    url: `/api/v1/plugins/${key}`,
    params: { purge: String(purge) },
    successMessage: '已卸载',
    showSuccessMessage: true
  })
}

export function updatePluginConfig(key: string, config: Record<string, any>, restart = false) {
  return request.put({
    url: `/api/v1/plugins/${key}/config`,
    params: { config, restart },
    successMessage: restart ? '配置已保存并重启' : '配置已保存',
    showSuccessMessage: true
  })
}

export function fetchPluginLogs(key: string, lines = 300) {
  return request.get<{ text: string }>({ url: `/api/v1/plugins/${key}/logs`, params: { lines } })
}

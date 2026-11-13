/**
 * 运维能力接口（网卡配置 / 策略路由 / DHCP / DNS 服务 / 动态路由 / SNMP / 插件源与在线市场）
 *
 * @module api/ops
 */
import request from '@/utils/http'

/** 下发结果（逐项） */
export interface ApplyResult {
  item: string
  ok: boolean
  message: string
}

/** 保存策略类接口的统一返回：记录 id + 冲突告警 */
export interface SaveWithWarnings {
  id: string
  warnings: string[]
}

/* ------------------------------- 网卡配置 / 网桥 / 聚合 ------------------------------- */

export function fetchIfaceConfigs() {
  return request.get<any[]>({ url: '/api/v1/network/iface-configs' })
}

export function fetchIfaceMeta() {
  return request.get<Record<string, any>>({ url: '/api/v1/network/iface-configs/meta' })
}

export function fetchNetplanPreview() {
  return request.get<{ preview: string }>({ url: '/api/v1/network/iface-configs/preview' })
}

export function saveIfaceConfig(data: Record<string, any>) {
  if (data.id) {
    return request.put<{ id: string }>({
      url: '/api/v1/network/iface-configs',
      params: data,
      successMessage: '保存成功',
      showSuccessMessage: true
    })
  }
  return request.post<{ id: string }>({
    url: '/api/v1/network/iface-configs',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteIfaceConfig(id: string) {
  return request.del({
    url: `/api/v1/network/iface-configs/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function applyIfaceConfigs() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/iface-configs/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

/* ------------------------------- 策略路由 ------------------------------- */

export function fetchPolicyRoutes() {
  return request.get<{ list: any[]; kernelRules: string[] }>({ url: '/api/v1/network/policy-routes' })
}

export function savePolicyRoute(data: Record<string, any>) {
  if (data.id) {
    return request.put<{ id: string }>({
      url: '/api/v1/network/policy-routes',
      params: data,
      successMessage: '保存成功',
      showSuccessMessage: true
    })
  }
  return request.post<{ id: string }>({
    url: '/api/v1/network/policy-routes',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deletePolicyRoute(id: string) {
  return request.del({
    url: `/api/v1/network/policy-routes/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function applyPolicyRoutes() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/policy-routes/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

/* ------------------------------- DHCP / DNS 服务 ------------------------------- */

export function fetchDhcpConfig() {
  return request.get<{ config: any; status: any }>({ url: '/api/v1/network/dhcp' })
}

export function saveDhcpConfig(data: Record<string, any>) {
  return request.put<any>({
    url: '/api/v1/network/dhcp',
    params: data,
    successMessage: '配置已保存',
    showSuccessMessage: true
  })
}

export function applyDhcpConfig() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/dhcp/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function fetchDnsServerConfig() {
  return request.get<{ config: any; status: any }>({ url: '/api/v1/network/dns-server' })
}

export function saveDnsServerConfig(data: Record<string, any>) {
  return request.put<any>({
    url: '/api/v1/network/dns-server',
    params: data,
    successMessage: '配置已保存',
    showSuccessMessage: true
  })
}

export function applyDnsServerConfig() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/dns-server/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function installDnsmasq(which: 'dhcp' | 'dns') {
  return request.post<{ results: ApplyResult[] }>({
    url: which === 'dhcp' ? '/api/v1/network/dhcp/install' : '/api/v1/network/dns-server/install',
    successMessage: '安装指令已执行',
    showSuccessMessage: true
  })
}

export function fetchDnsmasqStatus() {
  return request.get<Record<string, any>>({ url: '/api/v1/network/dnsmasq/status' })
}

/* ------------------------------- 动态路由 BGP / OSPF ------------------------------- */

export function fetchRoutingConfigs() {
  return request.get<{ list: any[]; status: any }>({ url: '/api/v1/network/routing' })
}

export function saveRoutingConfig(data: Record<string, any>) {
  return request.put<{ id: string }>({
    url: '/api/v1/network/routing',
    params: data,
    successMessage: '配置已保存',
    showSuccessMessage: true
  })
}

export function applyRoutingConfigs() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/routing/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function installFRR() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/network/routing/install',
    successMessage: '安装指令已执行',
    showSuccessMessage: true
  })
}

export function fetchRoutingStatus() {
  return request.get<Record<string, any>>({ url: '/api/v1/network/routing/status' })
}

/* ------------------------------- SNMP ------------------------------- */

export function fetchSnmpConfig() {
  return request.get<{ config: any; status: any }>({ url: '/api/v1/device/snmp' })
}

export function saveSnmpConfig(data: Record<string, any>) {
  return request.put<any>({
    url: '/api/v1/device/snmp',
    params: data,
    successMessage: '配置已保存',
    showSuccessMessage: true
  })
}

export function applySnmpConfig() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/device/snmp/apply',
    successMessage: '下发完成',
    showSuccessMessage: true
  })
}

export function installSNMP() {
  return request.post<{ results: ApplyResult[] }>({
    url: '/api/v1/device/snmp/install',
    successMessage: '安装指令已执行',
    showSuccessMessage: true
  })
}

/* ------------------------------- 插件源与在线市场 ------------------------------- */

export function fetchMarketSources() {
  return request.get<any[]>({ url: '/api/v1/plugins/sources' })
}

export function saveMarketSource(data: Record<string, any>) {
  if (data.id) {
    return request.put<{ id: string }>({
      url: '/api/v1/plugins/sources',
      params: data,
      successMessage: '保存成功',
      showSuccessMessage: true
    })
  }
  return request.post<{ id: string }>({
    url: '/api/v1/plugins/sources',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function deleteMarketSource(id: string) {
  return request.del({
    url: `/api/v1/plugins/sources/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchMarketPlugins(refresh = false) {
  return request.get<any[]>({ url: '/api/v1/plugins/market', params: { refresh: String(refresh) } })
}

export function installMarketPlugin(sourceId: string, pluginId: string) {
  return request.post({
    url: '/api/v1/plugins/market/install',
    params: { sourceId, pluginId },
    successMessage: '安装成功',
    showSuccessMessage: true
  })
}

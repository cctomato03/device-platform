/**
 * 业务模块接口（部门 / 岗位 / 字典 / 参数 / 租户 / 文件 / 监控）
 *
 * 全部对接 Go 后端 /api/v1/**，直接返回后端领域结构（WebBasic.*），
 * 供 web-basic 自建页面使用；模板自带演示页不使用本文件。
 *
 * @module api/business
 */
import request from '@/utils/http'

/** 分页参数转换：current/size → page/pageSize */
export function toPageQuery(params: Record<string, any> = {}) {
  const { current, size, ...rest } = params
  const query: Record<string, any> = { ...rest }
  if (current) query.page = current
  if (size) query.pageSize = size
  Object.keys(query).forEach((k) => {
    if (query[k] === undefined || query[k] === null || query[k] === '') delete query[k]
  })
  return query
}

/** 分页结果包装为模板 useTable 可识别的结构 */
export function toTablePage<T>(res: WebBasic.Page<T>) {
  return {
    records: res.list || [],
    current: res.page || 1,
    size: res.pageSize || 10,
    total: res.total || 0
  }
}

/* ------------------------------- 部门 ------------------------------- */

export function fetchGetDeptTree() {
  return request.get<WebBasic.Dept[]>({ url: '/api/v1/system/depts/tree' })
}

export function fetchCreateDept(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/depts',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateDept(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/depts/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteDept(id: string | number) {
  return request.del<null>({ url: `/api/v1/system/depts/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

/* ------------------------------- 岗位 ------------------------------- */

export function fetchGetPostList() {
  return request.get<WebBasic.Post[]>({ url: '/api/v1/system/posts' })
}

export function fetchCreatePost(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/posts',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdatePost(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/posts/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeletePost(id: string | number) {
  return request.del<null>({ url: `/api/v1/system/posts/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

/* ------------------------------- 字典 ------------------------------- */

export function fetchGetDictTypes() {
  return request.get<WebBasic.DictType[]>({ url: '/api/v1/system/dict/types' })
}

export function fetchCreateDictType(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/dict/types',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateDictType(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/dict/types/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteDictType(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/dict/types/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/** 字典项（按字典编码查询，登录即可读） */
export function fetchGetDictData(code: string) {
  return request.get<WebBasic.DictData[]>({ url: `/api/v1/dicts/${code}` })
}

export function fetchCreateDictData(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/dict/data',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateDictData(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/dict/data/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteDictData(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/dict/data/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------- 参数 ------------------------------- */

export function fetchGetConfigList() {
  return request.get<WebBasic.Config[]>({ url: '/api/v1/system/configs' })
}

export function fetchCreateConfig(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/configs',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

/**
 * 修改参数配置
 * @param options.silent 静默模式：不弹成功提示（一次改多个参数时由调用方统一提示）
 */
export function fetchUpdateConfig(
  id: string | number,
  data: Record<string, any>,
  options: { silent?: boolean } = {}
) {
  return request.put<null>({
    url: `/api/v1/system/configs/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: !options.silent
  })
}

export function fetchDeleteConfig(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/configs/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/** 公开参数（免登录） */
export function fetchGetPublicConfig() {
  return request.get<Record<string, string>>({
    url: '/api/v1/system/configs/public',
    showErrorMessage: false
  })
}

/* ------------------------------- 租户 ------------------------------- */

export function fetchGetTenantList() {
  return request.get<WebBasic.Tenant[]>({ url: '/api/v1/system/tenants' })
}

export function fetchCreateTenant(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/tenants',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateTenant(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/tenants/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteTenant(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/tenants/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------- 文件 ------------------------------- */

/** 文件实体 → 页面展示结构（后端字段：storageType / objectKey / mime） */
export function mapFile(f: any): WebBasic.FileItem {
  const key: string = f.objectKey || ''
  const segs = key.split('/').filter(Boolean)
  return {
    id: String(f.id ?? ''),
    originName: f.originName || '',
    fileName: segs.length ? segs[segs.length - 1] : '',
    path: key,
    dir: segs.slice(0, -1).join('/'),
    url: f.url || '',
    size: Number(f.size || 0),
    ext: f.ext || '',
    mimeType: f.mime || '',
    storage: f.storageType || 'local',
    bizType: f.bizType || '',
    createTime: f.createTime || ''
  }
}

export async function fetchGetFileList(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.FileItem>>({
    url: '/api/v1/files',
    params: toPageQuery(params)
  })
  if (res?.list) res.list = res.list.map(mapFile)
  return toTablePage(res)
}

/** 上传文件（FormData） */
export function fetchUploadFile(file: File, bizType = 'default') {
  const form = new FormData()
  form.append('file', file)
  form.append('bizType', bizType)
  return request.post<WebBasic.FileItem>({
    url: '/api/v1/files/upload',
    data: form,
    successMessage: '上传成功',
    showSuccessMessage: true
  })
}

/**
 * 申请 OSS 直传签名（后端只签名，文件由浏览器直接 PUT 到 OSS）。
 * 存储为本地磁盘时后端返回「不支持」，调用方应回退到 fetchUploadFile。
 */
export function fetchGetOssTicket(data: { originName: string; size: number; bizType?: string }) {
  return request.post<WebBasic.OssTicket>({
    url: '/api/v1/files/oss-ticket',
    params: data,
    showErrorMessage: false
  })
}

/** 直传完成回执：把对象键登记进文件表 */
export async function fetchRecordOssFile(data: {
  objectKey: string
  originName: string
  size: number
  bizType?: string
  mime?: string
}) {
  const res = await request.post<any>({
    url: '/api/v1/files/oss-record',
    params: data,
    successMessage: '上传成功',
    showSuccessMessage: true
  })
  return mapFile(res)
}

export function fetchDeleteFile(id: string | number) {
  return request.del<null>({ url: `/api/v1/files/${id}`, successMessage: '删除成功', showSuccessMessage: true })
}

/* ------------------------------- 监控 ------------------------------- */

export async function fetchGetLoginLogs(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.LoginLog>>({
    url: '/api/v1/monitor/login-logs',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export async function fetchGetOperLogs(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.OperLog>>({
    url: '/api/v1/monitor/oper-logs',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetOnlineUsers() {
  return request.get<WebBasic.OnlineUser[]>({ url: '/api/v1/monitor/online' })
}

export function fetchKickUser(userId: string | number, jti: string) {
  return request.del<null>({
    url: '/api/v1/monitor/online',
    params: { userId, jti },
    successMessage: '已强制下线',
    showSuccessMessage: true
  })
}

export function fetchGetServerInfo() {
  return request.get<WebBasic.ServerInfo>({ url: '/api/v1/monitor/server' })
}

/* ------------------------------- 首页概览 ------------------------------- */

export function fetchGetDashboardSummary() {
  return request.get<WebBasic.DashboardSummary>({ url: '/api/v1/dashboard/summary' })
}

/* ------------------------------- 定时任务 ------------------------------- */

export function fetchGetJobList() {
  return request.get<WebBasic.Job[]>({ url: '/api/v1/system/jobs' })
}

/** 任务元信息：类型、内置处理器、是否允许 Shell 脚本 */
export function fetchGetJobMeta() {
  return request.get<WebBasic.JobMeta>({ url: '/api/v1/system/jobs/meta' })
}

export function fetchCreateJob(data: Record<string, any>) {
  return request.post<WebBasic.Job>({
    url: '/api/v1/system/jobs',
    data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateJob(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/jobs/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchSetJobStatus(id: string | number, status: number) {
  return request.put<null>({
    url: `/api/v1/system/jobs/${id}/status`,
    data: { status },
    successMessage: '状态已更新',
    showSuccessMessage: true
  })
}

export function fetchRunJob(id: string | number) {
  return request.post<null>({
    url: `/api/v1/system/jobs/${id}/run`,
    successMessage: '已提交执行，可在执行日志中查看',
    showSuccessMessage: true
  })
}

export function fetchDeleteJob(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/jobs/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchGetJobLogs(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.JobLog>>({
    url: '/api/v1/system/jobs/logs',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

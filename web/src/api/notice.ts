import request from '@/utils/http'

/** 分页参数转换：前端 current/size → 后端 page/pageSize */
function toPage(params: Record<string, any>) {
  const { current, size, ...rest } = params
  return { page: current, pageSize: size, ...rest }
}

/** 通知公告列表 */
export async function fetchGetNoticeList(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.Notice>>({
    url: '/api/v1/system/notices',
    params: toPage(params)
  })
  return {
    records: res.list || [],
    current: res.page || 1,
    size: res.pageSize || 10,
    total: res.total || 0
  }
}

/** 公告详情 */
export function fetchGetNotice(id: string) {
  return request.get<WebBasic.Notice>({ url: `/api/v1/system/notices/${id}` })
}

/** 新增公告 */
export function fetchCreateNotice(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/notices',
    data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

/** 修改公告 */
export function fetchUpdateNotice(id: string, data: Record<string, any>) {
  return request.put({
    url: `/api/v1/system/notices/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

/** 删除公告 */
export function fetchDeleteNotice(id: string) {
  return request.del({
    url: `/api/v1/system/notices/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/** 发布公告（扇出站内信） */
export function fetchPublishNotice(id: string) {
  // 发布结果需要带上「推送了多少条站内信」，由调用方展示，接口层保持静默
  return request.post<{ pushed: number }>({ url: `/api/v1/system/notices/${id}/publish` })
}

/** 我的站内信 */
export async function fetchGetMyMessages(params: Record<string, any>) {
  const res = await request.get<WebBasic.Page<WebBasic.Message>>({
    url: '/api/v1/messages',
    params: toPage(params)
  })
  return {
    records: res.list || [],
    current: res.page || 1,
    size: res.pageSize || 10,
    total: res.total || 0
  }
}


/** 标记单条已读 */
export function fetchMarkMessageRead(id: string) {
  return request.put({ url: `/api/v1/messages/${id}/read` })
}

/** 全部标记已读 */
export function fetchMarkAllMessagesRead() {
  return request.put<{ updated: number }>({ url: '/api/v1/messages/read-all' })
}

/** 删除我的站内信 */
export function fetchDeleteMyMessage(id: string) {
  return request.del({
    url: `/api/v1/messages/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

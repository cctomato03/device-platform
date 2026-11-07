/**
 * 工作中心接口：待办 / 通知 / 消息 / 汇总 / 推送
 *
 * 这些能力面向「每个用户」，与系统设置无关：
 *  - /my/*      当前登录用户自己的数据（后端强制 tenant + user 过滤）
 *  - /system/push/* 后台手工发起（需要 system:push:create）
 *  - /open/*    业务系统对接（X-API-KEY，见开发手册「对接契约」）
 *
 * @module api/workbench
 */
import request from '@/utils/http'

/* ------------------------------- 汇总 ------------------------------- */

/** 工作中心汇总（顶栏红点 / 工作台卡片） */
export function fetchMySummary() {
  return request.get<WebBasic.WorkbenchSummary>({
    url: '/api/v1/my/summary',
    showErrorMessage: false
  })
}

/** 待办统计 */
export function fetchMyTodoSummary() {
  return request.get<WebBasic.TodoSummary>({
    url: '/api/v1/my/todos/summary',
    showErrorMessage: false
  })
}

/* ------------------------------- 待办 ------------------------------- */

export function fetchGetMyTodos(params: Record<string, any>) {
  return request.get<WebBasic.Page<WebBasic.Todo>>({
    url: '/api/v1/my/todos',
    params
  })
}

/** 新建自己的待办 */
export function fetchCreateMyTodo(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/my/todos',
    params: data,
    successMessage: '待办已创建',
    showSuccessMessage: true
  })
}

/** 更新待办状态：1 待处理 2 已完成 3 已取消 */
export function fetchUpdateMyTodoStatus(id: string | number, status: number) {
  return request.put<null>({
    url: `/api/v1/my/todos/${id}/status`,
    params: { status },
    successMessage: status === 2 ? '待办已完成' : '待办状态已更新',
    showSuccessMessage: true
  })
}

export function fetchDeleteMyTodo(id: string | number) {
  return request.del<null>({
    url: `/api/v1/my/todos/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------- 通知 ------------------------------- */

export function fetchGetMyNotices(params: Record<string, any>) {
  return request.get<WebBasic.Page<WebBasic.MyNotice>>({
    url: '/api/v1/my/notices',
    params
  })
}

export function fetchMarkNoticeRead(id: string | number) {
  return request.post<null>({
    url: `/api/v1/my/notices/${id}/read`,
    showSuccessMessage: false
  })
}

/* ------------------------------- 消息 ------------------------------- */

export function fetchGetMyMessages(params: Record<string, any>) {
  return request.get<WebBasic.Page<WebBasic.Message>>({
    url: '/api/v1/messages',
    params
  })
}

export function fetchMarkMessageRead(id: string | number) {
  return request.put<null>({ url: `/api/v1/messages/${id}/read`, showSuccessMessage: false })
}

export function fetchMarkAllMessagesRead() {
  return request.put<{ updated: number }>({
    url: '/api/v1/messages/read-all',
    successMessage: '已全部标记为已读',
    showSuccessMessage: true
  })
}

export function fetchDeleteMyMessage(id: string | number) {
  return request.del<null>({
    url: `/api/v1/messages/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------- 推送（后台手工发起） ------------------------------- */

/** 推送站内信（不传 userIds 表示全员） */
export function fetchPushMessages(data: Record<string, any>) {
  return request.post<{ created: number }>({
    url: '/api/v1/system/push/messages',
    params: data,
    showSuccessMessage: false
  })
}

/** 派发待办给指定用户 */
export function fetchPushTodo(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/push/todos',
    params: data,
    showSuccessMessage: false
  })
}

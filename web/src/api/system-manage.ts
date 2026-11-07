/**
 * 系统管理接口（web-basic 适配层）
 *
 * 对接 Go 后端 /api/v1/system/**，并把出参映射为 Web Admin Pro 模板页面
 * 需要的展示结构（`Api.SystemManage.*`），页面代码保持与上游一致。
 *
 * 约定：
 * - 请求分页参数：模板用 `current / size`，后端用 `page / pageSize`，在此统一转换
 * - 响应分页结构：直接返回后端 `{ list, total, page, pageSize }`，
 *   模板 utils/table/tableConfig.ts 已内置 `list / total / page / pageSize` 识别
 *
 * @module api/system-manage
 */
import request from '@/utils/http'
import type { AppRouteRecord } from '@/types/router'

/** 分页入参转换 */
function toPage(params: { current?: number; size?: number } & Record<string, any> = {}) {
  const { current, size, ...rest } = params
  const query: Record<string, any> = { ...rest }
  if (current) query.page = current
  if (size) query.pageSize = size
  Object.keys(query).forEach((k) => {
    if (query[k] === undefined || query[k] === null || query[k] === '') delete query[k]
  })
  return query
}

/** 性别码 ↔ 展示值 */
const SEX_TEXT: Record<number, string> = { 0: '未知', 1: '男', 2: '女' }

/** 用户实体 → 模板列表项 */
function mapUser(u: WebBasic.User): Api.SystemManage.UserListItem {
  return {
    id: u.id,
    avatar: u.avatar || '',
    status: String(u.status),
    userName: u.username,
    userGender: SEX_TEXT[u.sex ?? 0] || '未知',
    nickName: u.nickname,
    userPhone: u.mobile,
    userEmail: u.email,
    userRoles: u.roleNames?.length ? u.roleNames : (u.roleIds ?? []),
    createBy: '',
    createTime: u.createTime || '',
    updateBy: '',
    updateTime: u.updateTime || '',
    deptId: u.deptId,
    deptName: u.deptName,
    sex: u.sex ?? 0,
    remark: u.remark,
    roleIds: u.roleIds ?? [],
    postIds: u.postIds ?? []
  }
}

/** 角色实体 → 模板列表项 */
function mapRole(r: WebBasic.Role): Api.SystemManage.RoleListItem {
  return {
    roleId: r.id,
    roleName: r.name,
    roleCode: r.code,
    description: r.remark,
    enabled: r.status === 1,
    createTime: r.createTime || '',
    dataScope: r.dataScope,
    sort: r.sort
  }
}

/* ------------------------------------------------------------------ */
/* 用户                                                                */
/* ------------------------------------------------------------------ */

/** 用户分页列表 */
export async function fetchGetUserList(params: Api.SystemManage.UserSearchParams) {
  const res = await request.get<WebBasic.Page<WebBasic.User>>({
    url: '/api/v1/system/users',
    params: toPage({
      current: params.current,
      size: params.size,
      username: params.userName,
      mobile: params.userPhone,
      email: params.userEmail,
      status: params.status
    })
  })
  return {
    records: (res.list || []).map(mapUser),
    current: res.page || 1,
    size: res.pageSize || 10,
    total: res.total || 0
  } as Api.Common.PaginatedResponse<Api.SystemManage.UserListItem>
}

/** 用户详情 */
export async function fetchGetUser(id: string | number) {
  const res = await request.get<WebBasic.User>({ url: `/api/v1/system/users/${id}` })
  return mapUser(res)
}

/** 新增用户 */
export function fetchCreateUser(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/users',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

/** 修改用户 */
export function fetchUpdateUser(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/users/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

/** 删除用户（支持批量） */
export function fetchDeleteUser(ids: Array<string | number>) {
  return request.request<null>({
    url: '/api/v1/system/users',
    method: 'DELETE',
    data: { ids },
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/** 修改用户状态 */
export function fetchChangeUserStatus(id: string | number, status: number) {
  return request.put<null>({
    url: `/api/v1/system/users/${id}/status`,
    params: { status },
    successMessage: '状态已更新',
    showSuccessMessage: true
  })
}

/** 重置用户密码（后端返回本次生成的新密码明文，仅此一次） */
export function fetchResetUserPassword(id: string | number, password?: string) {
  return request.put<{ password: string }>({
    url: `/api/v1/system/users/${id}/reset-password`,
    params: { password },
    successMessage: '密码已重置',
    showSuccessMessage: true
  })
}


/* ------------------------------------------------------------------ */
/* 角色                                                                */
/* ------------------------------------------------------------------ */

/** 角色分页列表 */
export async function fetchGetRoleList(params: Api.SystemManage.RoleSearchParams) {
  const res = await request.get<WebBasic.Page<WebBasic.Role>>({
    url: '/api/v1/system/roles',
    params: toPage({
      current: params.current,
      size: params.size,
      name: params.roleName || params.roleCode,
      // enabled 兼容 1/0 与 true/false 两种取值：1/true=启用，0/false=禁用
      status:
        params.enabled === undefined || params.enabled === null ? undefined : params.enabled ? 1 : 0
    })
  })
  return {
    records: (res.list || []).map(mapRole),
    current: res.page || 1,
    size: res.pageSize || 10,
    total: res.total || 0
  } as Api.Common.PaginatedResponse<Api.SystemManage.RoleListItem>
}

/** 全部角色（下拉选择用） */
export async function fetchGetAllRoles() {
  const list = await request.get<WebBasic.Role[]>({ url: '/api/v1/system/roles/all' })
  return (list || []).map(mapRole)
}

/** 新增角色 */
export function fetchCreateRole(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/roles',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

/** 修改角色 */
export function fetchUpdateRole(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/roles/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

/** 删除角色 */
export function fetchDeleteRole(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/roles/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/** 角色已授权的菜单 ID 列表 */
export async function fetchGetRoleGranted(roleId: string | number) {
  const res = await request.get<{ menuIds?: string[] } | string[]>({
    url: `/api/v1/system/roles/${roleId}/granted`
  })
  if (Array.isArray(res)) return res
  return res?.menuIds ?? []
}

/** 角色授权（菜单/按钮） */
export function fetchGrantRole(
  roleId: string | number,
  data: { menuIds: Array<string | number>; deptIds?: Array<string | number> }
) {
  return request.put<null>({
    url: `/api/v1/system/roles/${roleId}/grant`,
    params: data,
    successMessage: '权限已保存',
    showSuccessMessage: true
  })
}

/* ------------------------------------------------------------------ */
/* 菜单                                                                */
/* ------------------------------------------------------------------ */

/**
 * 当前用户的前端动态路由表（后端权限模式）
 * 返回结构已对齐 AppRouteRecord，可直接交给路由注册器与菜单渲染。
 */
export function fetchGetMenuList() {
  return request.get<AppRouteRecord[]>({ url: '/api/v1/auth/routes' })
}

/** 菜单实体树（菜单管理页 CRUD 用，含按钮/接口节点） */
export function fetchGetMenuTree() {
  return request.get<WebBasic.Menu[]>({ url: '/api/v1/system/menus/tree' })
}

/** 新增菜单 */
export function fetchCreateMenu(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/system/menus',
    params: data,
    successMessage: '新增成功',
    showSuccessMessage: true
  })
}

/** 修改菜单 */
export function fetchUpdateMenu(id: string | number, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/system/menus/${id}`,
    params: data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

/** 删除菜单 */
export function fetchDeleteMenu(id: string | number) {
  return request.del<null>({
    url: `/api/v1/system/menus/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

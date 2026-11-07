/**
 * 认证相关接口（web-basic 适配层）
 *
 * 对接 Go 后端 /api/v1/auth/*，并把出参映射为 Web Admin Pro 约定结构，
 * 使模板其余代码（登录页、路由守卫、用户信息面板）无需改动。
 *
 * @module api/auth
 */
import request from '@/utils/http'

/** 图形验证码出参 */
export interface CaptchaResult {
  /** 是否开启验证码（后端参数 sys.login.captcha 控制） */
  enabled: boolean
  /** 验证码 ID */
  captchaId: string
  /** 验证码图片（base64） */
  image: string
}

/** 后端登录出参 */
interface TokenPayload {
  accessToken: string
  refreshToken: string
  expiresIn: number
}

/** 后端用户信息出参 */
export interface UserInfoPayload {
  user: {
    userId: string
    tenantId: string
    username: string
    nickname: string
    deptId: string
    deptName: string
    avatar?: string
    email?: string
    mobile?: string
    sex?: number
    isSuper: boolean
    dataScope: number
    roles: string[]
  }
  permissions: string[]
  menus: unknown[]
}

/** 密码强度规则（免登录） */
export interface PasswordPolicy {
  minLength: number
  requireUpper: boolean
  requireLower: boolean
  requireDigit: boolean
  description: string
}

/** 获取密码规则（注册/改密页展示，避免用户盲试） */
export function fetchPasswordPolicy() {
  return request.get<PasswordPolicy>({
    url: '/api/v1/public/password-policy',
    showErrorMessage: false
  })
}

/** 获取图形验证码（免登录） */
export function fetchGetCaptcha() {
  return request.get<CaptchaResult>({
    url: '/api/v1/auth/captcha',
    showErrorMessage: false
  })
}

/**
 * 登录
 * @param params 登录参数（userName / password / 验证码）
 */
export async function fetchLogin(
  params: Api.Auth.LoginParams & { captchaId?: string; captchaCode?: string }
): Promise<Api.Auth.LoginResponse> {
  const res = await request.post<TokenPayload>({
    url: '/api/v1/auth/login',
    params: {
      username: params.userName,
      password: params.password,
      captchaId: params.captchaId,
      captchaCode: params.captchaCode
    }
  })
  return { token: res.accessToken, refreshToken: res.refreshToken }
}

/**
 * 获取用户信息（权限码 + 角色 + 基础资料）
 */
export async function fetchGetUserInfo(): Promise<Api.Auth.UserInfo> {
  const data = await request.get<UserInfoPayload>({ url: '/api/v1/auth/userinfo' })
  const user = data.user || ({} as UserInfoPayload['user'])
  return {
    userId: user.userId,
    userName: user.username,
    nickName: user.nickname,
    email: user.email || '',
    mobile: user.mobile || '',
    avatar: user.avatar || '',
    roles: user.roles || [],
    deptName: user.deptName || '',
    isSuper: !!user.isSuper,
    dataScope: user.dataScope ?? 0,
    buttons: data.permissions?.length ? data.permissions : []
  }
}

/** 退出登录 */
export function fetchLogout() {
  return request.post<null>({ url: '/api/v1/auth/logout', showErrorMessage: false })
}

/** 个人资料 */
export function fetchGetProfile() {
  return request.get<WebBasic.Profile>({ url: '/api/v1/profile' })
}

/** 修改个人资料 */
export function fetchUpdateProfile(data: {
  nickname: string
  avatar?: string
  mobile?: string
  email?: string
  sex?: number
}) {
  return request.put<null>({
    url: '/api/v1/profile',
    params: data,
    successMessage: '资料已更新',
    showSuccessMessage: true
  })
}

/** 修改自己的密码 */
export function fetchChangePassword(data: { oldPassword: string; newPassword: string }) {
  return request.put<null>({
    url: '/api/v1/profile/password',
    params: data,
    showSuccessMessage: false
  })
}

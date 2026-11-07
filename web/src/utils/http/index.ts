/**
 * HTTP 请求封装模块（web-basic 适配版）
 *
 * 基于 Web Admin Pro 原生封装，做以下三处适配，其余行为与上游保持一致：
 *
 * 1. **鉴权头**：上游直接写入 accessToken，本项目 Go 后端要求 `Authorization: Bearer <token>`
 * 2. **响应体兼容**：上游约定 `{ code: 200, msg, data }`，Go 后端统一返回 `{ code: 0, message, data }`
 *    （同时兼容两套 code / msg 字段）
 * 3. **双 Token 静默刷新**：401 时先用 refreshToken 换新 accessToken 并重放原请求（并发单飞），
 *    刷新失败才登出
 *
 * 其余能力（401 防抖提示、统一错误提示、请求重试、成功提示）与上游保持一致。
 *
 * @module utils/http
 * @author Web Admin Pro Team (adapted by web-basic)
 */

import axios, { AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { useUserStore } from '@/store/modules/user'
import { ApiStatus } from './status'
import { HttpError, handleError, showError, showSuccess } from './error'
import { $t } from '@/locales'
import { BaseResponse } from '@/types'

/** 请求配置常量 */
const REQUEST_TIMEOUT = 15000
const LOGOUT_DELAY = 500
const MAX_RETRIES = 0
const RETRY_DELAY = 1000
const UNAUTHORIZED_DEBOUNCE_TIME = 3000

/** 业务成功码：上游 200 / 本项目后端 0 */
const SUCCESS_CODES = [0, ApiStatus.success]
/** 业务未授权码：上游 401 / 本项目后端 20001 */
const UNAUTHORIZED_CODES = [ApiStatus.unauthorized, 20001]

/** 401 防抖状态 */
let isUnauthorizedErrorShown = false
let unauthorizedTimer: NodeJS.Timeout | null = null

/** 刷新令牌单飞控制 */
let refreshPromise: Promise<string> | null = null

/** 扩展 AxiosRequestConfig */
interface ExtendedAxiosRequestConfig extends AxiosRequestConfig {
  showErrorMessage?: boolean
  showSuccessMessage?: boolean
  /** 自定义成功提示文案；不填则用后端 message（若后端只返回通用的 success 则回落「操作成功」） */
  successMessage?: string
  /** 内部标记：是否已因 token 过期重放过 */
  _retried?: boolean
}

const { VITE_API_URL, VITE_WITH_CREDENTIALS } = import.meta.env

/** 拼接后端绝对地址（兼容 VITE_API_URL 为空、为 / 或为完整域名三种情况） */
export function apiUrl(path: string): string {
  const prefix = String(VITE_API_URL || '').replace(/\/$/, '')
  return `${prefix}${path}`
}

/** Axios实例 */
const axiosInstance = axios.create({
  timeout: REQUEST_TIMEOUT,
  baseURL: VITE_API_URL,
  withCredentials: VITE_WITH_CREDENTIALS === 'true',
  validateStatus: (status) => status >= 200 && status < 300,
  transformResponse: [
    (data, headers) => {
      const contentType = headers['content-type']
      if (contentType?.includes('application/json')) {
        try {
          return JSON.parse(data)
        } catch {
          return data
        }
      }
      return data
    }
  ]
})

/** 请求拦截器 */
axiosInstance.interceptors.request.use(
  (request: InternalAxiosRequestConfig) => {
    const { accessToken } = useUserStore()
    // 适配点 1：Go 后端使用 Bearer 方案
    if (accessToken) request.headers.set('Authorization', `Bearer ${accessToken}`)

    if (request.data && !(request.data instanceof FormData) && !request.headers['Content-Type']) {
      request.headers.set('Content-Type', 'application/json')
      request.data = JSON.stringify(request.data)
    }

    return request
  },
  (error) => {
    showError(createHttpError($t('httpMsg.requestConfigError'), ApiStatus.error))
    return Promise.reject(error)
  }
)

/** 响应拦截器 */
axiosInstance.interceptors.response.use(
  (response: AxiosResponse<BaseResponse>) => {
    // 适配点 2：兼容 { code: 0, message } 与 { code: 200, msg }
    const body = response.data as BaseResponse & { message?: string }
    const code = body?.code
    const msg = body?.msg ?? body?.message

    if (SUCCESS_CODES.includes(code)) return response
    if (UNAUTHORIZED_CODES.includes(code)) throw createHttpError(msg || '', ApiStatus.unauthorized)
    throw createHttpError(msg || $t('httpMsg.requestFailed'), code)
  },
  async (error) => {
    const status = error.response?.status
    const originalConfig = error.config as ExtendedAxiosRequestConfig | undefined

    // 适配点 3：HTTP 401 时先尝试静默刷新，成功后重放原请求
    if (status === ApiStatus.unauthorized) {
      if (originalConfig && !originalConfig._retried) {
        originalConfig._retried = true
        try {
          const token = await refreshAccessToken()
          originalConfig.headers = {
            ...(originalConfig.headers || {}),
            Authorization: `Bearer ${token}`
          }
          return axiosInstance.request(originalConfig)
        } catch {
          // 刷新失败 → 下方统一走登出流程
        }
      }
      handleUnauthorizedError()
    }

    return Promise.reject(handleError(error))
  }
)

/**
 * 用 refreshToken 换取新的 accessToken（并发请求共享同一个 Promise）
 */
async function refreshAccessToken(): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const userStore = useUserStore()
      const refreshToken = userStore.refreshToken
      if (!refreshToken) throw new Error('no refresh token')

      const { data } = await axios.post(apiUrl('/api/v1/auth/refresh'), { refreshToken })
      const ok = SUCCESS_CODES.includes(data?.code)
      if (!ok || !data?.data?.accessToken) throw new Error(data?.message || 'refresh failed')

      userStore.setToken(data.data.accessToken, data.data.refreshToken)
      return data.data.accessToken as string
    })().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

/** 统一创建HttpError */
function createHttpError(message: string, code: number) {
  return new HttpError(message, code)
}

/** 处理401错误（带防抖） */
function handleUnauthorizedError(message?: string): never {
  const error = createHttpError(message || $t('httpMsg.unauthorized'), ApiStatus.unauthorized)

  if (!isUnauthorizedErrorShown) {
    isUnauthorizedErrorShown = true
    logOut()

    unauthorizedTimer = setTimeout(resetUnauthorizedError, UNAUTHORIZED_DEBOUNCE_TIME)

    showError(error, true)
    throw error
  }

  throw error
}

/** 重置401防抖状态 */
function resetUnauthorizedError() {
  isUnauthorizedErrorShown = false
  if (unauthorizedTimer) clearTimeout(unauthorizedTimer)
  unauthorizedTimer = null
}

/** 退出登录函数 */
function logOut() {
  setTimeout(() => {
    useUserStore().logOut()
  }, LOGOUT_DELAY)
}

/** 是否需要重试 */
function shouldRetry(statusCode: number) {
  return [
    ApiStatus.requestTimeout,
    ApiStatus.internalServerError,
    ApiStatus.badGateway,
    ApiStatus.serviceUnavailable,
    ApiStatus.gatewayTimeout
  ].includes(statusCode)
}

/** 请求重试逻辑 */
async function retryRequest<T>(
  config: ExtendedAxiosRequestConfig,
  retries: number = MAX_RETRIES
): Promise<T> {
  try {
    return await request<T>(config)
  } catch (error) {
    if (retries > 0 && error instanceof HttpError && shouldRetry(error.code)) {
      await delay(RETRY_DELAY)
      return retryRequest<T>(config, retries - 1)
    }
    throw error
  }
}

/** 延迟函数 */
function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** 请求函数 */
async function request<T = any>(config: ExtendedAxiosRequestConfig): Promise<T> {
  // POST | PUT 参数自动填充
  if (
    ['POST', 'PUT'].includes(config.method?.toUpperCase() || '') &&
    config.params &&
    !config.data
  ) {
    config.data = config.params
    config.params = undefined
  }

  try {
    const res = await axiosInstance.request<BaseResponse<T>>(config)
    const body = res.data as BaseResponse<T> & { message?: string }

    // 显示成功消息：优先用接口定义的自定义文案，其次用后端返回的业务文案；
    // 后端统一返回的通用 "success" 对用户没有意义，回落为「操作成功」。
    if (config.showSuccessMessage) {
      const backendMsg = String(body?.msg ?? body?.message ?? '').trim()
      const isGeneric = !backendMsg || /^(success|ok|操作成功)$/i.test(backendMsg)
      showSuccess(config.successMessage || (isGeneric ? $t('httpMsg.operationSuccess') : backendMsg))
    }

    return body.data as T
  } catch (error) {
    if (error instanceof HttpError && error.code !== ApiStatus.unauthorized) {
      const showMsg = config.showErrorMessage !== false
      showError(error, showMsg)
    }
    return Promise.reject(error)
  }
}

/** API方法集合 */
const api = {
  get<T>(config: ExtendedAxiosRequestConfig) {
    return retryRequest<T>({ ...config, method: 'GET' })
  },
  post<T>(config: ExtendedAxiosRequestConfig) {
    return retryRequest<T>({ ...config, method: 'POST' })
  },
  put<T>(config: ExtendedAxiosRequestConfig) {
    return retryRequest<T>({ ...config, method: 'PUT' })
  },
  del<T>(config: ExtendedAxiosRequestConfig) {
    return retryRequest<T>({ ...config, method: 'DELETE' })
  },
  request<T>(config: ExtendedAxiosRequestConfig) {
    return retryRequest<T>(config)
  }
}

export default api

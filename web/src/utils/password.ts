/**
 * 密码强度校验（与后端 crypto.CheckStrength 完全一致）
 *
 * 目的：把「密码太弱」这类业务规则在前端就讲清楚，而不是提交后看到一条含糊的错误提示。
 * 规则来源：GET /api/v1/public/password-policy（后端 auth.pwd_min_len 与策略常量）。
 *
 * @module utils/password
 */
import { fetchPasswordPolicy, type PasswordPolicy } from '@/api/auth'

/** 默认策略（接口不可用时的兜底，与后端默认值保持一致） */
export const DEFAULT_PASSWORD_POLICY: PasswordPolicy = {
  minLength: 8,
  requireUpper: true,
  requireLower: true,
  requireDigit: true,
  description: '至少 8 位，且同时包含大写字母、小写字母和数字'
}

let cached: PasswordPolicy | null = null
let loading: Promise<PasswordPolicy> | null = null

/** 常见弱口令（与后端黑名单一致，用于即时提示） */
const WEAK = new Set([
  '12345678',
  '123456789',
  '1234567890',
  '87654321',
  'password',
  'passw0rd',
  'password1',
  'p@ssw0rd',
  'admin123',
  'admin@123',
  'administrator',
  'qwerty123',
  '1qaz2wsx',
  'abc12345',
  'a1234567',
  'iloveyou',
  'welcome1',
  'changeme',
  'letmein1',
  'test1234'
])

/** 读取密码策略（带缓存，只请求一次） */
export async function getPasswordPolicy(): Promise<PasswordPolicy> {
  if (cached) return cached
  if (!loading) {
    loading = fetchPasswordPolicy()
      .then((p) => {
        cached = { ...DEFAULT_PASSWORD_POLICY, ...(p || {}) }
        return cached
      })
      .catch(() => DEFAULT_PASSWORD_POLICY)
      .finally(() => {
        loading = null
      })
  }
  return loading
}

/**
 * 校验密码强度
 * @returns 通过返回空字符串；不通过返回给用户看的原因
 */
export function checkPassword(pwd: string, policy: PasswordPolicy = DEFAULT_PASSWORD_POLICY): string {
  const value = pwd || ''
  if (value.length < policy.minLength) {
    return `密码长度不能少于 ${policy.minLength} 位（当前 ${value.length} 位）`
  }
  if (value.length > 64) return '密码长度不能超过 64 位'

  const hasUpper = /[A-Z]/.test(value)
  const hasLower = /[a-z]/.test(value)
  const hasDigit = /[0-9]/.test(value)
  if (policy.requireUpper && !hasUpper) return '密码还需要包含：大写字母'
  if (policy.requireLower && !hasLower) return '密码还需要包含：小写字母'
  if (policy.requireDigit && !hasDigit) return '密码还需要包含：数字'

  if (WEAK.has(value.toLowerCase())) return '该密码属于常见弱口令，请更换为更难猜测的密码'
  if (/(.)\1{2,}/.test(value)) return '密码不能包含 3 个及以上连续相同的字符'
  return ''
}

/** 生成 Element Plus 表单校验器（blur 时立即提示，不用等提交） */
export function passwordValidator() {
  return (_rule: unknown, value: string, callback: (e?: Error) => void) => {
    const msg = checkPassword(value)
    callback(msg ? new Error(msg) : undefined)
  }
}

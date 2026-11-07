import { h } from 'vue'

/**
 * web-basic 前端通用工具
 *
 * @module utils/web-basic
 */

/**
 * 表格操作列容器：把多个「图标按钮」水平排列且禁止换行。
 *
 * 注意：ArtButtonTable 是 inline-flex + `mr-2.5`，直接塞进块级 div 时，
 * 列宽不足会「换行成两行」把整行撑高（本项目踩过的坑）。
 * 这里用 flex + nowrap + 固定间距；列宽建议 ≥ 140 才能容纳 2 个按钮。
 */
export function actionCell(...nodes: unknown[]) {
  return h('div', { style: 'display:flex;flex-wrap:nowrap;align-items:center;gap:2px' }, nodes as never[])
}
/** 字节数格式化为易读大小 */
export function formatSize(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

/**
 * 时间格式化：统一把后端时间转成「人看的」格式 `YYYY-MM-DD HH:mm:ss`
 *
 * 兼容三种入参：
 * - RFC3339 / ISO 字符串：`2026-11-01T00:32:52+08:00`
 * - Unix 时间戳（秒，10 位）或毫秒（13 位）数字
 * - 已经格式化好的字符串（原样返回）
 */
export function formatTime(value?: string | number | null): string {
  if (value === null || value === undefined || value === '') return '-'
  let d: Date
  if (typeof value === 'number') {
    d = new Date(value < 1e12 ? value * 1000 : value)
  } else {
    const num = Number(value)
    // 纯数字字符串也按时间戳处理
    d = !Number.isNaN(num) && String(value).trim() !== '' && /^\d+$/.test(value) ? new Date(num < 1e12 ? num * 1000 : num) : new Date(value)
  }
  if (Number.isNaN(d.getTime())) return String(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes()
  )}:${pad(d.getSeconds())}`
}

/** 仅日期：YYYY-MM-DD */
export function formatDate(value?: string | number | null): string {
  const full = formatTime(value)
  return full === '-' ? '-' : full.slice(0, 10)
}

/** 相对时间（如「3 分钟前」），超过 30 天回退为绝对时间 */
export function formatRelativeTime(value?: string | number | null): string {
  if (value === null || value === undefined || value === '') return '-'
  const full = formatTime(value)
  if (full === '-') return '-'
  const t = new Date(full.replace(' ', 'T')).getTime()
  if (Number.isNaN(t)) return full
  const diff = Date.now() - t
  if (diff < 0) return full
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  const day = Math.floor(hour / 24)
  if (day < 30) return `${day} 天前`
  return full
}

/** Unix 秒 → YYYY-MM-DD HH:mm:ss */
export function formatUnix(seconds?: number): string {
  if (!seconds) return '-'
  return formatTime(new Date(seconds * 1000).toISOString())
}

/** 数据范围文案 */
export const DATA_SCOPE_TEXT: Record<number, string> = {
  1: '全部数据',
  2: '本部门数据',
  3: '本部门及以下',
  4: '仅本人数据',
  5: '自定义部门'
}

/** 性别文案 */
export const SEX_TEXT: Record<number, string> = { 0: '未知', 1: '男', 2: '女' }

/**
 * 带鉴权的文件下载（Excel 导出等）
 *
 * 为什么不用 axios 封装：后端返回的是二进制流，而 http 封装会把响应体当 JSON 解包，
 * 这里直接用 fetch 拿 Blob，再触发浏览器下载。文件名优先用后端 Content-Disposition。
 *
 * @param path 接口路径（不含 VITE_API_URL 前缀）
 * @param params 查询参数
 * @param fallbackName 后端没给文件名时的兜底名
 */
export async function downloadFile(
  path: string,
  params: Record<string, any> = {},
  fallbackName = 'export.xlsx'
): Promise<void> {
  const { apiUrl } = await import('@/utils/http')
  const { useUserStore } = await import('@/store/modules/user')
  const query = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') query.append(k, String(v))
  })
  const qs = query.toString()
  const token = useUserStore().accessToken
  const res = await fetch(apiUrl(path) + (qs ? `?${qs}` : ''), {
    headers: token ? { Authorization: `Bearer ${token}` } : {}
  })
  if (!res.ok) {
    // 后端出错时返回的是 JSON 业务错误，尽量把 message 透出来
    let msg = `下载失败（HTTP ${res.status}）`
    try {
      const body = await res.json()
      msg = body?.message || body?.msg || msg
    } catch {
      /* 非 JSON，忽略 */
    }
    throw new Error(msg)
  }
  const blob = await res.blob()
  const disposition = res.headers.get('content-disposition') || ''
  const utf8Match = /filename\*=UTF-8''([^;]+)/i.exec(disposition)
  const plainMatch = /filename="?([^";]+)"?/i.exec(disposition)
  const filename = utf8Match
    ? decodeURIComponent(utf8Match[1])
    : plainMatch
      ? plainMatch[1]
      : fallbackName

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

/** 带鉴权的文件上传（multipart/form-data，Excel 导入等） */
export async function uploadFile<T = any>(path: string, file: File): Promise<T> {
  const { apiUrl } = await import('@/utils/http')
  const { useUserStore } = await import('@/store/modules/user')
  const token = useUserStore().accessToken
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(apiUrl(path), {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form
  })
  const body = await res.json().catch(() => null)
  if (!res.ok || (body && body.code !== 0)) {
    throw new Error(body?.message || body?.msg || `上传失败（HTTP ${res.status}）`)
  }
  return (body?.data ?? null) as T
}
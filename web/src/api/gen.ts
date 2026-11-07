import request from '@/utils/http'

/** 代码生成器：库表清单 */
export function fetchGenTables() {
  return request.get<WebBasic.GenTable[]>({ url: '/api/v1/tool/gen/tables' })
}

/** 表字段详情 */
export function fetchGenTableDetail(table: string) {
  return request.get<WebBasic.GenTable>({ url: `/api/v1/tool/gen/tables/${table}` })
}

/** 预览生成代码 */
export function fetchGenPreview(table: string) {
  return request.get<WebBasic.GenFile[]>({ url: `/api/v1/tool/gen/tables/${table}/preview` })
}

/** 打包下载生成的代码（zip） */
export async function downloadGenCode(tables: string[]) {
  // 下载接口是 POST + JSON body，fetch 直接拿 Blob（http 封装会把响应体当 JSON 解包）
  const { apiUrl } = await import('@/utils/http')
  const { useUserStore } = await import('@/store/modules/user')
  const token = useUserStore().accessToken
  const res = await fetch(apiUrl('/api/v1/tool/gen/download'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: JSON.stringify({ tables })
  })
  if (!res.ok) throw new Error(`下载失败（HTTP ${res.status}）`)
  const blob = await res.blob()
  const disposition = res.headers.get('content-disposition') || ''
  const m = /filename\*=UTF-8''([^;]+)/i.exec(disposition)
  const filename = m ? decodeURIComponent(m[1]) : 'codegen.zip'
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

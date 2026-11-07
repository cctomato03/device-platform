/**
 * 文件上传（统一入口）
 *
 * 策略：优先「前端直传 OSS」，不可用时自动回退「服务端中转上传」。
 *
 *   1. 调后端 /files/oss-ticket 拿签名（本地磁盘存储会返回「不支持」）；
 *   2. 拿到签名后浏览器直接 PUT 到 OSS（不占用应用服务器上行带宽）；
 *   3. 上传成功后调 /files/oss-record 把对象键登记进文件表；
 *   4. 1~3 任一步失败（CORS 未配置、签名过期、网络异常、存储非 OSS）→ 回退 /files/upload。
 *
 * OSS 侧需配置 CORS：允许站点域名发起 PUT，允许 Content-Type 请求头。
 *
 * @module utils/upload
 */
import {
  fetchGetOssTicket,
  fetchRecordOssFile,
  fetchUploadFile,
  fetchGetPublicConfig,
  mapFile
} from '@/api/business'

export interface UploadOptions {
  /** 业务类型，决定对象键里的目录（如 avatar / document / site） */
  bizType?: string
  /** 是否静默成功提示 */
  silent?: boolean
}

/**
 * 后端是否支持直传（公开参数 sys.upload.direct，由服务端存储配置决定）。
 * 缓存一次，避免每次上传都多一个请求；本地存储时直接走中转上传，不产生无意义的 400。
 */
let directSupported: boolean | null = null

async function canDirectUpload(): Promise<boolean> {
  if (directSupported !== null) return directSupported
  try {
    const cfg = await fetchGetPublicConfig()
    directSupported = `${cfg?.['sys.upload.direct']}` === 'true'
  } catch {
    directSupported = false
  }
  return directSupported
}

/**
 * 上传一个文件，返回文件记录（url / objectKey 等）。
 *
 * @param file   浏览器 File 对象
 * @param options 业务类型等选项
 */
export async function uploadFile(
  file: File,
  options: UploadOptions = {}
): Promise<WebBasic.FileItem> {
  const bizType = options.bizType || 'default'

  if (await canDirectUpload()) {
    const ticket = await fetchGetOssTicket({
      originName: file.name,
      size: file.size,
      bizType
    }).catch(() => null)

    if (ticket?.uploadUrl) {
      try {
        const res = await fetch(ticket.uploadUrl, {
          method: ticket.method || 'PUT',
          headers: ticket.headers || {},
          body: file
        })
        if (!res.ok) throw new Error(`OSS 直传失败：HTTP ${res.status}`)
        return await fetchRecordOssFile({
          objectKey: ticket.objectKey,
          originName: file.name,
          size: file.size,
          mime: file.type,
          bizType
        })
      } catch (err) {
        // 直传失败不抛错：静默回退到服务端上传，用户无感
        console.warn('[upload] OSS 直传失败，回退服务端上传:', err)
      }
    }
  }

  const uploaded = await fetchUploadFile(file, bizType)
  return mapFile(uploaded)
}

export default uploadFile

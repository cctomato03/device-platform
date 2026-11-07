/**
 * 站点个性化配置接口
 *
 * - 公开接口：登录页渲染前读取网站名称 / logo / 登录页背景与文案 / 注册开关
 * - 后台接口：系统设置 → 站点设置 页面的读取与保存
 *
 * @module api/site
 */
import request from '@/utils/http'

/** 公开读取（免登录，失败不弹错误提示：后台还没配置时要能静默降级） */
export function fetchPublicSiteConfig() {
  return request.get<WebBasic.SiteConfig>({
    url: '/api/v1/public/site-config',
    showErrorMessage: false
  })
}

/** 后台读取（需 system:site:list） */
export function fetchSiteConfig() {
  return request.get<WebBasic.SiteConfig>({ url: '/api/v1/system/site-config' })
}

/** 保存配置（需 system:site:update） */
export function fetchUpdateSiteConfig(data: WebBasic.SiteConfig) {
  return request.put<WebBasic.SiteConfig>({
    url: '/api/v1/system/site-config',
    data,
    successMessage: '站点配置已保存',
    showSuccessMessage: true
  })
}

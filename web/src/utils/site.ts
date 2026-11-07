/**
 * 站点个性化配置（运行时）
 *
 * 后端 `sys_site_config` 单行表 → 前端启动时读取一次并写入全局：
 * - 网站名称（顶栏/登录页/浏览器标题）
 * - 站点 logo（为空时用内置 logo）
 * - 登录页背景图与文字描述
 * - 注册开关（关闭时隐藏登录页注册入口，并拦截至 /auth/register 的路由）
 *
 * 之所以用普通模块而不是 pinia store：登录页/路由守卫都需要在读 pinia 之前拿到它，
 * 模块级 reactive 对象在任何地方 import 都能直接用。
 *
 * @module utils/site
 */
import { reactive } from 'vue'
import AppConfig from '@/config'
import { fetchPublicSiteConfig } from '@/api/site'

/** 内置默认值（后端接口不可用时降级，保证登录页一定能渲染） */
const DEFAULTS = {
  siteName: AppConfig.systemInfo.name,
  siteLogo: '',
  loginBg: '',
  loginDesc: '',
  loginSubTitle: '',
  registerEnabled: false
}

/** 站点配置（响应式，任意组件可直接用） */
export const siteConfig = reactive({ ...DEFAULTS })

/** 是否已从后端同步过（避免重复请求） */
let loaded = false

/** 把后端配置写入全局（含浏览器标题用到的 AppConfig） */
export function applySiteConfig(cfg?: Partial<WebBasic.SiteConfig> | null): void {
  const name = (cfg?.siteName || '').trim() || DEFAULTS.siteName
  siteConfig.siteName = name
  siteConfig.siteLogo = cfg?.siteLogo || ''
  siteConfig.loginBg = cfg?.loginBg || ''
  siteConfig.loginDesc = cfg?.loginDesc || ''
  siteConfig.loginSubTitle = cfg?.loginSubTitle || ''
  siteConfig.registerEnabled = cfg?.registerEnabled === true

  // 模板的标题、登录页标题等都读 AppConfig.systemInfo.name，这里一次性对齐
  AppConfig.systemInfo.name = name
}

/** 启动时拉取站点配置（失败则使用默认值，绝不阻塞应用启动） */
export async function initSiteConfig(force = false): Promise<void> {
  if (loaded && !force) return
  loaded = true
  try {
    const cfg = await fetchPublicSiteConfig()
    applySiteConfig(cfg)
  } catch {
    applySiteConfig(null)
  }
}

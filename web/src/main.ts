import App from './App.vue'
import { createApp } from 'vue'
import { initStore } from './store'                 // Store
import { initRouter } from './router'               // Router
import language from './locales'                    // 国际化
import '@styles/core/tailwind.css'                  // tailwind
import '@styles/index.scss'                         // 样式
import '@utils/ui/iconify-loader'                   // Iconify 离线图标集（ep / ri，去除 CDN 依赖）
import '@utils/sys/console.ts'                      // 控制台输出内容
import { setupGlobDirectives } from './directives'
import { setupErrorHandle } from './utils/sys/error-handle'
import { initSiteConfig } from './utils/site'   // 站点个性化配置（启动前同步）

document.addEventListener(
  'touchstart',
  function () {},
  { passive: false }
)

/**
 * 启动流程：先同步「站点个性化配置」（网站名称/logo/登录页背景与文案/注册开关），
 * 再创建应用 —— 这样登录页首帧就是配置后的样子，不会出现名称闪烁。
 * 接口失败时自动降级为内置默认值，绝不会阻塞启动。
 */
async function bootstrap() {
  const app = createApp(App)
  // 注意顺序：initStore 必须先执行——http 请求拦截器里 useUserStore() 依赖已安装的 pinia，
  // 否则读取站点配置会抛 "no active pinia" 而静默降级成默认名称。
  initStore(app)
  await initSiteConfig()

  initRouter(app)
  setupGlobDirectives(app)
  setupErrorHandle(app)

  app.use(language)
  app.mount('#app')
}

bootstrap()
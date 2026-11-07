import { AppRouteRecord } from '@/types/router'
import { dashboardRoutes } from './dashboard'
import { systemRoutes } from './system'
import { exceptionRoutes } from './exception'

/**
 * 模块化路由聚合
 *
 * 说明：本项目采用「后端菜单驱动」模式（VITE_ACCESS_MODE=backend），
 * 运行时路由来自 GET /api/v1/auth/routes；这里的静态模块只用于「前端菜单模式」兜底，
 * 因此只保留实际存在的模块，不再包含模板自带的示例页。
 */
export const routeModules: AppRouteRecord[] = [
  dashboardRoutes,
  systemRoutes,
  exceptionRoutes
]

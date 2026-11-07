/**
 * 公共路由别名
 * 存放系统级公共路由路径，如布局容器、登录页等
 */
export enum RoutesAlias {
  Layout = '/index/index', // 布局容器
  Login = '/auth/login', // 登录页
  Register = '/auth/register' // 注册页（受站点配置的注册开关控制）
}

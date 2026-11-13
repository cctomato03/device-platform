/**
 * 本地存储键常量集中定义
 *
 * 分散在业务代码里的 `localStorage.setItem('xxx')` 容易出现拼写不一致、
 * 升级后散落遗留键的问题；这里统一命名，便于检索与清理。
 *
 * @module utils/storage/storage-config
 */
export const StorageConfig = {
  /** 最近一次登录的用户 ID（切换账号时用于清理上一个用户的页签/设置） */
  LAST_USER_ID_KEY: 'sys-last-user-id',
  /** 当前前端资源版本（用于检测前端升级） */
  APP_VERSION_KEY: 'sys-app-version',
  /** 侧边栏折叠状态 */
  SIDEBAR_COLLAPSED_KEY: 'sys-sidebar-collapsed',
  /** 主题模式（明/暗/跟随系统） */
  THEME_MODE_KEY: 'sys-theme-mode',
  /** 主题配置（主题色、圆角、风格等，设置面板持久化） */
  THEME_KEY: 'sys-theme',
  /** 响应式布局下侧边栏菜单类型（折叠/展开的个性化记忆） */
  RESPONSIVE_MENU_TYPE_KEY: 'sys-responsive-menu-type',
  /** 界面语言 */
  LANGUAGE_KEY: 'sys-language',
  /** 表格列设置前缀（配合表格 id 使用） */
  TABLE_SETTING_PREFIX: 'sys-table-setting',
  /** 用户搜索历史前缀 */
  SEARCH_HISTORY_PREFIX: 'sys-search-history'
} as const

export type StorageConfigKey = (typeof StorageConfig)[keyof typeof StorageConfig]

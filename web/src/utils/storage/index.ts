/**
 * 本地存储工具入口
 *
 * 提供：
 * - checkStorageCompatibility：启动时校验 localStorage 可用性与前端版本变化，
 *   版本变化时清理已废弃的版本化键，避免旧结构数据把新界面搞崩；
 * - StorageKeyManager / StorageConfig 的统一出口。
 *
 * @module utils/storage
 */
import { APP_VERSION, StorageKeyManager } from './storage-key-manager'
import { StorageConfig } from './storage-config'

export { APP_VERSION, StorageKeyManager } from './storage-key-manager'
export { StorageConfig } from './storage-config'

/** 版本化键前缀：sys-v{version}- */
const VERSIONED_PREFIX = 'sys-v'

/**
 * 检测并要求清理不兼容的本地存储
 *
 * 场景：前端发版后，旧版本持久化的 store 结构与新代码不兼容（例如表格列配置、
 * 用户信息字段变化），会表现为「页面空白 / 报错」。这里在应用挂载时做一次检查：
 * 记录到 localStorage 的前端版本与当前不一致时，删除所有 `sys-v*` 开头的旧键。
 *
 * 注意：只清理「版本化」键，不动用户手工保存的其它数据。
 */
export function checkStorageCompatibility(): void {
  if (typeof window === 'undefined' || !window.localStorage) return

  try {
    const stored = localStorage.getItem(StorageConfig.APP_VERSION_KEY)
    if (stored === APP_VERSION) return

    if (stored) {
      // 版本变化：清理所有旧版本化键（sys-v1.0.0-xxx …）
      const stale: string[] = []
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i)
        if (key && key.startsWith(VERSIONED_PREFIX) && !key.startsWith(`${VERSIONED_PREFIX}${APP_VERSION}-`)) {
          stale.push(key)
        }
      }
      stale.forEach((key) => localStorage.removeItem(key))
    }

    localStorage.setItem(StorageConfig.APP_VERSION_KEY, APP_VERSION)
  } catch {
    // 隐私模式 / 配额不足：静默跳过，不影响页面使用
  }
}

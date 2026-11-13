/**
 * 本地存储键管理（版本化）
 *
 * 目的：前端持久化数据（Pinia store / 表格列设置等）在升级后可能与新结构不兼容，
 * 这里给存储键带上版本号 `sys-v{version}-{storeId}`，并在首次运行时把
 * 旧版本（无版本号或旧版本号）的键自动迁移到当前版本，避免用户设置丢失。
 *
 * @module utils/storage/storage-key-manager
 */

/** 当前前端版本号（由构建注入，缺省 0.0.0） */
export const APP_VERSION: string = (import.meta.env.VITE_VERSION as string) || '0.0.0'

/** 存储键前缀 */
const PREFIX = 'sys'

/** 历史版本号（用于迁移，升版时把上一版本追加到末尾即可） */
const HISTORY_VERSIONS = ['1.0.0', '2.0.0', '3.0.0']

export class StorageKeyManager {
  private readonly version: string

  constructor(version: string = APP_VERSION) {
    this.version = version
  }

  /**
   * 生成带版本号的存储键
   * @param storeId 业务标识（一般就是 store 的 id）
   */
  getStorageKey(storeId: string): string {
    return `${PREFIX}-v${this.version}-${storeId}`
  }

  /**
   * 迁移旧版本数据到当前版本键
   *
   * 规则：当前键不存在、而历史键存在时，把历史键的值复制过来（只做一次，
   * 复制成功后保留旧键，便于用户回滚版本）。
   */
  migrate(storeId: string): void {
    if (typeof localStorage === 'undefined') return
    const currentKey = this.getStorageKey(storeId)
    if (localStorage.getItem(currentKey) !== null) return

    const candidates = [
      `${PREFIX}-${storeId}`,
      ...HISTORY_VERSIONS.map((v) => `${PREFIX}-v${v}-${storeId}`)
    ]
    for (const key of candidates) {
      const value = localStorage.getItem(key)
      if (value !== null) {
        try {
          localStorage.setItem(currentKey, value)
        } catch {
          // 配额不足时忽略，不影响页面运行
        }
        break
      }
    }
  }

  /** 清理指定业务的历史键（调试或用户重置设置时使用） */
  clearHistory(storeId: string): void {
    if (typeof localStorage === 'undefined') return
    const candidates = [
      `${PREFIX}-${storeId}`,
      ...HISTORY_VERSIONS.map((v) => `${PREFIX}-v${v}-${storeId}`)
    ]
    candidates.forEach((key) => localStorage.removeItem(key))
  }
}

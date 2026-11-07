/**
 * 离线图标加载器
 *
 * 内网 / 无外网环境下 Iconify 默认会去 `https://api.iconify.design` 取图标，
 * 取不到就是空白。这里把项目**实际用到的图标**打包进产物，彻底去掉运行时外网依赖。
 *
 * 数据来源：
 * - `iconify-subset.ts` 由 `node scripts/gen-icon-subset.mjs` 扫描源码自动生成
 *   （ep 57+/ri 150+ 个图标，约 60KB，远小于整包 1.1MB+）
 * - 新增图标后重新执行生成脚本即可；也可临时用整包：
 *   `import epIcons from '@iconify-json/ep/icons.json'; addCollection(epIcons)`
 *
 * 自定义图标 / iconfont 接入见 docs/08-二次开发指南.md「图标体系」。
 *
 * @module utils/ui/iconify-loader
 */
import { addCollection } from '@iconify/vue'
import { epSubset, riSubset } from './iconify-subset'

/** 注册离线图标集（重复调用无副作用） */
export function setupOfflineIcons(): void {
  addCollection(epSubset as never)
  addCollection(riSubset as never)
}

setupOfflineIcons()

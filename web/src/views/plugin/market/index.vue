<!-- 插件市场：安装 / 启停 / 配置 / 日志 / 卸载（schema 驱动的统一配置界面） -->
<template>
  <div class="art-full-height">
    <ElCard class="mb-3">
      <template #header>
        <div class="flex-cb">
          <span>插件市场</span>
          <ElSpace>
            <ElTag v-if="meta.dockerAvailable" type="success" size="small">Docker 可用</ElTag>
            <ElTag v-else type="info" size="small">Docker 不可用</ElTag>
            <ElTag type="info" size="small">{{ meta.arch }}</ElTag>
            <ElUpload
              v-auth="'plugin:market:install'"
              :show-file-list="false"
              :auto-upload="false"
              accept=".zip,.tar.gz,.tgz"
              :on-change="onFileChange"
            >
              <ElButton size="small" type="primary" :loading="uploading">上传插件包</ElButton>
            </ElUpload>
            <ElButton size="small" @click="loadAll">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTabs v-model="tab">
        <ElTabPane :label="`已安装 (${plugins.length})`" name="installed">
          <ElEmpty v-if="!plugins.length" description="还没有安装插件，可从「可安装」里选择平台自带插件，或上传插件包" />
          <ElRow :gutter="12">
            <ElCol v-for="p in plugins" :key="p.key" :xs="24" :sm="12" :md="8" :lg="6">
              <ElCard class="plugin-card mb-3" shadow="hover">
                <div class="flex-cb">
                  <div class="flex items-center gap-2">
                    <ArtSvgIcon :icon="p.icon || 'ri:apps-2-line'" class="text-2xl text-theme" />
                    <div>
                      <p class="font-medium">{{ p.name }}</p>
                      <p class="text-xs text-g-400">v{{ p.version }} · {{ p.runtime }}</p>
                    </div>
                  </div>
                  <ElTag :type="statusType(p.status)" size="small">{{ statusLabel(p.status) }}</ElTag>
                </div>

                <p class="plugin-desc text-xs text-g-500 mt-2">{{ p.description || '无描述' }}</p>

                <div class="mt-2 flex flex-wrap gap-1">
                  <ElTag size="small" type="info">{{ p.category || '未分类' }}</ElTag>
                  <ElTag v-if="p.pid" size="small">PID {{ p.pid }}</ElTag>
                  <ElTag v-if="p.autoStart" size="small" type="success">自启</ElTag>
                  <ElTag v-if="!p.enabled" size="small" type="warning">已禁用</ElTag>
                </div>

                <ElAlert
                  v-if="p.lastError"
                  type="error"
                  :closable="false"
                  class="mt-2"
                  :title="p.lastError"
                />

                <div class="mt-3 flex flex-wrap gap-1">
                  <ElButton
                    v-auth="'plugin:lifecycle:update'"
                    size="small"
                    type="success"
                    :disabled="p.status === 'running'"
                    @click="onAction(p, 'start')"
                  >
                    启动
                  </ElButton>
                  <ElButton
                    v-auth="'plugin:lifecycle:update'"
                    size="small"
                    :disabled="p.status !== 'running'"
                    @click="onAction(p, 'stop')"
                  >
                    停止
                  </ElButton>
                  <ElButton v-auth="'plugin:lifecycle:update'" size="small" @click="onAction(p, 'restart')">
                    重启
                  </ElButton>
                  <ElButton v-auth="'plugin:config:update'" size="small" type="primary" @click="openConfig(p)">
                    配置
                  </ElButton>
                  <ElButton v-auth="'plugin:log:view'" size="small" @click="openLogs(p)">日志</ElButton>
                  <ElButton
                    v-auth="'plugin:lifecycle:update'"
                    size="small"
                    link
                    @click="onAction(p, p.enabled ? 'disable' : 'enable')"
                  >
                    {{ p.enabled ? '禁用' : '启用' }}
                  </ElButton>
                  <ElButton
                    v-auth="'plugin:lifecycle:update'"
                    size="small"
                    link
                    @click="onAutoStart(p, !p.autoStart)"
                  >
                    {{ p.autoStart ? '关自启' : '开自启' }}
                  </ElButton>
                  <ElButton v-auth="'plugin:market:uninstall'" size="small" link type="danger" @click="onUninstall(p)">
                    卸载
                  </ElButton>
                </div>
              </ElCard>
            </ElCol>
          </ElRow>
        </ElTabPane>

        <ElTabPane :label="`在线市场 (${market.length})`" name="market">
          <ElAlert type="info" :closable="false" class="mb-3">
            在线市场来自「插件源」里配置的索引地址；安装前会校验索引签名与插件包 sha256。
            未配置插件源时，可先在「插件源」页添加索引。
          </ElAlert>
          <ElTable v-loading="marketLoading" :data="market" size="small">
            <ElTableColumn prop="name" label="插件" min-width="150" />
            <ElTableColumn prop="version" label="版本" width="100" />
            <ElTableColumn prop="category" label="分类" width="100" />
            <ElTableColumn prop="sourceName" label="来源" width="130" />
            <ElTableColumn label="已装版本" width="110">
              <template #default="{ row }">{{ row.installedVersion || '-' }}</template>
            </ElTableColumn>
            <ElTableColumn label="架构" width="120">
              <template #default="{ row }">
                <ElTag size="small" :type="row.compatible ? 'success' : 'danger'">
                  {{ row.compatible ? '兼容' : '不兼容' }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <ElButton
                  v-auth="'plugin:market:install'"
                  size="small"
                  type="primary"
                  :disabled="!row.compatible"
                  :loading="installingId === row.id"
                  @click="onInstallMarket(row)"
                >
                  {{ row.upgradable ? '升级' : row.installed ? '已安装' : '安装' }}
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-3">
            <ElButton size="small" :loading="marketLoading" @click="loadMarket(true)">刷新索引</ElButton>
          </div>
        </ElTabPane>

        <ElTabPane :label="`可安装 (${bundled.length})`" name="bundled">
          <ElAlert type="info" :closable="false" class="mb-3">
            平台自带插件来自安装目录 <code>{{ meta.pluginDir }}/../plugins-bundled</code>；
            也可以上传任意符合规范的插件包（.zip / .tar.gz）。插件规范见「开发手册 → 插件开发规范」。
          </ElAlert>
          <ElEmpty v-if="!bundled.length" description="没有可安装的内置插件" />
          <ElRow :gutter="12">
            <ElCol v-for="b in bundled" :key="b.key" :xs="24" :sm="12" :md="8">
              <ElCard class="mb-3" shadow="hover">
                <div class="flex-cb">
                  <div>
                    <p class="font-medium">{{ b.name }}</p>
                    <p class="text-xs text-g-400">v{{ b.version }} · {{ b.category }}</p>
                  </div>
                  <ElTag v-if="b.installed" type="success" size="small">已安装</ElTag>
                </div>
                <p class="text-xs text-g-500 mt-2">{{ b.description }}</p>
                <div class="mt-3">
                  <ElButton
                    v-auth="'plugin:market:install'"
                    size="small"
                    type="primary"
                    :disabled="b.installed"
                    @click="onInstallBundled(b)"
                  >
                    {{ b.installed ? '已安装' : '安装' }}
                  </ElButton>
                </div>
              </ElCard>
            </ElCol>
          </ElRow>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <!-- 配置抽屉：按 manifest.schema 动态渲染 -->
    <ElDrawer v-model="configVisible" :title="`插件配置 · ${current?.name || ''}`" size="560px">
      <ElForm label-width="120px">
        <template v-for="field in schema" :key="field.key">
          <!-- 列表类型（如 frp 的端口映射） -->
          <ElFormItem v-if="field.type === 'list'" :label="field.label">
            <div class="w-full">
              <p v-if="field.help" class="text-xs text-g-400 mb-2">{{ field.help }}</p>
              <div v-for="(item, idx) in asList(config[field.key])" :key="idx" class="list-item mb-2">
                <div class="flex-cb mb-1">
                  <span class="text-xs text-g-500">#{{ idx + 1 }}</span>
                  <ElButton link type="danger" size="small" @click="removeItem(field, idx)">移除</ElButton>
                </div>
                <ElRow :gutter="8">
                  <ElCol v-for="sub in field.fields || []" :key="sub.key" :span="12">
                    <div class="mb-2">
                      <p class="text-xs text-g-500 mb-1">{{ sub.label }}</p>
                      <ElSelect
                        v-if="sub.type === 'select'"
                        v-model="item[sub.key]"
                        size="small"
                        style="width: 100%"
                      >
                        <ElOption v-for="o in sub.options || []" :key="o.value" :label="o.label" :value="o.value" />
                      </ElSelect>
                      <ElInputNumber
                        v-else-if="sub.type === 'int'"
                        v-model="item[sub.key]"
                        size="small"
                        :min="sub.min ?? 0"
                        :max="sub.max ?? 99999999"
                        controls-position="right"
                        style="width: 100%"
                      />
                      <ElInput v-else v-model="item[sub.key]" size="small" :placeholder="sub.placeholder" />
                    </div>
                  </ElCol>
                </ElRow>
              </div>
              <ElButton size="small" @click="addItem(field)">+ 添加一项</ElButton>
            </div>
          </ElFormItem>

          <!-- 布尔 -->
          <ElFormItem v-else-if="field.type === 'bool'" :label="field.label">
            <ElSwitch v-model="config[field.key]" />
            <span v-if="field.help" class="ml-2 text-xs text-g-400">{{ field.help }}</span>
          </ElFormItem>

          <!-- 数字 -->
          <ElFormItem v-else-if="field.type === 'int'" :label="field.label">
            <ElInputNumber
              v-model="config[field.key]"
              :min="field.min ?? 0"
              :max="field.max ?? 99999999"
              controls-position="right"
            />
            <span v-if="field.help" class="ml-2 text-xs text-g-400">{{ field.help }}</span>
          </ElFormItem>

          <!-- 下拉 -->
          <ElFormItem v-else-if="field.type === 'select'" :label="field.label">
            <ElSelect v-model="config[field.key]" style="width: 100%">
              <ElOption v-for="o in field.options || []" :key="o.value" :label="o.label" :value="o.value" />
            </ElSelect>
          </ElFormItem>

          <!-- 多行文本 -->
          <ElFormItem v-else-if="field.type === 'textarea'" :label="field.label">
            <ElInput v-model="config[field.key]" type="textarea" :rows="4" :placeholder="field.placeholder" />
          </ElFormItem>

          <!-- 字符串 / 密码 -->
          <ElFormItem v-else :label="field.label">
            <ElInput
              v-model="config[field.key]"
              :type="field.type === 'password' ? 'password' : 'text'"
              :show-password="field.type === 'password'"
              :placeholder="field.placeholder"
              clearable
            />
            <p v-if="field.help" class="text-xs text-g-400 mt-1">{{ field.help }}</p>
          </ElFormItem>
        </template>
      </ElForm>

      <template #footer>
        <ElCheckbox v-model="restartAfterSave">保存后立即重启</ElCheckbox>
        <div class="mt-2 flex justify-end gap-2">
          <ElButton @click="configVisible = false">取消</ElButton>
          <ElButton type="primary" :loading="saving" @click="onSaveConfig">保存配置</ElButton>
        </div>
      </template>
    </ElDrawer>

    <!-- 日志抽屉 -->
    <ElDrawer v-model="logVisible" :title="`插件日志 · ${current?.name || ''}`" size="720px">
      <div class="flex-cb mb-2">
        <ElSpace>
          <ElTag size="small">{{ logLines }} 行</ElTag>
          <ElCheckbox v-model="autoRefresh">自动刷新（3s）</ElCheckbox>
        </ElSpace>
        <ElButton size="small" @click="loadLogs">刷新</ElButton>
      </div>
      <pre class="log-box">{{ logText || '（暂无日志）' }}</pre>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { fetchMarketPlugins, installMarketPlugin } from '@/api/ops'
  import {
    fetchBundledPlugins,
    fetchPlugin,
    fetchPluginLogs,
    fetchPluginMeta,
    fetchPlugins,
    installBundledPlugin,
    pluginAction,
    uninstallPlugin,
    updatePluginConfig,
    uploadPlugin
  } from '@/api/platform'

  defineOptions({ name: 'PluginMarket' })

  const tab = ref('installed')
  const plugins = ref<any[]>([])
  const bundled = ref<any[]>([])
  const meta = ref<Record<string, any>>({})
  const uploading = ref(false)
  const saving = ref(false)
  const market = ref<any[]>([])
  const marketLoading = ref(false)
  const installingId = ref('')

  const configVisible = ref(false)
  const logVisible = ref(false)
  const current = ref<any>(null)
  const schema = ref<any[]>([])
  const config = reactive<Record<string, any>>({})
  const restartAfterSave = ref(false)

  const logText = ref('')
  const logLines = ref(300)
  const autoRefresh = ref(false)
  let logTimer: any = null

  const statusLabel = (s: string) =>
    ({ running: '运行中', stopped: '已停止', error: '异常', installed: '未启动' })[s] || s

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
  const statusType = (s: string): TagType =>
    ({ running: 'success', stopped: 'info', error: 'danger', installed: 'warning' } as Record<string, TagType>)[s] ||
    'info'

  const asList = (v: any): any[] => (Array.isArray(v) ? v : [])

  const loadAll = async () => {
    const [list, b, m] = await Promise.all([fetchPlugins(), fetchBundledPlugins(), fetchPluginMeta()])
    plugins.value = (list as any[]) || []
    bundled.value = (b as any[]) || []
    meta.value = (m as any) || {}
    loadMarket(false)
  }

  const loadMarket = async (refresh: boolean) => {
    marketLoading.value = true
    try {
      market.value = (await fetchMarketPlugins(refresh)) as any[]
    } catch {
      market.value = [] // 未配置插件源或索引不可达：页面保持可用
    } finally {
      marketLoading.value = false
    }
  }

  const onInstallMarket = async (row: any) => {
    installingId.value = row.id
    try {
      await installMarketPlugin(row.sourceId, row.id)
      await loadAll()
    } finally {
      installingId.value = ''
    }
  }

  const onFileChange = async (file: any) => {
    if (!file?.raw) return
    uploading.value = true
    try {
      await uploadPlugin(file.raw as File)
      await loadAll()
    } finally {
      uploading.value = false
    }
  }

  const onInstallBundled = async (b: any) => {
    await installBundledPlugin(b.key)
    await loadAll()
  }

  const onAction = async (p: any, action: string) => {
    await pluginAction(p.key, action)
    await loadAll()
  }

  const onAutoStart = async (p: any, value: boolean) => {
    await pluginAction(p.key, 'autostart', value)
    await loadAll()
  }

  const onUninstall = async (p: any) => {
    try {
      await ElMessageBox.confirm(
        `确认卸载插件「${p.name}」？插件程序文件会被删除，运行数据目录保留在 ${meta.value.pluginDataDir || 'plugin-data'} 下。`,
        '卸载确认',
        { type: 'warning', confirmButtonText: '确认卸载', cancelButtonText: '取消' }
      )
    } catch {
      return // 用户取消
    }
    await uninstallPlugin(p.key, false)
    ElMessage.success('已卸载')
    await loadAll()
  }

  const openConfig = async (p: any) => {
    const detail = (await fetchPlugin(p.key)) as any
    current.value = detail
    schema.value = detail?.schema || []
    Object.keys(config).forEach((k) => delete config[k])
    Object.assign(config, detail?.configMap || {})
    configVisible.value = true
  }

  const addItem = (field: any) => {
    const list = asList(config[field.key])
    const item: Record<string, any> = {}
    ;(field.fields || []).forEach((sub: any) => {
      item[sub.key] = sub.default ?? (sub.type === 'int' ? 0 : '')
    })
    list.push(item)
    config[field.key] = [...list]
  }

  const removeItem = (field: any, idx: number) => {
    const list = asList(config[field.key]).slice()
    list.splice(idx, 1)
    config[field.key] = list
  }

  const onSaveConfig = async () => {
    if (!current.value) return
    saving.value = true
    try {
      await updatePluginConfig(current.value.key, { ...config }, restartAfterSave.value)
      configVisible.value = false
      await loadAll()
    } finally {
      saving.value = false
    }
  }

  const openLogs = async (p: any) => {
    current.value = p
    logVisible.value = true
    await loadLogs()
  }

  const loadLogs = async () => {
    if (!current.value) return
    const res = (await fetchPluginLogs(current.value.key, logLines.value)) as any
    logText.value = res?.text || ''
  }

  watch(autoRefresh, (v) => {
    if (logTimer) {
      clearInterval(logTimer)
      logTimer = null
    }
    if (v) logTimer = setInterval(loadLogs, 3000)
  })

  watch(logVisible, (v) => {
    if (!v) {
      autoRefresh.value = false
    }
  })

  onMounted(loadAll)
  onUnmounted(() => logTimer && clearInterval(logTimer))
</script>

<style lang="scss" scoped>
  .plugin-card {
    min-height: 230px;
  }

  .plugin-desc {
    display: -webkit-box;
    overflow: hidden;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .list-item {
    padding: 8px;
    border: 1px solid var(--art-border-color);
    border-radius: 6px;
  }

  .log-box {
    height: calc(100vh - 180px);
    padding: 12px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    border-radius: 6px;
    background: #1e1e1e;
    color: #d4d4d4;
  }
</style>

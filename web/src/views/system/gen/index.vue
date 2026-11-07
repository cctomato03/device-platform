<!-- 代码生成器：读 information_schema 生成 CRUD 骨架（entity/service/api/router/页面/菜单 SQL） -->
<template>
  <div class="gen-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElInput
              v-model="keyword"
              placeholder="搜索表名/注释"
              clearable
              class="!w-60"
              @keyup.enter="loadData"
              @clear="loadData"
            />
            <ElButton @click="loadData">刷新</ElButton>
            <ElButton
              v-auth="'tool:gen:download'"
              type="primary"
              :disabled="!selectedTables.length"
              :loading="downloading"
              @click="handleBatchDownload"
            >
              生成选中（{{ selectedTables.length }}）
            </ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable
        :loading="loading"
        :columns="columns"
        :data="filteredTables"
        @selection-change="handleSelectionChange"
      />
    </ElCard>

    <ElDrawer v-model="previewVisible" :title="`生成预览：${previewTable}`" size="72%">
      <ElTabs v-model="activeFile" type="border-card" class="gen-tabs">
        <ElTabPane v-for="f in previewFiles" :key="f.path" :label="fileName(f.path)" :name="f.path">
          <div class="gen-code-header">
            <span class="gen-path">{{ f.path }}</span>
            <ElButton size="small" text @click="copyCode(f.content)">复制</ElButton>
          </div>
          <pre class="gen-code">{{ f.content }}</pre>
        </ElTabPane>
      </ElTabs>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import { downloadGenCode, fetchGenPreview, fetchGenTables } from '@/api/gen'
  import { ElMessage } from 'element-plus'

  defineOptions({ name: 'ToolGen' })

  const loading = ref(false)
  const downloading = ref(false)
  const keyword = ref('')
  const tables = ref<WebBasic.GenTable[]>([])
  const selectedTables = ref<string[]>([])
  const previewVisible = ref(false)
  const previewTable = ref('')
  const previewFiles = ref<WebBasic.GenFile[]>([])
  const activeFile = ref('')

  const filteredTables = computed(() => {
    const kw = keyword.value.trim().toLowerCase()
    if (!kw) return tables.value
    return tables.value.filter(
      (t) =>
        t.name.toLowerCase().includes(kw) || (t.comment || '').toLowerCase().includes(kw)
    )
  })

  const loadData = async () => {
    loading.value = true
    try {
      tables.value = await fetchGenTables()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const { columnChecks, columns } = useTableColumns(() => [
    { type: 'selection', width: 50 },
    { prop: 'name', label: '表名', minWidth: 200 },
    { prop: 'comment', label: '表注释', minWidth: 200 },
    { prop: 'rowCount', label: '数据量', width: 100 },
    { prop: 'entity', label: '实体名', width: 150 },
    { prop: 'routePath', label: '建议接口前缀', minWidth: 180 },
    { prop: 'createAt', label: '创建时间', width: 180 },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.GenTable) =>
        h(ArtButtonMore, {
          list: [
            { key: 'preview', label: '预览代码', icon: 'ri:eye-line', auth: 'tool:gen:preview' },
            { key: 'download', label: '生成下载', icon: 'ri:download-2-line', auth: 'tool:gen:download' }
          ],
          onClick: (item: ButtonMoreItem) =>
            item.key === 'preview' ? handlePreview(row) : handleDownload([row.name])
        })
    }
  ])

  const handleSelectionChange = (rows: WebBasic.GenTable[]) => {
    selectedTables.value = rows.map((r) => r.name)
  }

  const handlePreview = async (row: WebBasic.GenTable) => {
    previewTable.value = row.name
    previewFiles.value = await fetchGenPreview(row.name)
    activeFile.value = previewFiles.value[0]?.path || ''
    previewVisible.value = true
  }

  const handleDownload = async (list: string[]) => {
    downloading.value = true
    try {
      await downloadGenCode(list)
      ElMessage.success('代码已打包下载')
    } catch (err: any) {
      ElMessage.error(err?.message || '下载失败')
    } finally {
      downloading.value = false
    }
  }

  const handleBatchDownload = () => handleDownload(selectedTables.value)

  const fileName = (path: string) => path.split('/').pop() || path

  const copyCode = async (content: string) => {
    try {
      await navigator.clipboard.writeText(content)
      ElMessage.success('已复制')
    } catch {
      ElMessage.error('复制失败，请手动选择')
    }
  }
</script>

<style scoped>
  .gen-tabs {
    height: 100%;
  }

  .gen-code-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .gen-code {
    max-height: calc(100vh - 260px);
    margin: 0;
    overflow: auto;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre;
  }
</style>

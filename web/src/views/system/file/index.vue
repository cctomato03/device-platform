<!-- 文件管理（本地存储 / 阿里云 OSS，web-basic 真实数据） -->
<template>
  <div class="file-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElUpload
              v-auth="'system:file:upload'"
              :show-file-list="false"
              :http-request="handleUpload"
              :disabled="uploading"
            >
              <ElButton type="primary" :loading="uploading" v-ripple>上传文件</ElButton>
            </ElUpload>
            <ElSelect v-model="query.bizType" placeholder="业务类型" clearable class="!w-40" @change="applyQuery">
              <ElOption label="默认" value="default" />
              <ElOption label="头像" value="avatar" />
              <ElOption label="文档" value="document" />
              <ElOption label="图片" value="image" />
              <ElOption label="站点设置" value="site" />
            </ElSelect>
            <ElRadioGroup v-model="quickRange" size="default" @change="applyQuickRange">
              <ElRadioButton value="">全部</ElRadioButton>
              <ElRadioButton value="today">今天</ElRadioButton>
              <ElRadioButton value="week">近 7 天</ElRadioButton>
              <ElRadioButton value="month">本月</ElRadioButton>
            </ElRadioGroup>
            <ElDatePicker
              v-model="query.dateRange"
              type="daterange"
              value-format="YYYY-MM-DD"
              range-separator="~"
              start-placeholder="上传起始日"
              end-placeholder="上传截止日"
              class="!w-70"
              @change="applyQuery"
            />
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable
        :loading="loading"
        :columns="columns"
        :data="data"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { fetchGetFileList, fetchDeleteFile } from '@/api/business'
  import { uploadFile } from '@/utils/upload'
  import { ElImage, ElButton, ElMessageBox, ElMessage, type UploadRequestOptions } from 'element-plus'
  import { formatSize, formatTime } from '@/utils/web-basic'
  import defaultFileIcon from '@/assets/images/common/logo.webp'

  defineOptions({ name: 'File' })

  const uploading = ref(false)
  const query = reactive<{ bizType?: string; dateRange?: [string, string] | null }>({
    bizType: undefined,
    dateRange: null
  })
  // 快捷时间范围：today / week / month / ""（全部）
  const quickRange = ref('')

  const {
    columns,
    columnChecks,
    data,
    loading,
    pagination,
    getData,
    replaceSearchParams,
    handleSizeChange,
    handleCurrentChange,
    refreshData
  } = useTable({
    core: {
      apiFn: fetchGetFileList,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', width: 60, label: '序号' },
        {
          prop: 'originName',
          label: '文件名',
          minWidth: 240,
          formatter: (row) =>
            h('div', { class: 'flex-c gap-2' }, [
              isImage(row.ext)
                ? h(ElImage, {
                    class: 'size-8 rounded object-cover',
                    src: row.url,
                    previewSrcList: [row.url],
                    previewTeleported: true
                  })
                : h('img', { class: 'size-8 rounded', src: defaultFileIcon }),
              h('span', { class: 'truncate' }, row.originName)
            ])
        },
        {
          prop: 'size',
          label: '大小',
          width: 110,
          formatter: (row) => formatSize(row.size || 0)
        },
        { prop: 'ext', label: '类型', width: 90 },
        { prop: 'bizType', label: '业务', width: 110, formatter: (row) => row.bizType || 'default' },
        {
          prop: 'dir',
          label: '存储目录',
          minWidth: 190,
          showOverflowTooltip: true,
          formatter: (row) => row.dir || '-'
        },
        {
          prop: 'storage',
          label: '存储',
          width: 110,
          formatter: (row) => (row.storage === 'oss' ? '阿里云 OSS' : '本地磁盘')
        },
        { prop: 'createTime', label: '上传时间', width: 180, formatter: (row) => formatTime(row.createTime) },
        {
          prop: 'operation',
          label: '操作',
          width: 80,
          fixed: 'right',
          formatter: (row) =>
            h(ArtButtonMore, {
              list: [
                { key: 'view', label: '预览 / 打开', icon: 'ri:eye-line' },
                { key: 'copy', label: '复制链接', icon: 'ri:file-copy-line' },
                {
                  key: 'delete',
                  label: '删除文件',
                  icon: 'ri:delete-bin-4-line',
                  color: '#f56c6c',
                  auth: 'system:file:delete'
                }
              ],
              onClick: (item: ButtonMoreItem) => {
                if (item.key === 'view') openFile(row.url)
                else if (item.key === 'copy') copyUrl(row.url)
                else handleDelete(row)
              }
            })
        }
      ]
    }
  })

  /** 条件变化后重新查询（业务类型 + 日期区间一起下发，分页回到第一页） */
  const applyQuery = () => {
    quickRange.value = ''
    replaceSearchParams({
      bizType: query.bizType || undefined,
      startDate: query.dateRange?.[0] || undefined,
      endDate: query.dateRange?.[1] || undefined
    })
    getData()
  }

  /** 快捷时间范围 → 日期区间 */
  const applyQuickRange = (val: string | number | boolean | undefined) => {
    const fmt = (d: Date) =>
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    const today = new Date()
    if (val === 'today') {
      query.dateRange = [fmt(today), fmt(today)]
    } else if (val === 'week') {
      const from = new Date(today)
      from.setDate(from.getDate() - 6)
      query.dateRange = [fmt(from), fmt(today)]
    } else if (val === 'month') {
      query.dateRange = [fmt(new Date(today.getFullYear(), today.getMonth(), 1)), fmt(today)]
    } else {
      query.dateRange = null
    }
    applyQuery()
  }

  const isImage = (ext = '') => ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg'].includes(ext.toLowerCase())

  const openFile = (url: string) => window.open(url, '_blank')

  const copyUrl = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url)
      ElMessage.success('链接已复制')
    } catch {
      ElMessage.warning(`复制失败，请手动复制：${url}`)
    }
  }

  /** 自定义上传 */
  const handleUpload = async (options: UploadRequestOptions) => {
    uploading.value = true
    try {
      // 直传优先：OSS 可用时浏览器直传，否则自动回退服务端上传
      await uploadFile(options.file as File, { bizType: query.bizType || 'default' })
      refreshData()
    } finally {
      uploading.value = false
    }
  }

  const handleDelete = async (row: WebBasic.FileItem) => {
    try {
      await ElMessageBox.confirm(`确定删除文件「${row.originName}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteFile(row.id)
    refreshData()
  }
</script>
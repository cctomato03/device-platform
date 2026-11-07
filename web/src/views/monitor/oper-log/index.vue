<!-- 操作日志 -->
<template>
  <div class="oper-log-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElInput
              v-model="username"
              placeholder="搜索操作人"
              clearable
              class="!w-52"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            />
            <ElButton @click="handleSearch">查询</ElButton>
            <ElButton v-auth="'monitor:oper-log:export'" :loading="exporting" @click="handleExport">
              导出
            </ElButton>
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
        @row-click="handleRowClick"
      />
    </ElCard>

    <ElDrawer v-model="drawerVisible" title="日志详情" size="520px">
      <ElDescriptions v-if="current" :column="1" border>
        <ElDescriptionsItem label="操作模块">{{ current.title || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="操作人">{{ current.userName || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="部门">{{ current.deptName || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="请求">
          {{ current.httpMethod }} {{ current.requestUri }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="处理方法">{{ current.method || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="IP / 归属">{{ current.ip }} {{ current.location }}</ElDescriptionsItem>
        <ElDescriptionsItem label="耗时">{{ current.costMs }} ms</ElDescriptionsItem>
        <ElDescriptionsItem label="状态">
          <ElTag :type="current.status === 1 ? 'success' : 'danger'">
            {{ current.status === 1 ? '成功' : '失败' }}
          </ElTag>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="错误信息">{{ current.errorMsg || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="请求参数">
          <pre class="log-pre">{{ current.params || '-' }}</pre>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="字段变更">
          <ElTable v-if="diffList.length" :data="diffList" size="small" border max-height="240">
            <ElTableColumn label="字段" width="140">
              <template #default="{ row }">{{ fieldLabel(row.field) }}</template>
            </ElTableColumn>
            <ElTableColumn label="修改前" min-width="120">
              <template #default="{ row }">
                <span class="diff-before">{{ row.before || '空' }}</span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="修改后" min-width="120">
              <template #default="{ row }">
                <span class="diff-after">{{ row.after || '空' }}</span>
              </template>
            </ElTableColumn>
          </ElTable>
          <span v-else>-</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="返回结果">
          <pre class="log-pre">{{ current.result || '-' }}</pre>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="时间">{{ formatTime(current.operTime) }}</ElDescriptionsItem>
      </ElDescriptions>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import { useTable } from '@/hooks/core/useTable'
  import { fetchGetOperLogs } from '@/api/business'
  import { ElTag } from 'element-plus'
  import { formatTime, downloadFile } from '@/utils/web-basic'

  defineOptions({ name: 'MonitorOperLog' })

  const username = ref('')
  const drawerVisible = ref(false)
  const current = ref<WebBasic.OperLog | null>(null)

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
      apiFn: fetchGetOperLogs,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', width: 60, label: '序号' },
        { prop: 'title', label: '操作模块', minWidth: 150 },
        { prop: 'userName', label: '操作人', width: 130 },
        {
          prop: 'requestUri',
          label: '请求地址',
          minWidth: 240,
          showOverflowTooltip: true,
          formatter: (row) => `${row.httpMethod} ${row.requestUri}`
        },
        { prop: 'ip', label: 'IP', width: 130 },
        { prop: 'costMs', label: '耗时(ms)', width: 100 },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row) =>
            h(
              ElTag,
              { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
              () => (row.status === 1 ? '成功' : '失败')
            )
        },
        { prop: 'operTime', label: '操作时间', width: 180, formatter: (row) => formatTime(row.operTime) }
      ]
    }
  })

  const handleSearch = () => {
    replaceSearchParams({ username: username.value || undefined })
    getData()
  }

  const exporting = ref(false)
  const handleExport = async () => {
    exporting.value = true
    try {
      await downloadFile(
        '/api/v1/monitor/oper-logs/export',
        { username: username.value || undefined },
        '操作日志.xlsx'
      )
    } catch (err: any) {
      ElMessage.error(err?.message || '导出失败')
    } finally {
      exporting.value = false
    }
  }

  /** 解析后端写入的字段级变更差异（sys_oper_log.diff） */
  const diffList = computed<{ field: string; before: string; after: string }[]>(() => {
    const raw = current.value?.diff
    if (!raw) return []
    try {
      const arr = JSON.parse(raw)
      return Array.isArray(arr) ? arr : []
    } catch {
      return []
    }
  })

  /** 常见列名 → 中文（未收录则原样展示） */
  const fieldLabels: Record<string, string> = {
    name: '名称',
    code: '编码',
    status: '状态',
    sort: '排序',
    remark: '备注',
    nickname: '昵称',
    mobile: '手机号',
    email: '邮箱',
    sex: '性别',
    avatar: '头像',
    dept_id: '部门',
    parent_id: '上级',
    ancestors: '祖级路径',
    leader: '负责人',
    phone: '电话',
    permission: '权限码',
    path: '路由地址',
    component: '组件路径',
    icon: '图标',
    visible: '显示',
    keep_alive: '缓存',
    data_scope: '数据范围',
    config_key: '参数键',
    value: '参数值',
    value_type: '值类型',
    is_public: '是否公开',
    label: '字典标签',
    tag_type: '标签样式',
    css_class: '样式类',
    is_default: '默认项',
    cron: 'cron 表达式',
    handler: '处理器',
    target: '目标地址',
    params: '任务参数',
    timeout_sec: '超时(秒)',
    retry_times: '重试次数',
    concurrent: '并发策略',
    contact_name: '联系人',
    contact_phone: '联系电话',
    max_users: '最大用户数',
    expire_at: '到期时间',
    password: '密码'
  }
  const fieldLabel = (field: string) => fieldLabels[field] || field

  const handleRowClick = (row: WebBasic.OperLog) => {
    current.value = row
    drawerVisible.value = true
  }
</script>

<style scoped>
  .diff-before {
    color: var(--el-color-danger);
    text-decoration: line-through;
  }

  .diff-after {
    color: var(--el-color-success);
  }

  .log-pre {
    max-height: 160px;
    margin: 0;
    overflow: auto;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>

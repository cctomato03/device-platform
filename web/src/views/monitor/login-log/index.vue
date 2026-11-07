<!-- 登录日志 -->
<template>
  <div class="login-log-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElInput
              v-model="username"
              placeholder="搜索登录账号"
              clearable
              class="!w-52"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            />
            <ElButton @click="handleSearch">查询</ElButton>
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
  import { useTable } from '@/hooks/core/useTable'
  import { fetchGetLoginLogs } from '@/api/business'
  import { formatTime } from '@/utils/web-basic'
  import { ElTag } from 'element-plus'

  defineOptions({ name: 'MonitorLoginLog' })

  const username = ref('')

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
      apiFn: fetchGetLoginLogs,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', width: 60, label: '序号' },
        { prop: 'username', label: '账号', minWidth: 140 },
        { prop: 'ip', label: 'IP', width: 140 },
        {
          prop: 'status',
          label: '结果',
          width: 100,
          formatter: (row) =>
            h(
              ElTag,
              { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
              () => (row.status === 1 ? '成功' : '失败')
            )
        },
        { prop: 'msg', label: '说明', minWidth: 160 },
        {
          prop: 'userAgent',
          label: '客户端',
          minWidth: 240,
          showOverflowTooltip: true
        },
        {
          prop: 'loginTime',
          label: '登录时间',
          width: 180,
          formatter: (row) => formatTime(row.loginTime)
        }
      ]
    }
  })

  const handleSearch = () => {
    replaceSearchParams({ username: username.value || undefined })
    getData()
  }
</script>

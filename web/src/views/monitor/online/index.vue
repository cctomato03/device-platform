<!-- 在线用户（Redis 会话，可强制下线） -->
<template>
  <div class="online-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElTag type="info">当前在线会话：{{ tableData.length }}</ElTag>
        </template>
      </ArtTableHeader>

      <ArtTable
        :loading="loading"
        :columns="columns"
        :data="tableData"
      />
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import { fetchGetOnlineUsers, fetchKickUser } from '@/api/business'
  import { formatUnix } from '@/utils/web-basic'
  import { ElMessageBox, ElMessage } from 'element-plus'

  defineOptions({ name: 'MonitorOnline' })

  const loading = ref(false)
  const tableData = ref<WebBasic.OnlineUser[]>([])

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetOnlineUsers()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'username', label: '账号', minWidth: 140 },
    { prop: 'nickname', label: '昵称', minWidth: 140 },
    {
      prop: 'loginTime',
      label: '登录时间',
      width: 190,
      formatter: (row: WebBasic.OnlineUser) => formatUnix(row.loginTime)
    },
    {
      prop: 'jti',
      label: '会话标识',
      minWidth: 260,
      showOverflowTooltip: true
    },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.OnlineUser) =>
        h(ArtButtonMore, {
          list: [
            {
              key: 'kick',
              label: '强制下线',
              icon: 'ri:logout-circle-r-line',
              color: '#f56c6c'
            }
          ],
          onClick: () => handleKick(row)
        })
    }
  ])

  const handleKick = async (row: WebBasic.OnlineUser) => {
    try {
      await ElMessageBox.confirm(`确定强制下线用户「${row.username}」吗？`, '强制下线', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchKickUser(row.userId, row.jti)
    loadData()
  }
</script>
<!-- 流量日志：平台按周期采集的出入流量记录 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>流量日志</span>
          <ElSpace>
            <ElInput v-model="query.iface" placeholder="网卡（可选）" clearable size="small" style="width: 140px" />
            <ElButton size="small" type="primary" @click="search">查询</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="loggedAt" label="采样时间" width="180" />
        <ElTableColumn prop="iface" label="网卡" width="110" />
        <ElTableColumn label="入向累计" width="140">
          <template #default="{ row }">{{ formatBytes(row.bytesIn) }}</template>
        </ElTableColumn>
        <ElTableColumn label="出向累计" width="140">
          <template #default="{ row }">{{ formatBytes(row.bytesOut) }}</template>
        </ElTableColumn>
        <ElTableColumn label="入向速率" width="130">
          <template #default="{ row }">{{ formatBps(row.inBps) }}</template>
        </ElTableColumn>
        <ElTableColumn label="出向速率" width="130">
          <template #default="{ row }">{{ formatBps(row.outBps) }}</template>
        </ElTableColumn>
        <ElTableColumn prop="packetsIn" label="入包数" width="120" />
        <ElTableColumn prop="packetsOut" label="出包数" width="120" />
      </ElTable>

      <div class="mt-3 flex justify-end">
        <ElPagination
          v-model:current-page="query.current"
          v-model:page-size="query.size"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchTrafficLogs } from '@/api/platform'

  defineOptions({ name: 'LogTraffic' })

  const loading = ref(false)
  const rows = ref<any[]>([])
  const total = ref(0)
  const query = reactive<Record<string, any>>({ iface: '', current: 1, size: 20 })

  const loadData = async () => {
    loading.value = true
    try {
      const res = (await fetchTrafficLogs({ ...query })) as any
      rows.value = res?.list || []
      total.value = res?.total || 0
    } finally {
      loading.value = false
    }
  }

  const search = () => {
    query.current = 1
    loadData()
  }

  const formatBytes = (n?: number) => {
    if (!n || n <= 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let i = 0
    let v = n
    while (v >= 1024 && i < units.length - 1) {
      v /= 1024
      i++
    }
    return `${v.toFixed(1)} ${units[i]}`
  }

  const formatBps = (bps?: number) => {
    if (!bps || bps <= 0) return '0 bps'
    const units = ['bps', 'Kbps', 'Mbps', 'Gbps']
    let i = 0
    let v = bps
    while (v >= 1000 && i < units.length - 1) {
      v /= 1000
      i++
    }
    return `${v.toFixed(1)} ${units[i]}`
  }

  onMounted(loadData)
</script>

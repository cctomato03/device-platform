<!-- 网络流量：出入流量趋势 + 流量采样日志 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12" class="mb-3">
      <ElCol v-for="card in cards" :key="card.label" :xs="24" :sm="12" :md="6">
        <ElCard>
          <p class="text-sm text-g-500">{{ card.label }}</p>
          <p class="mt-1 text-xl font-semibold">{{ card.value }}</p>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>流量采样记录</span>
          <ElSpace>
            <ElInput v-model="query.iface" placeholder="网卡（可选）" clearable size="small" style="width: 140px" />
            <ElButton size="small" type="primary" @click="loadData">查询</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" size="small" height="440">
        <ElTableColumn prop="loggedAt" label="采样时间" width="180" />
        <ElTableColumn prop="iface" label="网卡" width="110" />
        <ElTableColumn label="入向流量" width="140">
          <template #default="{ row }">{{ formatBytes(row.bytesIn) }}</template>
        </ElTableColumn>
        <ElTableColumn label="出向流量" width="140">
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
  import { fetchHostOverview, fetchTrafficLogs } from '@/api/platform'

  defineOptions({ name: 'HostTraffic' })

  const loading = ref(false)
  const rows = ref<any[]>([])
  const total = ref(0)
  const overview = ref<any>(null)
  const query = reactive({ iface: '', current: 1, size: 20 })

  const cards = computed(() => {
    const net = overview.value?.net || {}
    return [
      { label: '累计接收', value: formatBytes(net.bytesRecv) },
      { label: '累计发送', value: formatBytes(net.bytesSent) },
      { label: '实时入向', value: formatBps(net.recvRateBps) },
      { label: '实时出向', value: formatBps(net.sendRateBps) }
    ]
  })

  const loadData = async () => {
    loading.value = true
    try {
      const res = (await fetchTrafficLogs({ ...query })) as any
      rows.value = res?.list || []
      total.value = res?.total || 0
      overview.value = await fetchHostOverview()
    } finally {
      loading.value = false
    }
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

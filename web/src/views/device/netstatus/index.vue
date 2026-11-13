<!-- 网络状态：网卡快速状态 + 路由表 + DNS -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol v-for="card in cards" :key="card.label" :xs="12" :sm="6">
        <ElCard class="mb-3">
          <p class="text-sm text-g-500">{{ card.label }}</p>
          <p class="mt-1 text-lg font-semibold break-all">{{ card.value }}</p>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="14">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>网卡</span>
              <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
            </div>
          </template>
          <ElTable :data="status?.interfaces || []" size="small">
            <ElTableColumn prop="name" label="网卡" width="120" />
            <ElTableColumn label="地址" min-width="180">
              <template #default="{ row }">
                <span class="break-all">{{ (row.addrs || []).join(', ') || '-' }}</span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="状态" width="90">
              <template #default="{ row }">
                <ElTag :type="row.up ? 'success' : 'info'" size="small">{{ row.up ? 'UP' : 'DOWN' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="接收/发送" width="180">
              <template #default="{ row }">
                {{ formatBytes(row.bytesRecv) }} / {{ formatBytes(row.bytesSent) }}
              </template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
      <ElCol :xs="24" :md="10">
        <ElCard class="mb-3" header="路由与 DNS">
          <p class="text-sm font-medium mb-1">路由表</p>
          <pre class="mini-box">{{ (status?.routes || []).join('\n') || '无' }}</pre>
          <p class="text-sm font-medium mt-3 mb-1">DNS</p>
          <ElSpace wrap>
            <ElTag v-for="d in status?.dns || []" :key="d" size="small">{{ d }}</ElTag>
            <span v-if="!(status?.dns || []).length" class="text-g-400">未配置</span>
          </ElSpace>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchNetworkStatus } from '@/api/platform'

  defineOptions({ name: 'DeviceNetStatus' })

  const loading = ref(false)
  const status = ref<any>(null)

  const cards = computed(() => [
    { label: '主机名', value: status.value?.hostname || '-' },
    { label: '默认网关', value: status.value?.gateway || '-' },
    { label: '网卡数量', value: (status.value?.interfaces || []).length },
    { label: '在线网卡', value: (status.value?.interfaces || []).filter((i: any) => i.up).length }
  ])

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

  const loadData = async () => {
    loading.value = true
    try {
      status.value = await fetchNetworkStatus()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .mini-box {
    max-height: 180px;
    padding: 8px;
    overflow: auto;
    font-size: 12px;
    line-height: 1.6;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

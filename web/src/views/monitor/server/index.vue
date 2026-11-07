<!-- 服务器监控（Go 运行时 + MySQL / Redis 健康状态） -->
<template>
  <div class="server-page art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :sm="12" :md="6" v-for="card in cards" :key="card.label">
        <ElCard class="mb-3">
          <div class="flex-cb">
            <div>
              <p class="text-sm text-g-500">{{ card.label }}</p>
              <p class="mt-1 text-2xl font-semibold">{{ card.value }}</p>
            </div>
            <ArtSvgIcon :icon="card.icon" class="text-3xl text-theme opacity-80" />
          </div>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>运行环境</span>
          <ElSpace>
            <ElTag :type="info?.mysqlOk ? 'success' : 'danger'">
              MySQL {{ info?.mysqlOk ? '正常' : '异常' }}
            </ElTag>
            <ElTag :type="info?.redisOk ? 'success' : 'danger'">
              Redis {{ info?.redisOk ? '正常' : 'danger' }}
            </ElTag>
            <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElDescriptions :column="3" border>
        <ElDescriptionsItem label="Go 版本">{{ info?.goVersion || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="操作系统">{{ info?.os || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="架构">{{ info?.arch || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="CPU 核数">{{ info?.numCpu ?? '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="Goroutine 数">{{ info?.goroutines ?? '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="已运行">
          {{ uptimeText }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="堆内存">{{ info?.heapAllocMb?.toFixed(2) ?? '-' }} MB</ElDescriptionsItem>
        <ElDescriptionsItem label="系统内存">{{ info?.sysMb?.toFixed(2) ?? '-' }} MB</ElDescriptionsItem>
        <ElDescriptionsItem label="服务地址">{{ origin }}</ElDescriptionsItem>
      </ElDescriptions>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchGetServerInfo } from '@/api/business'

  defineOptions({ name: 'MonitorServer' })

  const loading = ref(false)
  const info = ref<WebBasic.ServerInfo | null>(null)

  const origin = window.location.origin

  const loadData = async () => {
    loading.value = true
    try {
      info.value = await fetchGetServerInfo()
    } finally {
      loading.value = false
    }
  }

  const uptimeText = computed(() => {
    const s = info.value?.uptimeSec ?? 0
    const d = Math.floor(s / 86400)
    const h = Math.floor((s % 86400) / 3600)
    const m = Math.floor((s % 3600) / 60)
    return `${d} 天 ${h} 小时 ${m} 分`
  })

  const cards = computed(() => [
    { label: 'CPU 核数', value: info.value?.numCpu ?? '-', icon: 'ri:cpu-line' },
    { label: 'Goroutines', value: info.value?.goroutines ?? '-', icon: 'ri:flow-chart' },
    {
      label: '堆内存',
      value: info.value ? `${info.value.heapAllocMb.toFixed(1)} MB` : '-',
      icon: 'ri:database-2-line'
    },
    { label: '运行时长', value: `${Math.floor((info.value?.uptimeSec ?? 0) / 60)} 分钟`, icon: 'ri:timer-line' }
  ])

  onMounted(loadData)
</script>

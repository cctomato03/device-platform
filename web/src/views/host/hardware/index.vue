<!-- 硬件状态：CPU/内存/磁盘/温度/负载明细 -->
<template>
  <div class="art-full-height">
    <ElCard class="mb-3">
      <template #header>
        <div class="flex-cb">
          <span>硬件概览</span>
          <ElSpace>
            <ElTag v-for="t in tempTags" :key="t" type="warning" size="small">{{ t }}</ElTag>
            <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>
      <ElDescriptions :column="3" border size="small">
        <ElDescriptionsItem label="主机名">{{ snap?.host?.hostname || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="CPU 型号">{{ snap?.cpu?.model || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="核心 / 逻辑">{{ snap?.cpu?.cores }} / {{ snap?.cpu?.logical }}</ElDescriptionsItem>
        <ElDescriptionsItem label="主频">{{ snap?.cpu?.mhz ? snap.cpu.mhz.toFixed(0) + ' MHz' : '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="平均负载">
          {{ snap?.load?.load1 }} / {{ snap?.load?.load5 }} / {{ snap?.load?.load15 }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="进程数">{{ snap?.load?.processCount ?? '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="内核版本">{{ snap?.host?.kernelVersion || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="启动时间">{{ bootTime }}</ElDescriptionsItem>
        <ElDescriptionsItem label="虚拟化">{{ snap?.host?.virtualization || '物理机' }}</ElDescriptionsItem>
      </ElDescriptions>
    </ElCard>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="每核 CPU 使用率">
          <div v-for="(v, i) in snap?.cpu?.perCore || []" :key="i" class="mb-2">
            <div class="flex-cb mb-1 text-xs">
              <span>核心 {{ i }}</span>
              <span>{{ v.toFixed(1) }}%</span>
            </div>
            <ElProgress :percentage="Math.min(Math.round(v), 100)" :show-text="false" :stroke-width="8" />
          </div>
        </ElCard>
      </ElCol>
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="内存与磁盘">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="内存总量">{{ formatBytes(snap?.memory?.total) }}</ElDescriptionsItem>
            <ElDescriptionsItem label="已用 / 可用">
              {{ formatBytes(snap?.memory?.used) }} / {{ formatBytes(snap?.memory?.available) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="Swap">
              {{ formatBytes(snap?.memory?.swapUsed) }} / {{ formatBytes(snap?.memory?.swapTotal) }}
            </ElDescriptionsItem>
          </ElDescriptions>
          <ElTable :data="snap?.disks || []" size="small" class="mt-3">
            <ElTableColumn prop="mountpoint" label="挂载点" show-overflow-tooltip />
            <ElTableColumn prop="fstype" label="类型" width="100" />
            <ElTableColumn label="已用/总量" width="180">
              <template #default="{ row }">{{ formatBytes(row.used) }} / {{ formatBytes(row.total) }}</template>
            </ElTableColumn>
            <ElTableColumn label="使用率" width="100">
              <template #default="{ row }">{{ row.usedPercent }}%</template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchHostSnapshot } from '@/api/platform'

  defineOptions({ name: 'HostHardware' })

  const loading = ref(false)
  const snap = ref<any>(null)

  const tempTags = computed(() => {
    const list = (snap.value?.temperature || []) as number[]
    return list.slice(0, 4).map((t) => `${t.toFixed(1)} ℃`)
  })

  const bootTime = computed(() => {
    const ts = Number(snap.value?.host?.bootTime || 0)
    if (!ts) return '-'
    return new Date(ts * 1000).toLocaleString()
  })

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
      snap.value = await fetchHostSnapshot()
    } finally {
      loading.value = false
    }
  }

  let timer: any = null
  onMounted(() => {
    loadData()
    timer = setInterval(loadData, 10000)
  })
  onUnmounted(() => timer && clearInterval(timer))
</script>

<!-- 系统监控：指标趋势（CPU / 内存 / 磁盘 / 网络）与实时数值 -->
<template>
  <div class="art-full-height">
    <ElCard class="mb-3">
      <template #header>
        <div class="flex-cb">
          <span>指标趋势</span>
          <ElSpace>
            <ElRadioGroup v-model="minutes" size="small" @change="loadHistory">
              <ElRadioButton :value="15">15 分钟</ElRadioButton>
              <ElRadioButton :value="60">1 小时</ElRadioButton>
              <ElRadioButton :value="360">6 小时</ElRadioButton>
              <ElRadioButton :value="1440">24 小时</ElRadioButton>
            </ElRadioGroup>
            <ElButton size="small" :loading="loading" @click="refresh">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElRow :gutter="12">
        <ElCol v-for="chart in charts" :key="chart.key" :xs="24" :md="12">
          <div class="mb-4">
            <div class="flex-cb mb-1">
              <span class="text-sm font-medium">{{ chart.label }}</span>
              <span class="text-sm text-g-500">当前 {{ chart.current }}</span>
            </div>
            <div class="chart-bars">
              <div
                v-for="(point, i) in chart.points"
                :key="i"
                class="chart-bar"
                :style="{ height: point.h + '%' }"
                :title="point.tip"
              />
              <div v-if="!chart.points.length" class="text-xs text-g-400">暂无数据</div>
            </div>
          </div>
        </ElCol>
      </ElRow>
      <p class="text-xs text-g-400">
        说明：柱状图为最近 {{ minutes }} 分钟采样（平台默认每
        {{ intervalSec }} 秒采集一次），用于快速判断趋势；数值精度以「系统概览」为准。
      </p>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchHostMetrics, fetchHostOverview } from '@/api/platform'

  defineOptions({ name: 'HostMonitor' })

  const minutes = ref(60)
  const loading = ref(false)
  const samples = ref<any[]>([])
  const overview = ref<any>(null)
  const intervalSec = 10

  /** 把某一指标转换为柱状图数据（百分比高度） */
  const toPoints = (field: string, max?: number) => {
    const list = samples.value || []
    const values = list.map((s) => Number(s[field] || 0))
    const peak = max || Math.max(1, ...values)
    return list.map((s, i) => ({
      h: Math.max(2, Math.round((values[i] / peak) * 100)),
      tip: `${s.sampledAt} ${values[i].toFixed(2)}`
    }))
  }

  const charts = computed(() => {
    const s = overview.value?.summary || {}
    return [
      {
        key: 'cpu',
        label: 'CPU 使用率 (%)',
        current: `${s.cpuPercent ?? 0}%`,
        points: toPoints('cpuPercent', 100)
      },
      {
        key: 'mem',
        label: '内存使用率 (%)',
        current: `${s.memPercent ?? 0}%`,
        points: toPoints('memPercent', 100)
      },
      {
        key: 'disk',
        label: '磁盘使用率 (%)',
        current: `${s.diskPercent ?? 0}%`,
        points: toPoints('diskPercent', 100)
      },
      {
        key: 'net',
        label: '网络流量 (bps)',
        current: `${Math.round(s.netInBps || 0)} / ${Math.round(s.netOutBps || 0)}`,
        points: toPoints('netInBps')
      }
    ]
  })

  const loadHistory = async () => {
    loading.value = true
    try {
      samples.value = (await fetchHostMetrics(minutes.value)) as any[]
    } finally {
      loading.value = false
    }
  }

  const refresh = async () => {
    await Promise.all([loadHistory(), (async () => (overview.value = await fetchHostOverview()))()])
  }

  let timer: any = null
  onMounted(() => {
    refresh()
    timer = setInterval(refresh, 20000)
  })
  onUnmounted(() => timer && clearInterval(timer))
</script>

<style lang="scss" scoped>
  .chart-bars {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    height: 120px;
    padding: 6px;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }

  .chart-bar {
    flex: 1;
    min-width: 2px;
    border-radius: 2px 2px 0 0;
    background: var(--el-color-primary);
    opacity: 0.85;

    &:hover {
      opacity: 1;
    }
  }
</style>

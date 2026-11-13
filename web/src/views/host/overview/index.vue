<!-- 系统概览：设备整体运行状态（CPU/内存/磁盘/网络/告警/插件） -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol v-for="card in cards" :key="card.label" :xs="24" :sm="12" :md="6">
        <ElCard class="mb-3">
          <div class="flex-cb">
            <div>
              <p class="text-sm text-g-500">{{ card.label }}</p>
              <p class="mt-1 text-2xl font-semibold">{{ card.value }}</p>
              <p class="mt-1 text-xs text-g-400">{{ card.hint }}</p>
            </div>
            <ArtSvgIcon :icon="card.icon" class="text-3xl text-theme opacity-80" />
          </div>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>设备信息</span>
              <ElSpace>
                <ElTag type="success">运行中</ElTag>
                <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
              </ElSpace>
            </div>
          </template>
          <ElDescriptions :column="2" border size="small">
            <ElDescriptionsItem label="设备名称">{{ overview?.host?.hostname || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="操作系统">{{ platformText }}</ElDescriptionsItem>
            <ElDescriptionsItem label="内核架构">{{ overview?.host?.kernelArch || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="已运行">{{ uptimeText }}</ElDescriptionsItem>
            <ElDescriptionsItem label="CPU">{{ overview?.cpu?.model || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="核心数">
              {{ overview?.cpu?.logical || 0 }} 逻辑 / {{ overview?.cpu?.cores || 0 }} 物理
            </ElDescriptionsItem>
            <ElDescriptionsItem label="平台版本">{{ overview?.platform?.version || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="数据目录">
              <span class="break-all">{{ overview?.platform?.home || '-' }}</span>
            </ElDescriptionsItem>
          </ElDescriptions>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3">
          <template #header><span>资源占用</span></template>
          <div v-for="item in usage" :key="item.label" class="mb-4">
            <div class="flex-cb mb-1">
              <span class="text-sm">{{ item.label }}</span>
              <span class="text-sm text-g-500">{{ item.value }}</span>
            </div>
            <ElProgress :percentage="Math.min(item.percent, 100)" :status="item.status" :stroke-width="10" />
          </div>
          <ElDescriptions :column="2" border size="small">
            <ElDescriptionsItem label="内存">
              {{ formatBytes(overview?.memory?.used) }} / {{ formatBytes(overview?.memory?.total) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="交换分区">
              {{ formatBytes(overview?.memory?.swapUsed) }} / {{ formatBytes(overview?.memory?.swapTotal) }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="入向速率">{{ formatBps(overview?.net?.recvRateBps) }}</ElDescriptionsItem>
            <ElDescriptionsItem label="出向速率">{{ formatBps(overview?.net?.sendRateBps) }}</ElDescriptionsItem>
          </ElDescriptions>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="磁盘分区">
          <ElTable :data="overview?.disks || []" size="small">
            <ElTableColumn prop="mountpoint" label="挂载点" min-width="120" show-overflow-tooltip />
            <ElTableColumn prop="fstype" label="文件系统" width="110" />
            <ElTableColumn label="使用率" width="180">
              <template #default="{ row }">
                <ElProgress :percentage="Math.min(Math.round(row.usedPercent), 100)" :stroke-width="8" />
              </template>
            </ElTableColumn>
            <ElTableColumn label="容量" width="160">
              <template #default="{ row }">
                {{ formatBytes(row.used) }} / {{ formatBytes(row.total) }}
              </template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="网卡状态">
          <ElTable :data="interfaces" size="small">
            <ElTableColumn prop="name" label="网卡" width="120" />
            <ElTableColumn label="地址" min-width="160">
              <template #default="{ row }">
                <span class="break-all">{{ (row.addrs || []).join(', ') || '-' }}</span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="状态" width="90">
              <template #default="{ row }">
                <ElTag :type="row.up ? 'success' : 'info'" size="small">{{ row.up ? 'UP' : 'DOWN' }}</ElTag>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchHostOverview, fetchNetworkStatus } from '@/api/platform'

  defineOptions({ name: 'HostOverview' })

  const loading = ref(false)
  const overview = ref<any>(null)

  const platformText = computed(() => {
    const h = overview.value?.host
    if (!h) return '-'
    return [h.platform, h.platformVersion].filter(Boolean).join(' ')
  })

  const uptimeText = computed(() => {
    const sec = Number(overview.value?.summary?.uptime || 0)
    if (!sec) return '-'
    const d = Math.floor(sec / 86400)
    const h = Math.floor((sec % 86400) / 3600)
    const m = Math.floor((sec % 3600) / 60)
    return `${d} 天 ${h} 小时 ${m} 分`
  })

  const cards = computed(() => {
    const s = overview.value?.summary || {}
    return [
      {
        label: 'CPU 使用率',
        value: `${s.cpuPercent ?? 0}%`,
        hint: `负载 ${overview.value?.load?.load1 ?? 0} / 5min ${overview.value?.load?.load5 ?? 0}`,
        icon: 'ep:cpu'
      },
      {
        label: '内存使用率',
        value: `${s.memPercent ?? 0}%`,
        hint: `共 ${formatBytes(s.memTotal)}`,
        icon: 'ri:database-2-line'
      },
      {
        label: '磁盘使用率',
        value: `${s.diskPercent ?? 0}%`,
        hint: `最高分区占用率`,
        icon: 'ep:files'
      },
      {
        label: '实时流量',
        value: formatBps(s.netInBps),
        hint: `出向 ${formatBps(s.netOutBps)}`,
        icon: 'ri:exchange-line'
      }
    ]
  })

  type ProgressStatus = '' | 'success' | 'warning' | 'exception'
  const usage = computed<{ label: string; value: string; percent: number; status: ProgressStatus }[]>(() => {
    const s = overview.value?.summary || {}
    const mem = Number(s.memPercent || 0)
    const disk = Number(s.diskPercent || 0)
    return [
      { label: 'CPU', value: `${s.cpuPercent ?? 0}%`, percent: Number(s.cpuPercent || 0), status: '' },
      {
        label: '内存',
        value: `${mem}%`,
        percent: mem,
        status: (mem >= 90 ? 'exception' : mem >= 75 ? 'warning' : '') as ProgressStatus
      },
      {
        label: '磁盘',
        value: `${disk}%`,
        percent: disk,
        status: (disk >= 90 ? 'exception' : disk >= 75 ? 'warning' : '') as ProgressStatus
      }
    ]
  })

  const interfaces = computed(() => (overview.value?.netIfaces as any[]) || [])

  const loadData = async () => {
    loading.value = true
    try {
      const data = (await fetchHostOverview()) as any
      overview.value = data
      // 网卡明细：概览接口未带，单独取一次网络状态（失败不影响主流程）
      try {
        const net = (await fetchNetworkStatus()) as any
        overview.value.netIfaces = net?.interfaces || []
      } catch {
        overview.value.netIfaces = []
      }
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

  let timer: any = null
  onMounted(() => {
    loadData()
    timer = setInterval(loadData, 15000)
  })
  onUnmounted(() => timer && clearInterval(timer))
</script>

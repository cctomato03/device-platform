<!-- 关于系统：平台版本、运行环境、数据目录、硬件摘要 -->
<template>
  <div class="art-full-height">
    <ElCard class="mb-3">
      <template #header>
        <div class="flex-cb">
          <span>{{ about?.platform?.name || '设备管理平台' }}</span>
          <ElTag type="success" size="small">v{{ about?.platform?.version || '-' }}</ElTag>
        </div>
      </template>

      <ElDescriptions :column="2" border size="small">
        <ElDescriptionsItem label="平台版本">{{ about?.platform?.version || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="运行时长">{{ uptimeText }}</ElDescriptionsItem>
        <ElDescriptionsItem label="运行环境">
          {{ about?.platform?.os || '-' }} / {{ about?.platform?.arch || '-' }}
        </ElDescriptionsItem>
        <ElDescriptionsItem label="Go 版本">{{ about?.platform?.goVersion || '-' }}</ElDescriptionsItem>
        <ElDescriptionsItem label="数据根目录">
          <span class="break-all">{{ about?.platform?.home || '-' }}</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="数据库文件">
          <span class="break-all">{{ about?.platform?.dbFile || '-' }}</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="日志目录">
          <span class="break-all">{{ about?.platform?.logDir || '-' }}</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="插件目录">
          <span class="break-all">{{ about?.platform?.pluginDir || '-' }}</span>
        </ElDescriptionsItem>
      </ElDescriptions>
    </ElCard>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="硬件摘要">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="主机名">{{ about?.host?.hostname || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="系统">
              {{ about?.host?.platform }} {{ about?.host?.platformVersion }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="内核">{{ about?.host?.kernelVersion || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="CPU">
              {{ about?.cpu?.model || '-' }}（{{ about?.cpu?.logical || 0 }} 核）
            </ElDescriptionsItem>
            <ElDescriptionsItem label="内存总量">{{ formatBytes(about?.memory?.total) }}</ElDescriptionsItem>
          </ElDescriptions>
        </ElCard>
      </ElCol>
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="设备配置摘要">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="设备名称">{{ about?.device?.name || '未设置' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="时区">{{ about?.device?.timezone || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="NTP">
              {{ about?.device?.ntp_enabled === '1' ? '已开启' : '已关闭' }} ·
              {{ about?.device?.ntp_servers || '未配置服务器' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="DNS">{{ about?.device?.dns || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="采集时间">{{ about?.timestamp || '-' }}</ElDescriptionsItem>
          </ElDescriptions>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchPlatformAbout } from '@/api/platform'

  defineOptions({ name: 'DeviceAbout' })

  const about = ref<any>(null)

  const uptimeText = computed(() => {
    const sec = Number(about.value?.platform?.uptimeSec || 0)
    const d = Math.floor(sec / 86400)
    const h = Math.floor((sec % 86400) / 3600)
    const m = Math.floor((sec % 3600) / 60)
    return `${d} 天 ${h} 小时 ${m} 分`
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

  onMounted(async () => {
    about.value = await fetchPlatformAbout()
  })
</script>

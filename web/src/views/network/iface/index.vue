<!-- 网络接口：网卡状态、地址、收发统计 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>网络接口（{{ rows.length }} 块网卡）</span>
          <ElSpace>
            <ElTag v-if="hostname" type="info">主机 {{ hostname }}</ElTag>
            <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn type="expand">
          <template #default="{ row }">
            <div class="p-3">
              <ElDescriptions :column="2" border size="small">
                <ElDescriptionsItem label="地址">
                  <span class="break-all">{{ (row.addrs || []).join(' / ') || '-' }}</span>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="MAC">{{ row.mac || '-' }}</ElDescriptionsItem>
                <ElDescriptionsItem label="MTU">{{ row.mtu }}</ElDescriptionsItem>
                <ElDescriptionsItem label="标志位">{{ (row.flags || []).join(', ') }}</ElDescriptionsItem>
                <ElDescriptionsItem label="累计接收">{{ formatBytes(row.bytesRecv) }}</ElDescriptionsItem>
                <ElDescriptionsItem label="累计发送">{{ formatBytes(row.bytesSent) }}</ElDescriptionsItem>
              </ElDescriptions>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="name" label="网卡名" width="160" />
        <ElTableColumn label="地址" min-width="220">
          <template #default="{ row }">
            <span class="break-all">{{ (row.addrs || []).join(', ') || '-' }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="状态" width="100">
          <template #default="{ row }">
            <ElTag :type="row.up ? 'success' : 'info'" size="small">{{ row.up ? 'UP' : 'DOWN' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="mac" label="MAC 地址" width="180" />
        <ElTableColumn prop="mtu" label="MTU" width="90" />
        <ElTableColumn label="接收" width="120">
          <template #default="{ row }">{{ formatBytes(row.bytesRecv) }}</template>
        </ElTableColumn>
        <ElTableColumn label="发送" width="120">
          <template #default="{ row }">{{ formatBytes(row.bytesSent) }}</template>
        </ElTableColumn>
      </ElTable>

      <ElAlert type="info" :closable="false" class="mt-3">
        说明：网卡地址、MTU、链路状态来自系统实时采集；如需修改 IP（Ubuntu Server 请用 netplan），
        可先在「排查工具」验证连通性，再通过 SSH 编辑 <code>/etc/netplan/*.yaml</code>，
        平台后续版本将支持网卡配置的可视化下发。
      </ElAlert>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchInterfaces, fetchNetworkStatus } from '@/api/platform'

  defineOptions({ name: 'NetworkIface' })

  const loading = ref(false)
  const rows = ref<any[]>([])
  const hostname = ref('')

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchInterfaces()) as any[]
      const st = (await fetchNetworkStatus()) as any
      hostname.value = st?.hostname || ''
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

  onMounted(loadData)
</script>

<!-- 防火墙：firewalld 安装与状态、当前生效规则 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12" class="mb-3">
      <ElCol :xs="24" :md="8">
        <ElCard>
          <template #header>
            <div class="flex-cb">
              <span>防火墙状态</span>
              <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
            </div>
          </template>
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="当前管理方式">
              <ElTag :type="status?.manager === 'none' ? 'info' : 'success'" size="small">{{ managerLabel }}</ElTag>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="firewalld">
              {{ status?.firewalldInstalled ? (status?.firewalldActive ? '已安装并运行' : '已安装未运行') : '未安装' }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="nftables">{{ status?.nftables ? '可用' : '不可用' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="iptables">{{ status?.iptables ? '可用' : '不可用' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="root 权限">{{ status?.root ? '是' : '否（无法下发规则）' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="策略统计">
              安全策略 {{ status?.policyCount ?? 0 }} · NAT {{ status?.natCount ?? 0 }} · QoS {{ status?.qosCount ?? 0 }}
            </ElDescriptionsItem>
          </ElDescriptions>
          <div class="mt-3">
            <ElButton
              v-auth="'security:firewall:install'"
              type="primary"
              size="small"
              :loading="installing"
              :disabled="status?.firewalldInstalled && status?.firewalldActive"
              @click="onInstall"
            >
              {{ status?.firewalldInstalled ? '启动 firewalld' : '安装 firewalld' }}
            </ElButton>
          </div>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="16">
        <ElCard>
          <template #header><span>生效规则（只读）</span></template>
          <pre v-loading="loading" class="rule-box">{{ ruleText }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElCard v-if="results.length" header="执行结果">
      <div v-for="(r, i) in results" :key="i" class="mb-2 text-sm">
        <ElTag :type="r.ok ? 'success' : 'danger'" size="small">{{ r.ok ? '成功' : '失败' }}</ElTag>
        <span class="ml-2">{{ r.item }}</span>
        <span class="ml-2 text-g-400">{{ r.message }}</span>
      </div>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchFirewallStatus, installFirewalld, type ApplyResult } from '@/api/platform'

  defineOptions({ name: 'SecurityFirewall' })

  const loading = ref(false)
  const installing = ref(false)
  const status = ref<any>(null)
  const results = ref<ApplyResult[]>([])

  const managerLabel = computed(() => {
    const m = status.value?.manager
    return (
      { firewalld: 'firewalld', nftables: 'nftables', iptables: 'iptables', none: '未检测到防火墙' }[m as string] || m
    )
  })

  const ruleText = computed(() => {
    const rules = (status.value?.rules || []) as string[]
    if (!rules.length) return '暂无规则或尚未安装防火墙'
    return rules.join('\n')
  })

  const loadData = async () => {
    loading.value = true
    try {
      status.value = await fetchFirewallStatus()
    } finally {
      loading.value = false
    }
  }

  const onInstall = async () => {
    installing.value = true
    try {
      const res = (await installFirewalld()) as any
      results.value = res?.results || []
      await loadData()
    } finally {
      installing.value = false
    }
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .rule-box {
    min-height: 300px;
    max-height: 520px;
    padding: 12px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

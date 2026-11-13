<!-- DNS 配置：查看/修改系统 DNS 服务器 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="DNS 服务器">
          <ElForm label-width="100px">
            <ElFormItem label="DNS 列表">
              <ElTooltip content="每行一个，例如 223.5.5.5" placement="top">
                <ElInput v-model="dnsText" type="textarea" :rows="5" placeholder="223.5.5.5&#10;114.114.114.114" />
              </ElTooltip>
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'network:dns:update'" type="primary" :loading="saving" @click="onSave">保存并生效</ElButton>
              <ElButton :loading="loading" @click="loadData">重新读取</ElButton>
            </ElFormItem>
          </ElForm>
          <ElAlert v-if="results.length" :type="results[0].ok ? 'success' : 'error'" :closable="false">
            {{ results[0].item }}：{{ results[0].message }}
          </ElAlert>
          <ElAlert type="info" :closable="false" class="mt-2">
            保存会写入 <code>/etc/resolv.conf</code>；若系统启用了 systemd-resolved 或 netplan 管理 DNS，
            请同时在对应配置中保持一致，避免重启后被覆盖。
          </ElAlert>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="当前解析配置">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="配置文件">{{ config?.source || '/etc/resolv.conf' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="DNS 服务器">
              <ElSpace wrap>
                <ElTag v-for="s in config?.servers || []" :key="s" type="success" size="small">{{ s }}</ElTag>
                <span v-if="!(config?.servers || []).length" class="text-g-400">未配置</span>
              </ElSpace>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="搜索域">{{ (config?.search || []).join(', ') || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="默认网关">{{ gateway || '-' }}</ElDescriptionsItem>
          </ElDescriptions>
          <div class="mt-3">
            <p class="text-sm font-medium mb-1">路由表</p>
            <pre class="route-box">{{ (routes || []).join('\n') || '无（非 Linux 环境或缺少 iproute2）' }}</pre>
          </div>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage } from 'element-plus'
  import { fetchDNSConfig, fetchNetworkStatus, updateDNSConfig, type ApplyResult } from '@/api/platform'

  defineOptions({ name: 'NetworkDNS' })

  const loading = ref(false)
  const saving = ref(false)
  const config = ref<any>(null)
  const dnsText = ref('')
  const results = ref<ApplyResult[]>([])
  const routes = ref<string[]>([])
  const gateway = ref('')

  const loadData = async () => {
    loading.value = true
    try {
      config.value = await fetchDNSConfig()
      dnsText.value = (config.value?.servers || []).join('\n')
      const st = (await fetchNetworkStatus()) as any
      routes.value = st?.routes || []
      gateway.value = st?.gateway || ''
    } finally {
      loading.value = false
    }
  }

  const onSave = async () => {
    const servers = dnsText.value
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
    if (!servers.length) {
      ElMessage.warning('请至少填写一个 DNS 服务器')
      return
    }
    saving.value = true
    try {
      const res = (await updateDNSConfig(servers)) as any
      results.value = res?.results || []
      await loadData()
    } finally {
      saving.value = false
    }
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .route-box {
    max-height: 260px;
    padding: 8px;
    overflow: auto;
    font-size: 12px;
    line-height: 1.6;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

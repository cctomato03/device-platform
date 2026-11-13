<!-- DNS 服务：dnsmasq 本地解析 + 上游转发 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="14">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>DNS 服务（dnsmasq）</span>
              <ElSpace>
                <ElTag size="small" :type="status?.active ? 'success' : 'info'">
                  {{ status?.active ? '运行中' : '未运行' }}
                </ElTag>
                <ElButton v-auth="'network:dnsserver:install'" size="small" @click="onInstall">安装依赖</ElButton>
                <ElButton v-auth="'network:dnsserver:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElForm label-width="110px">
            <ElFormItem label="启用 DNS">
              <ElSwitch v-model="form.enabled" />
            </ElFormItem>
            <ElFormItem label="监听地址">
              <ElInput v-model="form.listenAddr" placeholder="0.0.0.0 或具体网卡 IP" />
            </ElFormItem>
            <ElFormItem label="上游 DNS" required>
              <ElInput v-model="form.upstreams" placeholder="223.5.5.5,8.8.8.8" />
            </ElFormItem>
            <ElFormItem label="缓存条数">
              <ElInputNumber v-model="form.cacheSize" :min="150" :max="100000" />
            </ElFormItem>
            <ElFormItem label="本地解析">
              <div class="w-full">
                <div v-for="(r, idx) in records" :key="idx" class="mb-2 flex gap-2">
                  <ElInput v-model="r.name" placeholder="域名（nas.lan）" size="small" />
                  <ElInput v-model="r.ip" placeholder="IP 地址" size="small" />
                  <ElButton size="small" link type="danger" @click="records.splice(idx, 1)">移除</ElButton>
                </div>
                <ElButton size="small" @click="records.push({ name: '', ip: '' })">+ 添加记录</ElButton>
              </div>
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'network:dnsserver:update'" type="primary" :loading="saving" @click="onSave">
                保存配置
              </ElButton>
              <ElButton @click="loadData">重置</ElButton>
            </ElFormItem>
          </ElForm>

          <ElCard v-if="results.length" shadow="never" header="下发结果">
            <ResultList :results="results" />
          </ElCard>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="10">
        <ElCard class="mb-3">
          <template #header><span>服务状态</span></template>
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="dnsmasq">{{ status?.installed ? '已安装' : '未安装' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="运行状态">{{ status?.active ? '运行中' : '未运行' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="版本">{{ status?.version || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="配置文件">{{ status?.confFile || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="租约数">{{ (status?.leases || []).length }}</ElDescriptionsItem>
          </ElDescriptions>
          <ElAlert v-if="(status?.notes || []).length" type="info" :closable="false" class="mt-3">
            <p v-for="(n, i) in status?.notes || []" :key="i">{{ n }}</p>
          </ElAlert>
        </ElCard>

        <ElCard>
          <template #header><span>生成的 dnsmasq 配置</span></template>
          <pre class="code-box">{{ status?.confPreview || '（尚未生成）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import {
    applyDnsServerConfig,
    fetchDnsServerConfig,
    installDnsmasq,
    saveDnsServerConfig,
    type ApplyResult
  } from '@/api/ops'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkDnsServer' })

  const saving = ref(false)
  const status = ref<any>(null)
  const results = ref<ApplyResult[]>([])
  const records = ref<any[]>([])
  const form = reactive<Record<string, any>>({
    enabled: false,
    listenAddr: '0.0.0.0',
    upstreams: '223.5.5.5',
    cacheSize: 1000
  })

  const loadData = async () => {
    const res = (await fetchDnsServerConfig()) as any
    const cfg = res?.config || {}
    Object.assign(form, {
      enabled: !!cfg.enabled,
      listenAddr: cfg.listenAddr || '0.0.0.0',
      upstreams: cfg.upstreams || '',
      cacheSize: cfg.cacheSize || 1000
    })
    try {
      records.value = JSON.parse(cfg.records || '[]')
    } catch {
      records.value = []
    }
    status.value = res?.status || null
  }

  const onSave = async () => {
    saving.value = true
    try {
      await saveDnsServerConfig({ ...form, records: records.value })
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onApply = async () => {
    const res = (await applyDnsServerConfig()) as any
    results.value = res?.results || []
    await loadData()
  }

  const onInstall = async () => {
    const res = (await installDnsmasq('dns')) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 300px;
    padding: 10px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

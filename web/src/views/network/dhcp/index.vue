<!-- DHCP 服务：dnsmasq 地址池 / 网关 / DNS / 静态绑定 + 租约查看 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="14">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>DHCP 服务（dnsmasq）</span>
              <ElSpace>
                <ElTag size="small" :type="status?.active ? 'success' : 'info'">
                  {{ status?.active ? '运行中' : '未运行' }}
                </ElTag>
                <ElButton v-auth="'network:dhcp:install'" size="small" @click="onInstall">安装依赖</ElButton>
                <ElButton v-auth="'network:dhcp:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElForm label-width="110px">
            <ElFormItem label="启用 DHCP">
              <ElSwitch v-model="form.enabled" />
            </ElFormItem>
            <ElFormItem label="服务网卡" required>
              <ElSelect v-model="form.iface" filterable allow-create style="width: 100%">
                <ElOption v-for="i in ifaces" :key="i.name" :label="i.name" :value="i.name" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem label="地址池起" required>
              <ElInput v-model="form.rangeStart" placeholder="192.168.1.100" />
            </ElFormItem>
            <ElFormItem label="地址池止" required>
              <ElInput v-model="form.rangeEnd" placeholder="192.168.1.200" />
            </ElFormItem>
            <ElFormItem label="子网掩码">
              <ElInput v-model="form.netmask" placeholder="255.255.255.0 或 24" />
            </ElFormItem>
            <ElFormItem label="租期(小时)">
              <ElInputNumber v-model="form.leaseHours" :min="1" :max="720" />
            </ElFormItem>
            <ElFormItem label="下发网关">
              <ElInput v-model="form.gateway" placeholder="192.168.1.1" />
            </ElFormItem>
            <ElFormItem label="下发 DNS">
              <ElInput v-model="form.dns" placeholder="223.5.5.5（多个用逗号分隔）" />
            </ElFormItem>
            <ElFormItem label="域名后缀">
              <ElInput v-model="form.domain" placeholder="lan" />
            </ElFormItem>
            <ElFormItem label="静态绑定">
              <div class="w-full">
                <div v-for="(b, idx) in bindings" :key="idx" class="mb-2 flex gap-2">
                  <ElInput v-model="b.mac" placeholder="MAC（aa:bb:cc:dd:ee:ff）" size="small" />
                  <ElInput v-model="b.ip" placeholder="IP" size="small" />
                  <ElInput v-model="b.hostname" placeholder="主机名（可选）" size="small" />
                  <ElButton size="small" link type="danger" @click="bindings.splice(idx, 1)">移除</ElButton>
                </div>
                <ElButton size="small" @click="bindings.push({ mac: '', ip: '', hostname: '' })">+ 添加绑定</ElButton>
              </div>
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'network:dhcp:update'" type="primary" :loading="saving" @click="onSave">保存配置</ElButton>
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
          <template #header>
            <div class="flex-cb">
              <span>租约（{{ (status?.leases || []).length }}）</span>
              <ElButton size="small" @click="loadData">刷新</ElButton>
            </div>
          </template>
          <ElTable :data="status?.leases || []" size="small" max-height="240">
            <ElTableColumn prop="ip" label="IP" width="130" />
            <ElTableColumn prop="mac" label="MAC" width="140" />
            <ElTableColumn prop="hostname" label="主机名" min-width="100" />
            <ElTableColumn prop="expireAt" label="到期" width="160" />
          </ElTable>
          <ElAlert v-if="(status?.notes || []).length" type="warning" :closable="false" class="mt-3">
            <p v-for="(n, i) in status?.notes || []" :key="i">{{ n }}</p>
          </ElAlert>
        </ElCard>

        <ElCard>
          <template #header><span>生成的 dnsmasq 配置</span></template>
          <pre class="code-box">{{ status?.confPreview || '（尚未生成，保存后点「下发配置」）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage } from 'element-plus'
  import {
    applyDhcpConfig,
    fetchDhcpConfig,
    installDnsmasq,
    saveDhcpConfig,
    type ApplyResult
  } from '@/api/ops'
  import { fetchInterfaces } from '@/api/platform'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkDhcp' })

  const saving = ref(false)
  const ifaces = ref<any[]>([])
  const status = ref<any>(null)
  const results = ref<ApplyResult[]>([])
  const bindings = ref<any[]>([])
  const form = reactive<Record<string, any>>({
    enabled: false,
    iface: '',
    rangeStart: '',
    rangeEnd: '',
    netmask: '255.255.255.0',
    leaseHours: 12,
    gateway: '',
    dns: '',
    domain: ''
  })

  const loadData = async () => {
    const res = (await fetchDhcpConfig()) as any
    const cfg = res?.config || {}
    Object.assign(form, {
      enabled: !!cfg.enabled,
      iface: cfg.iface || '',
      rangeStart: cfg.rangeStart || '',
      rangeEnd: cfg.rangeEnd || '',
      netmask: cfg.netmask || '255.255.255.0',
      leaseHours: cfg.leaseHours || 12,
      gateway: cfg.gateway || '',
      dns: cfg.dns || '',
      domain: cfg.domain || ''
    })
    try {
      bindings.value = JSON.parse(cfg.staticBindings || '[]')
    } catch {
      bindings.value = []
    }
    status.value = res?.status || null
    try {
      ifaces.value = (await fetchInterfaces()) as any[]
    } catch {
      ifaces.value = []
    }
  }

  const onSave = async () => {
    saving.value = true
    try {
      await saveDhcpConfig({ ...form, staticBindings: bindings.value })
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onApply = async () => {
    const res = (await applyDhcpConfig()) as any
    results.value = res?.results || []
    await loadData()
  }

  const onInstall = async () => {
    const res = (await installDnsmasq('dhcp')) as any
    results.value = res?.results || []
    ElMessage.success('安装指令已执行')
    await loadData()
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 260px;
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

<!-- SNMP 服务：v2c/v3 配置、依赖安装、状态与验证命令 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="14">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>SNMP 服务（net-snmp）</span>
              <ElSpace>
                <ElTag size="small" :type="status?.active ? 'success' : 'info'">
                  {{ status?.active ? '运行中' : '未运行' }}
                </ElTag>
                <ElButton v-auth="'device:snmp:install'" size="small" @click="onInstall">安装依赖</ElButton>
                <ElButton v-auth="'device:snmp:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElForm label-width="120px">
            <ElFormItem label="启用 SNMP">
              <ElSwitch v-model="form.enabled" />
            </ElFormItem>
            <ElFormItem label="协议版本">
              <ElRadioGroup v-model="form.version">
                <ElRadioButton value="2c">v2c</ElRadioButton>
                <ElRadioButton value="3">v3</ElRadioButton>
              </ElRadioGroup>
            </ElFormItem>
            <ElFormItem v-if="form.version === '2c'" label="Community" required>
              <ElInput v-model="form.community" placeholder="不要使用默认 public" show-password />
            </ElFormItem>
            <template v-else>
              <ElFormItem label="用户名" required>
                <ElInput v-model="form.v3User" placeholder="snmp_ro" />
              </ElFormItem>
              <ElFormItem label="密码">
                <ElInput v-model="form.v3Password" type="password" show-password placeholder="留空表示不修改" />
                <p class="text-xs text-g-400 mt-1">下发时会用该密码创建/更新 v3 用户（SHA 认证 + AES 加密）</p>
              </ElFormItem>
            </template>
            <ElFormItem label="监听地址">
              <ElInput v-model="form.listenAddr" placeholder="0.0.0.0 或具体网卡 IP" />
            </ElFormItem>
            <ElFormItem label="允许来源">
              <ElInput v-model="form.allowedHost" placeholder="监控服务器 IP（留空表示全部允许）" />
            </ElFormItem>
            <ElFormItem label="设备位置">
              <ElInput v-model="form.location" placeholder="机房 A / 3 号机柜" />
            </ElFormItem>
            <ElFormItem label="联系人">
              <ElInput v-model="form.contact" placeholder="ops@example.com" />
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'device:snmp:update'" type="primary" :loading="saving" @click="onSave">保存配置</ElButton>
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
            <ElDescriptionsItem label="snmpd">{{ status?.installed ? '已安装' : '未安装' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="运行状态">{{ status?.active ? '运行中' : '未运行' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="版本">{{ status?.version || '-' }}</ElDescriptionsItem>
          </ElDescriptions>
          <p class="text-xs text-g-500 mt-3">验证命令（在监控服务器上执行）：</p>
          <pre class="code-box small">{{ status?.walkHint || '-' }}</pre>
          <ElAlert v-if="(status?.notes || []).length" type="info" :closable="false" class="mt-2">
            <p v-for="(n, i) in status?.notes || []" :key="i">{{ n }}</p>
          </ElAlert>
        </ElCard>

        <ElCard>
          <template #header><span>生成的 snmpd.conf</span></template>
          <pre class="code-box">{{ status?.confPreview || '（尚未生成）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { applySnmpConfig, fetchSnmpConfig, installSNMP, saveSnmpConfig, type ApplyResult } from '@/api/ops'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'DeviceSnmp' })

  const saving = ref(false)
  const status = ref<any>(null)
  const results = ref<ApplyResult[]>([])
  const form = reactive<Record<string, any>>({
    enabled: false,
    version: '2c',
    community: 'public',
    v3User: '',
    v3Password: '',
    listenAddr: '0.0.0.0',
    allowedHost: '',
    location: '',
    contact: ''
  })

  const loadData = async () => {
    const res = (await fetchSnmpConfig()) as any
    const cfg = res?.config || {}
    Object.assign(form, {
      enabled: !!cfg.enabled,
      version: cfg.version || '2c',
      community: cfg.community || 'public',
      v3User: cfg.v3User || '',
      v3Password: '',
      listenAddr: cfg.listenAddr || '0.0.0.0',
      allowedHost: cfg.allowedHost || '',
      location: cfg.location || '',
      contact: cfg.contact || ''
    })
    status.value = res?.status || null
  }

  const onSave = async () => {
    saving.value = true
    try {
      await saveSnmpConfig({ ...form })
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onApply = async () => {
    const res = (await applySnmpConfig()) as any
    results.value = res?.results || []
    await loadData()
  }

  const onInstall = async () => {
    const res = (await installSNMP()) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 240px;
    padding: 10px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }

  .code-box.small {
    max-height: 80px;
    font-size: 11px;
  }
</style>

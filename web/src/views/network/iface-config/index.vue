<!-- 网卡配置：静态地址 / DHCP / MTU（netplan 下发，含片段预览） -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="15">
        <ElCard class="art-table-card">
          <template #header>
            <div class="flex-cb">
              <span>网卡地址配置</span>
              <ElSpace>
                <ElButton v-auth="'network:ifacecfg:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
                <ElButton v-auth="'network:ifacecfg:update'" size="small" type="primary" @click="openForm()">
                  新增网卡
                </ElButton>
                <ElButton size="small" @click="loadData">刷新</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElAlert type="warning" :closable="false" class="mb-3">
            下发会写入 <code>{{ meta?.status?.file || '/etc/netplan/90-devplat.yaml' }}</code> 并执行
            <code>netplan generate/apply</code>。校验失败会自动回滚；<strong>改错可能导致失联，请先确认控制台可用</strong>。
          </ElAlert>

          <ElTable v-loading="loading" :data="rows.filter((r) => r.kind === 'ethernet')" size="small">
            <ElTableColumn prop="name" label="网卡" width="120" />
            <ElTableColumn label="获取方式" width="110">
              <template #default="{ row }">
                <ElTag size="small" :type="row.dhcp4 ? 'warning' : 'success'">{{ row.dhcp4 ? 'DHCP' : '静态' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="addresses" label="地址" min-width="180" show-overflow-tooltip />
            <ElTableColumn prop="gateway" label="网关" width="140" />
            <ElTableColumn prop="mtu" label="MTU" width="80" />
            <ElTableColumn label="实时状态" width="140">
              <template #default="{ row }">
                <ElTag size="small" :type="row.up ? 'success' : 'info'">{{ row.up ? 'UP' : 'DOWN' }}</ElTag>
                <span class="ml-1 text-xs text-g-400">{{ (row.addrs || []).join(',') }}</span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="已下发" width="90">
              <template #default="{ row }">
                <ElTag size="small" :type="row.applied ? 'success' : 'warning'">{{ row.applied ? '是' : '否' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <ElButton v-auth="'network:ifacecfg:update'" link type="primary" size="small" @click="openForm(row)">
                  编辑
                </ElButton>
                <ElButton v-auth="'network:ifacecfg:update'" link type="danger" size="small" @click="onDelete(row)">
                  删除
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>

          <ElCard v-if="results.length" class="mt-3" shadow="never" header="下发结果">
            <ResultList :results="results" />
          </ElCard>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="9">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>netplan 片段预览</span>
              <ElButton size="small" @click="loadPreview">刷新预览</ElButton>
            </div>
          </template>
          <pre class="code-box">{{ preview || '（暂无配置）' }}</pre>
          <p class="text-xs text-g-400 mt-2">
            netplan 工具：{{ meta?.status?.installed ? '已安装' : '未检测到' }} ·
            托管文件：{{ meta?.status?.exists ? '已存在' : '尚未生成' }}
          </p>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑网卡配置' : '新增网卡配置'" width="560px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="网卡名" required>
          <ElInput v-model="form.name" placeholder="eth0" />
        </ElFormItem>
        <ElFormItem label="地址方式">
          <ElRadioGroup v-model="form.dhcp4">
            <ElRadioButton :value="false">静态地址</ElRadioButton>
            <ElRadioButton :value="true">DHCP</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem v-if="!form.dhcp4" label="地址" required>
          <ElInput v-model="form.addresses" placeholder="192.168.1.10/24，多个用逗号分隔" />
        </ElFormItem>
        <ElFormItem v-if="!form.dhcp4" label="网关">
          <ElInput v-model="form.gateway" placeholder="192.168.1.1" />
        </ElFormItem>
        <ElFormItem label="MTU">
          <ElInputNumber v-model="form.mtu" :min="0" :max="9000" />
          <span class="ml-2 text-xs text-g-400">0 表示不修改</span>
        </ElFormItem>
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled" />
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="form.remark" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="formVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="saving" @click="onSave">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessageBox } from 'element-plus'
  import {
    applyIfaceConfigs,
    deleteIfaceConfig,
    fetchIfaceConfigs,
    fetchIfaceMeta,
    fetchNetplanPreview,
    saveIfaceConfig,
    type ApplyResult
  } from '@/api/ops'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkIfaceConfig' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const meta = ref<any>(null)
  const preview = ref('')
  const results = ref<ApplyResult[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const emptyForm = () => ({
    id: '',
    name: '',
    kind: 'ethernet',
    dhcp4: false,
    addresses: '',
    gateway: '',
    mtu: 0,
    members: '',
    bondMode: 'active-backup',
    stp: false,
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchIfaceConfigs()) as any[]
      meta.value = await fetchIfaceMeta()
    } finally {
      loading.value = false
    }
  }

  const loadPreview = async () => {
    try {
      const res = (await fetchNetplanPreview()) as any
      preview.value = res?.preview || ''
    } catch {
      preview.value = '（暂无启用的网卡配置）'
    }
  }

  const openForm = (row?: any) => {
    Object.keys(form).forEach((k) => delete form[k])
    Object.assign(form, emptyForm(), row ? { ...row } : {})
    formVisible.value = true
  }

  const onSave = async () => {
    saving.value = true
    try {
      await saveIfaceConfig({ ...form })
      formVisible.value = false
      await loadData()
      loadPreview()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除网卡配置「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteIfaceConfig(row.id)
    await loadData()
    loadPreview()
  }

  const onApply = async () => {
    const res = (await applyIfaceConfigs()) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(async () => {
    await loadData()
    loadPreview()
  })
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 460px;
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

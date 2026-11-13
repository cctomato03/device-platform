<!-- 带宽管理 QOS：接口限速规则与下发 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>带宽限速（tc htb / ingress police）</span>
          <ElSpace>
            <ElButton v-auth="'security:qos:apply'" size="small" type="warning" @click="onApply">下发全部</ElButton>
            <ElButton v-auth="'security:qos:create'" size="small" type="primary" @click="openForm()">新增规则</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        出向（egress）使用 htb 队列限速；入向（ingress）使用 police 丢弃超出流量。
        下发会先清理该网卡的平台根队列，需要 root 与 iproute2。
      </ElAlert>

      <ElAlert v-if="warnings.length" type="warning" :closable="false" class="mb-3" title="检测到潜在冲突（已保存，未阻断）">
        <p v-for="(w, i) in warnings" :key="i" class="text-sm">{{ w }}</p>
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="name" label="名称" min-width="130" />
        <ElTableColumn prop="iface" label="网卡" width="120" />
        <ElTableColumn label="方向" width="100">
          <template #default="{ row }">{{ row.direction === 'egress' ? '出向' : '入向' }}</template>
        </ElTableColumn>
        <ElTableColumn label="限速" width="140">
          <template #default="{ row }">{{ formatRate(row.rateKbps) }}</template>
        </ElTableColumn>
        <ElTableColumn label="突发" width="100">
          <template #default="{ row }">{{ row.burstKb ? row.burstKb + ' KB' : '默认' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="target" label="目标" width="160" />
        <ElTableColumn label="启用" width="80">
          <template #default="{ row }">
            <ElTag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="已下发" width="90">
          <template #default="{ row }">
            <ElTag :type="row.applied ? 'success' : 'warning'" size="small">{{ row.applied ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'security:qos:update'" link type="primary" size="small" @click="openForm(row)">编辑</ElButton>
            <ElButton v-auth="'security:qos:delete'" link type="danger" size="small" @click="onDelete(row)">删除</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElCard v-if="results.length" class="mt-3" shadow="never" header="下发结果">
        <div v-for="(r, i) in results" :key="i" class="mb-1 text-sm">
          <ElTag :type="r.ok ? 'success' : 'danger'" size="small">{{ r.ok ? '成功' : '失败' }}</ElTag>
          <span class="ml-2">{{ r.item }}</span>
          <span class="ml-2 text-g-400">{{ r.message }}</span>
        </div>
      </ElCard>
    </ElCard>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑限速规则' : '新增限速规则'" width="520px">
      <ElForm :model="form" label-width="100px">
        <ElFormItem label="名称" required>
          <ElInput v-model="form.name" placeholder="办公网限速 50M" />
        </ElFormItem>
        <ElFormItem label="网卡" required>
          <ElSelect v-model="form.iface" filterable allow-create style="width: 100%">
            <ElOption v-for="i in interfaces" :key="i.name" :label="i.name" :value="i.name" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="方向">
          <ElRadioGroup v-model="form.direction">
            <ElRadioButton value="egress">出向</ElRadioButton>
            <ElRadioButton value="ingress">入向</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="限速 (Kbps)" required>
          <ElInputNumber v-model="form.rateKbps" :min="8" :max="100000000" />
        </ElFormItem>
        <ElFormItem label="突发 (KB)">
          <ElInputNumber v-model="form.burstKb" :min="0" :max="1048576" />
        </ElFormItem>
        <ElFormItem label="目标网段">
          <ElInput v-model="form.target" placeholder="留空表示整个网卡" />
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
    applyQosRules,
    deleteQosRule,
    fetchInterfaces,
    fetchQosRules,
    saveQosRule,
    type ApplyResult
  } from '@/api/platform'

  defineOptions({ name: 'SecurityQos' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const interfaces = ref<any[]>([])
  const results = ref<ApplyResult[]>([])
  const warnings = ref<string[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const formatRate = (kbps?: number) => {
    if (!kbps) return '-'
    if (kbps >= 1024 * 1024) return `${(kbps / 1024 / 1024).toFixed(1)} Gbps`
    if (kbps >= 1024) return `${(kbps / 1024).toFixed(1)} Mbps`
    return `${kbps} Kbps`
  }

  const emptyForm = () => ({
    id: '',
    name: '',
    iface: '',
    direction: 'egress',
    rateKbps: 10240,
    burstKb: 32,
    target: '',
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchQosRules()) as any[]
      interfaces.value = (await fetchInterfaces()) as any[]
    } finally {
      loading.value = false
    }
  }

  const openForm = (row?: any) => {
    Object.keys(form).forEach((k) => delete form[k])
    Object.assign(form, emptyForm(), row ? { ...row } : {})
    if (!form.iface && interfaces.value.length) form.iface = interfaces.value[0].name
    formVisible.value = true
  }

  const onSave = async () => {
    saving.value = true
    try {
      const res = (await saveQosRule({ ...form })) as any
      warnings.value = res?.warnings || []
      formVisible.value = false
      loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除限速规则「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteQosRule(row.id)
    loadData()
  }

  const onApply = async () => {
    const res = (await applyQosRules()) as any
    results.value = res?.results || []
    loadData()
  }

  onMounted(loadData)
</script>

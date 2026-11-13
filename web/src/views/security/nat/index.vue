<!-- NAT 策略：SNAT / DNAT 配置与下发 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>NAT 策略</span>
          <ElSpace>
            <ElButton v-auth="'security:nat:apply'" size="small" type="warning" @click="onApply">下发全部</ElButton>
            <ElButton v-auth="'security:nat:create'" size="small" type="primary" @click="openForm()">新增规则</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        下发会重建 <code>DEVPLAT_NAT</code> 链（nat 表 POSTROUTING），需要 root 与 iptables。
      </ElAlert>

      <ElAlert v-if="warnings.length" type="warning" :closable="false" class="mb-3" title="检测到潜在冲突（已保存，未阻断）">
        <p v-for="(w, i) in warnings" :key="i" class="text-sm">{{ w }}</p>
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="name" label="名称" min-width="130" />
        <ElTableColumn label="类型" width="90">
          <template #default="{ row }">
            <ElTag :type="row.type === 'dnat' ? 'warning' : 'success'" size="small">{{ row.type.toUpperCase() }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="protocol" label="协议" width="80" />
        <ElTableColumn prop="srcIface" label="出口网卡" width="110" />
        <ElTableColumn prop="srcAddr" label="源地址" width="140" />
        <ElTableColumn prop="dstAddr" label="目的地址" width="140" />
        <ElTableColumn prop="dstPort" label="目的端口" width="100" />
        <ElTableColumn label="转换到" width="180">
          <template #default="{ row }">{{ row.toAddr }}{{ row.toPort ? ':' + row.toPort : '' }}</template>
        </ElTableColumn>
        <ElTableColumn label="启用" width="80">
          <template #default="{ row }">
            <ElTag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'security:nat:update'" link type="primary" size="small" @click="openForm(row)">编辑</ElButton>
            <ElButton v-auth="'security:nat:delete'" link type="danger" size="small" @click="onDelete(row)">删除</ElButton>
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

    <ElDialog v-model="formVisible" :title="form.id ? '编辑 NAT 规则' : '新增 NAT 规则'" width="560px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="名称" required>
          <ElInput v-model="form.name" placeholder="内网服务器映射" />
        </ElFormItem>
        <ElFormItem label="类型">
          <ElRadioGroup v-model="form.type">
            <ElRadioButton value="snat">SNAT（源地址转换）</ElRadioButton>
            <ElRadioButton value="dnat">DNAT（端口映射）</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="协议">
          <ElSelect v-model="form.protocol" style="width: 100%">
            <ElOption label="TCP" value="tcp" />
            <ElOption label="UDP" value="udp" />
            <ElOption label="任意" value="any" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="form.type === 'snat'" label="出口网卡">
          <ElInput v-model="form.srcIface" placeholder="eth0" />
        </ElFormItem>
        <ElFormItem label="源地址">
          <ElInput v-model="form.srcAddr" placeholder="192.168.1.0/24" />
        </ElFormItem>
        <ElFormItem v-if="form.type === 'dnat'" label="目的地址">
          <ElInput v-model="form.dstAddr" placeholder="公网地址（留空表示任意）" />
        </ElFormItem>
        <ElFormItem v-if="form.type === 'dnat'" label="目的端口">
          <ElInputNumber v-model="form.dstPort" :min="0" :max="65535" />
        </ElFormItem>
        <ElFormItem :label="form.type === 'dnat' ? '转发到地址' : '转换为源地址'">
          <ElInput v-model="form.toAddr" placeholder="10.0.0.10" />
        </ElFormItem>
        <ElFormItem v-if="form.type === 'dnat'" label="转发到端口">
          <ElInputNumber v-model="form.toPort" :min="0" :max="65535" />
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
  import { applyNATRules, deleteNATRule, fetchNATRules, saveNATRule, type ApplyResult } from '@/api/platform'

  defineOptions({ name: 'SecurityNAT' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const results = ref<ApplyResult[]>([])
  const warnings = ref<string[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const emptyForm = () => ({
    id: '',
    name: '',
    type: 'dnat',
    protocol: 'tcp',
    srcIface: '',
    srcAddr: '',
    dstAddr: '',
    dstPort: 0,
    toAddr: '',
    toPort: 0,
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchNATRules()) as any[]
    } finally {
      loading.value = false
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
      const res = (await saveNATRule({ ...form })) as any
      warnings.value = res?.warnings || []
      formVisible.value = false
      loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除 NAT 规则「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteNATRule(row.id)
    loadData()
  }

  const onApply = async () => {
    const res = (await applyNATRules()) as any
    results.value = res?.results || []
    loadData()
  }

  onMounted(loadData)
</script>

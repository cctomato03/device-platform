<!-- 安全策略：一体化访问控制（in/out/forward），下发到平台专属 iptables 链 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>安全策略（按优先级从小到大生效）</span>
          <ElSpace>
            <ElButton v-auth="'security:policy:apply'" size="small" type="warning" @click="onApply">下发全部</ElButton>
            <ElButton v-auth="'security:policy:create'" size="small" type="primary" @click="openForm()">新增策略</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        下发会重建平台专属链 <code>DEVPLAT_IN / DEVPLAT_OUT / DEVPLAT_FWD</code> 并挂到系统链首位，
        不改动系统已有规则；需要 root 权限，且系统需安装 iptables。
      </ElAlert>

      <ElAlert v-if="warnings.length" type="warning" :closable="false" class="mb-3" title="检测到潜在冲突（已保存，未阻断）">
        <p v-for="(w, i) in warnings" :key="i" class="text-sm">{{ w }}</p>
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="priority" label="优先级" width="90" />
        <ElTableColumn prop="name" label="策略名称" min-width="130" />
        <ElTableColumn label="方向" width="90">
          <template #default="{ row }">{{ dirLabel(row.direction) }}</template>
        </ElTableColumn>
        <ElTableColumn label="动作" width="90">
          <template #default="{ row }">
            <ElTag :type="row.action === 'accept' ? 'success' : 'danger'" size="small">{{ row.action }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="protocol" label="协议" width="80" />
        <ElTableColumn prop="srcAddr" label="源地址" width="150" />
        <ElTableColumn prop="dstAddr" label="目的地址" width="150" />
        <ElTableColumn prop="port" label="端口" width="80" />
        <ElTableColumn label="启用" width="80">
          <template #default="{ row }">
            <ElTag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'security:policy:update'" link type="primary" size="small" @click="openForm(row)">编辑</ElButton>
            <ElButton v-auth="'security:policy:delete'" link type="danger" size="small" @click="onDelete(row)">删除</ElButton>
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

    <ElDialog v-model="formVisible" :title="form.id ? '编辑策略' : '新增策略'" width="560px">
      <ElForm :model="form" label-width="100px">
        <ElFormItem label="策略名称" required>
          <ElInput v-model="form.name" placeholder="允许运维网段访问 SSH" />
        </ElFormItem>
        <ElFormItem label="方向">
          <ElSelect v-model="form.direction" style="width: 100%">
            <ElOption label="入向（INPUT）" value="in" />
            <ElOption label="出向（OUTPUT）" value="out" />
            <ElOption label="转发（FORWARD）" value="forward" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="动作">
          <ElSelect v-model="form.action" style="width: 100%">
            <ElOption label="允许 accept" value="accept" />
            <ElOption label="丢弃 drop" value="drop" />
            <ElOption label="拒绝 reject" value="reject" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="协议">
          <ElSelect v-model="form.protocol" style="width: 100%">
            <ElOption label="TCP" value="tcp" />
            <ElOption label="UDP" value="udp" />
            <ElOption label="ICMP" value="icmp" />
            <ElOption label="任意" value="any" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="源地址">
          <ElInput v-model="form.srcAddr" placeholder="192.168.1.0/24（留空表示任意）" />
        </ElFormItem>
        <ElFormItem label="目的地址">
          <ElInput v-model="form.dstAddr" placeholder="10.0.0.10（留空表示任意）" />
        </ElFormItem>
        <ElFormItem label="端口">
          <ElInputNumber v-model="form.port" :min="0" :max="65535" />
        </ElFormItem>
        <ElFormItem label="优先级">
          <ElInputNumber v-model="form.priority" :min="1" :max="9999" />
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
  import { applyPolicies, deletePolicy, fetchPolicies, savePolicy, type ApplyResult } from '@/api/platform'

  defineOptions({ name: 'SecurityPolicy' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const results = ref<ApplyResult[]>([])
  const warnings = ref<string[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const dirLabel = (d: string) => ({ in: '入向', out: '出向', forward: '转发' })[d] || d

  const emptyForm = () => ({
    id: '',
    name: '',
    direction: 'in',
    action: 'accept',
    protocol: 'tcp',
    srcAddr: '',
    dstAddr: '',
    port: 0,
    priority: 100,
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchPolicies()) as any[]
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
      const res = (await savePolicy({ ...form })) as any
      warnings.value = res?.warnings || []
      formVisible.value = false
      loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除策略「${row.name}」？`, '删除确认', { type: 'warning' })
    await deletePolicy(row.id)
    loadData()
  }

  const onApply = async () => {
    const res = (await applyPolicies()) as any
    results.value = res?.results || []
    loadData()
  }

  onMounted(loadData)
</script>

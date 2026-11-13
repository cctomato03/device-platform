<!-- 静态路由：期望态维护 + 一键下发到内核 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>静态路由</span>
          <ElSpace>
            <ElButton v-auth="'network:route:apply'" size="small" type="warning" @click="onApply">下发全部</ElButton>
            <ElButton v-auth="'network:route:create'" size="small" type="primary" @click="openForm()">新增路由</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="warning" :closable="false" class="mb-3">
        路由下发依赖 Linux <code>ip route</code>，需要平台以 root 运行；下发失败会在下方逐条提示原因。
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="destination" label="目标网段" min-width="160" />
        <ElTableColumn prop="gateway" label="网关" width="150" />
        <ElTableColumn prop="device" label="出口网卡" width="120" />
        <ElTableColumn prop="metric" label="Metric" width="90" />
        <ElTableColumn prop="table" label="路由表" width="100" />
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
        <ElTableColumn prop="remark" label="备注" min-width="140" show-overflow-tooltip />
        <ElTableColumn label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'network:route:update'" link type="primary" size="small" @click="openForm(row)">
              编辑
            </ElButton>
            <ElButton v-auth="'network:route:delete'" link type="danger" size="small" @click="onDelete(row)">
              删除
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElCard v-if="results.length" class="mt-3" shadow="never" header="下发结果">
        <div v-for="(r, i) in results" :key="i" class="text-sm mb-1">
          <ElTag :type="r.ok ? 'success' : 'danger'" size="small">{{ r.ok ? '成功' : '失败' }}</ElTag>
          <span class="ml-2">{{ r.item }}</span>
          <span class="ml-2 text-g-400">{{ r.message }}</span>
        </div>
      </ElCard>
    </ElCard>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑静态路由' : '新增静态路由'" width="520px">
      <ElForm :model="form" label-width="96px">
        <ElFormItem label="目标网段" required>
          <ElInput v-model="form.destination" placeholder="10.0.0.0/8 或 192.168.1.0/24" />
        </ElFormItem>
        <ElFormItem label="网关">
          <ElInput v-model="form.gateway" placeholder="192.168.1.1（直连路由可留空）" />
        </ElFormItem>
        <ElFormItem label="出口网卡">
          <ElInput v-model="form.device" placeholder="eth0（可选）" />
        </ElFormItem>
        <ElFormItem label="Metric">
          <ElInputNumber v-model="form.metric" :min="0" :max="65535" />
        </ElFormItem>
        <ElFormItem label="路由表">
          <ElInput v-model="form.table" placeholder="main" />
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
  import { applyStaticRoutes, deleteStaticRoute, fetchStaticRoutes, saveStaticRoute, type ApplyResult } from '@/api/platform'

  defineOptions({ name: 'NetworkRoute' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const results = ref<ApplyResult[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const emptyForm = () => ({
    id: '',
    destination: '',
    gateway: '',
    device: '',
    metric: 0,
    table: 'main',
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchStaticRoutes()) as any[]
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
      await saveStaticRoute({ ...form })
      formVisible.value = false
      loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除路由 ${row.destination} ？`, '删除确认', { type: 'warning' })
    await deleteStaticRoute(row.id)
    loadData()
  }

  const onApply = async () => {
    const res = (await applyStaticRoutes()) as any
    results.value = res?.results || []
    loadData()
  }

  onMounted(loadData)
</script>

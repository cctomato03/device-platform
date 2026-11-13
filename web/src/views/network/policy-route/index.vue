<!-- 策略路由：ip rule + 路由表（先选表，再由静态路由页填表内路由） -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="16">
        <ElCard class="art-table-card">
          <template #header>
            <div class="flex-cb">
              <span>策略路由（优先级 {{ 10000 }} - {{ 10999 }} 为平台独占区间）</span>
              <ElSpace>
                <ElButton v-auth="'network:policyroute:apply'" size="small" type="warning" @click="onApply">
                  下发全部
                </ElButton>
                <ElButton v-auth="'network:policyroute:create'" size="small" type="primary" @click="openForm()">
                  新增规则
                </ElButton>
                <ElButton size="small" @click="loadData">刷新</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElTable v-loading="loading" :data="rows" size="small">
            <ElTableColumn prop="priority" label="优先级" width="90" />
            <ElTableColumn prop="name" label="名称" min-width="130" />
            <ElTableColumn prop="fromAddr" label="源地址" width="150" />
            <ElTableColumn prop="toAddr" label="目的地址" width="150" />
            <ElTableColumn prop="fwmark" label="fwmark" width="90" />
            <ElTableColumn label="接口" width="130">
              <template #default="{ row }">
                {{ [row.iif && 'in:' + row.iif, row.oif && 'out:' + row.oif].filter(Boolean).join(' ') || '-' }}
              </template>
            </ElTableColumn>
            <ElTableColumn prop="table" label="路由表" width="100" />
            <ElTableColumn label="启用" width="80">
              <template #default="{ row }">
                <ElTag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <ElButton v-auth="'network:policyroute:update'" link type="primary" size="small" @click="openForm(row)">
                  编辑
                </ElButton>
                <ElButton v-auth="'network:policyroute:delete'" link type="danger" size="small" @click="onDelete(row)">
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

      <ElCol :xs="24" :md="8">
        <ElCard>
          <template #header><span>内核当前规则</span></template>
          <pre class="code-box">{{ (kernelRules || []).join('\n') || '（非 Linux 环境或未安装 iproute2）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑策略路由' : '新增策略路由'" width="560px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="名称" required>
          <ElInput v-model="form.name" placeholder="研发网走专线" />
        </ElFormItem>
        <ElFormItem label="优先级" required>
          <ElInputNumber v-model="form.priority" :min="10000" :max="10999" />
        </ElFormItem>
        <ElFormItem label="源地址">
          <ElInput v-model="form.fromAddr" placeholder="192.168.10.0/24（留空表示任意）" />
        </ElFormItem>
        <ElFormItem label="目的地址">
          <ElInput v-model="form.toAddr" placeholder="10.0.0.0/8（留空表示任意）" />
        </ElFormItem>
        <ElFormItem label="fwmark">
          <ElInputNumber v-model="form.fwmark" :min="0" :max="4294967295" />
        </ElFormItem>
        <ElFormItem label="入接口">
          <ElInput v-model="form.iif" placeholder="eth0（可选）" />
        </ElFormItem>
        <ElFormItem label="出接口">
          <ElInput v-model="form.oif" placeholder="eth1（可选）" />
        </ElFormItem>
        <ElFormItem label="路由表" required>
          <ElInput v-model="form.table" placeholder="100 或 mgmt（需在静态路由页为它配路由）" />
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
    applyPolicyRoutes,
    deletePolicyRoute,
    fetchPolicyRoutes,
    savePolicyRoute,
    type ApplyResult
  } from '@/api/ops'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkPolicyRoute' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const kernelRules = ref<string[]>([])
  const results = ref<ApplyResult[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const emptyForm = () => ({
    id: '',
    name: '',
    priority: 10000,
    fromAddr: '',
    toAddr: '',
    fwmark: 0,
    iif: '',
    oif: '',
    table: '100',
    enabled: true,
    remark: ''
  })

  const loadData = async () => {
    loading.value = true
    try {
      const res = (await fetchPolicyRoutes()) as any
      rows.value = res?.list || []
      kernelRules.value = res?.kernelRules || []
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
      await savePolicyRoute({ ...form })
      formVisible.value = false
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除策略「${row.name}」？`, '删除确认', { type: 'warning' })
    await deletePolicyRoute(row.id)
    await loadData()
  }

  const onApply = async () => {
    const res = (await applyPolicyRoutes()) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 420px;
    padding: 10px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

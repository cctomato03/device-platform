<!-- 网桥与端口聚合：Linux bridge / bond（netplan 下发） -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>网桥与端口聚合</span>
          <ElSpace>
            <ElButton v-auth="'network:bridge:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
            <ElButton v-auth="'network:bridge:create'" size="small" type="primary" @click="openForm()">
              新增网桥/聚合
            </ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        成员网卡由平台自动置为「无地址」；聚合模式中 <code>802.3ad</code>（LACP）需要交换机侧同时配置聚合组，
        现场不确定时先用 <code>active-backup</code>。
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="name" label="名称" width="120" />
        <ElTableColumn label="类型" width="100">
          <template #default="{ row }">
            <ElTag size="small" :type="row.kind === 'bond' ? 'warning' : 'success'">
              {{ row.kind === 'bond' ? '端口聚合' : '网桥' }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="members" label="成员网卡" min-width="160" />
        <ElTableColumn label="模式/STP" width="150">
          <template #default="{ row }">
            {{ row.kind === 'bond' ? bondLabel(row.bondMode) : row.stp ? 'STP 开启' : 'STP 关闭' }}
          </template>
        </ElTableColumn>
        <ElTableColumn label="地址" min-width="170">
          <template #default="{ row }">{{ row.dhcp4 ? 'DHCP' : row.addresses || '无地址' }}</template>
        </ElTableColumn>
        <ElTableColumn label="已下发" width="90">
          <template #default="{ row }">
            <ElTag size="small" :type="row.applied ? 'success' : 'warning'">{{ row.applied ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'network:bridge:update'" link type="primary" size="small" @click="openForm(row)">
              编辑
            </ElButton>
            <ElButton v-auth="'network:bridge:delete'" link type="danger" size="small" @click="onDelete(row)">
              删除
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElCard v-if="results.length" class="mt-3" shadow="never" header="下发结果">
        <ResultList :results="results" />
      </ElCard>
    </ElCard>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑网桥/聚合' : '新增网桥/聚合'" width="560px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="类型">
          <ElRadioGroup v-model="form.kind">
            <ElRadioButton value="bridge">网桥（bridge）</ElRadioButton>
            <ElRadioButton value="bond">端口聚合（bond）</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="名称" required>
          <ElInput v-model="form.name" :placeholder="form.kind === 'bond' ? 'bond0' : 'br0'" />
        </ElFormItem>
        <ElFormItem label="成员网卡" required>
          <ElSelect v-model="memberList" multiple filterable allow-create style="width: 100%">
            <ElOption v-for="i in ifaces" :key="i.name" :label="`${i.name}${i.up ? '' : '（DOWN）'}`" :value="i.name" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="form.kind === 'bond'" label="聚合模式">
          <ElSelect v-model="form.bondMode" style="width: 100%">
            <ElOption v-for="(desc, mode) in meta?.bondModes || {}" :key="mode" :label="`${mode} · ${desc}`" :value="mode" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="form.kind === 'bridge'" label="生成树 STP">
          <ElSwitch v-model="form.stp" />
        </ElFormItem>
        <ElFormItem label="地址方式">
          <ElRadioGroup v-model="form.dhcp4">
            <ElRadioButton :value="true">DHCP</ElRadioButton>
            <ElRadioButton :value="false">静态</ElRadioButton>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem v-if="!form.dhcp4" label="地址">
          <ElInput v-model="form.addresses" placeholder="192.168.1.10/24（可留空，稍后在网卡配置页设置）" />
        </ElFormItem>
        <ElFormItem v-if="!form.dhcp4" label="网关">
          <ElInput v-model="form.gateway" placeholder="192.168.1.1" />
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
    saveIfaceConfig,
    type ApplyResult
  } from '@/api/ops'
  import { fetchInterfaces } from '@/api/platform'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkBridge' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const ifaces = ref<any[]>([])
  const meta = ref<any>(null)
  const results = ref<ApplyResult[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})
  const memberList = ref<string[]>([])

  const bondLabel = (mode: string) => mode || 'active-backup'

  const emptyForm = () => ({
    id: '',
    name: '',
    kind: 'bridge',
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
      const all = (await fetchIfaceConfigs()) as any[]
      rows.value = all.filter((r) => r.kind === 'bridge' || r.kind === 'bond')
      ifaces.value = (await fetchInterfaces()) as any[]
      meta.value = await fetchIfaceMeta()
    } finally {
      loading.value = false
    }
  }

  const openForm = (row?: any) => {
    Object.keys(form).forEach((k) => delete form[k])
    Object.assign(form, emptyForm(), row ? { ...row } : {})
    memberList.value = (form.members || '').split(',').filter(Boolean)
    formVisible.value = true
  }

  const onSave = async () => {
    form.members = memberList.value.join(',')
    saving.value = true
    try {
      await saveIfaceConfig({ ...form })
      formVisible.value = false
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除「${row.name}」？删除后成员网卡会恢复为独立网卡（需点下发生效）。`, '删除确认', {
      type: 'warning'
    })
    await deleteIfaceConfig(row.id)
    await loadData()
  }

  const onApply = async () => {
    const res = (await applyIfaceConfigs()) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(loadData)
</script>

<!-- 运行告警：阈值规则配置 + 告警记录 -->
<template>
  <div class="art-full-height">
    <ElTabs v-model="tab">
      <ElTabPane label="告警规则" name="rule">
        <ElCard class="art-table-card">
          <template #header>
            <div class="flex-cb">
              <span>阈值规则（CPU / 内存 / 磁盘 / 网络）</span>
              <ElSpace>
                <ElButton v-auth="'device:alert:create'" size="small" type="primary" @click="openForm()">新增规则</ElButton>
                <ElButton size="small" @click="loadRules">刷新</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElTable v-loading="loading" :data="rules" size="small">
            <ElTableColumn prop="name" label="规则名称" min-width="140" />
            <ElTableColumn label="指标" width="130">
              <template #default="{ row }">{{ metricLabel(row.metric) }}</template>
            </ElTableColumn>
            <ElTableColumn label="条件" width="150">
              <template #default="{ row }">{{ row.operator }} {{ row.threshold }}</template>
            </ElTableColumn>
            <ElTableColumn label="持续时间" width="110">
              <template #default="{ row }">{{ row.durationSec }} 秒</template>
            </ElTableColumn>
            <ElTableColumn label="级别" width="100">
              <template #default="{ row }">
                <ElTag :type="levelType(row.level)" size="small">{{ levelLabel(row.level) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="静默" width="100">
              <template #default="{ row }">{{ row.silenceSec }} 秒</template>
            </ElTableColumn>
            <ElTableColumn label="启用" width="80">
              <template #default="{ row }">
                <ElTag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="130" fixed="right">
              <template #default="{ row }">
                <ElButton v-auth="'device:alert:update'" link type="primary" size="small" @click="openForm(row)">编辑</ElButton>
                <ElButton v-auth="'device:alert:delete'" link type="danger" size="small" @click="onDelete(row)">删除</ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElCard>
      </ElTabPane>

      <ElTabPane label="告警记录" name="record">
        <ElCard class="art-table-card">
          <template #header>
            <div class="flex-cb">
              <span>告警记录</span>
              <ElSpace>
                <ElSelect v-model="recordQuery.status" size="small" style="width: 130px" @change="loadRecords">
                  <ElOption label="全部" :value="''" />
                  <ElOption label="触发中" :value="1" />
                  <ElOption label="已恢复" :value="2" />
                  <ElOption label="已确认" :value="3" />
                </ElSelect>
                <ElButton size="small" @click="loadRecords">刷新</ElButton>
              </ElSpace>
            </div>
          </template>

          <ElTable v-loading="recordLoading" :data="records" size="small">
            <ElTableColumn prop="firedAt" label="触发时间" width="180" />
            <ElTableColumn prop="ruleName" label="规则" min-width="140" />
            <ElTableColumn label="指标" width="120">
              <template #default="{ row }">{{ metricLabel(row.metric) }}</template>
            </ElTableColumn>
            <ElTableColumn label="级别" width="90">
              <template #default="{ row }">
                <ElTag :type="levelType(row.level)" size="small">{{ levelLabel(row.level) }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="message" label="内容" min-width="220" show-overflow-tooltip />
            <ElTableColumn label="状态" width="100">
              <template #default="{ row }">
                <ElTag :type="row.status === 1 ? 'warning' : row.status === 2 ? 'success' : 'info'" size="small">
                  {{ row.status === 1 ? '触发中' : row.status === 2 ? '已恢复' : '已确认' }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn prop="recoverAt" label="恢复时间" width="180" />
            <ElTableColumn label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <ElButton
                  v-if="row.status === 1"
                  v-auth="'device:alert:update'"
                  link
                  type="primary"
                  size="small"
                  @click="onAck(row)"
                >
                  确认
                </ElButton>
              </template>
            </ElTableColumn>
          </ElTable>

          <div class="mt-3 flex justify-end">
            <ElPagination
              v-model:current-page="recordQuery.current"
              v-model:page-size="recordQuery.size"
              :total="recordTotal"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next"
              @size-change="loadRecords"
              @current-change="loadRecords"
            />
          </div>
        </ElCard>
      </ElTabPane>
    </ElTabs>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑告警规则' : '新增告警规则'" width="540px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="规则名称" required>
          <ElInput v-model="form.name" placeholder="CPU 使用率过高" />
        </ElFormItem>
        <ElFormItem label="监控指标" required>
          <ElSelect v-model="form.metric" style="width: 100%">
            <ElOption label="CPU 使用率(%)" value="cpu" />
            <ElOption label="内存使用率(%)" value="mem" />
            <ElOption label="磁盘使用率(%)" value="disk" />
            <ElOption label="入向带宽(MB/s)" value="net_in" />
            <ElOption label="出向带宽(MB/s)" value="net_out" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="比较方式">
          <ElSelect v-model="form.operator" style="width: 100%">
            <ElOption label="大于 >" value=">" />
            <ElOption label="大于等于 >=" value=">=" />
            <ElOption label="小于 <" value="<" />
            <ElOption label="小于等于 <=" value="<=" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="阈值">
          <ElInputNumber v-model="form.threshold" :min="0" :max="100000" />
        </ElFormItem>
        <ElFormItem label="持续(秒)">
          <ElInputNumber v-model="form.durationSec" :min="10" :max="86400" />
        </ElFormItem>
        <ElFormItem label="级别">
          <ElSelect v-model="form.level" style="width: 100%">
            <ElOption label="提示" :value="1" />
            <ElOption label="警告" :value="2" />
            <ElOption label="严重" :value="3" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="静默(秒)">
          <ElInputNumber v-model="form.silenceSec" :min="0" :max="86400" />
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
  import { ackAlertRecord, deleteAlertRule, fetchAlertRecords, fetchAlertRules, saveAlertRule } from '@/api/platform'

  defineOptions({ name: 'DeviceAlert' })

  const tab = ref('rule')
  const loading = ref(false)
  const recordLoading = ref(false)
  const saving = ref(false)
  const rules = ref<any[]>([])
  const records = ref<any[]>([])
  const recordTotal = ref(0)
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})
  const recordQuery = reactive<Record<string, any>>({ current: 1, size: 10, status: '' })

  const metricLabel = (m: string) =>
    ({ cpu: 'CPU', mem: '内存', disk: '磁盘', net_in: '入向带宽', net_out: '出向带宽' })[m] || m

  const levelLabel = (l: number) => ({ 1: '提示', 2: '警告', 3: '严重' })[l] || '提示'

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
  const LEVEL_TYPES: Record<number, TagType> = { 1: 'info', 2: 'warning', 3: 'danger' }
  const levelType = (l: number): TagType => LEVEL_TYPES[l] || 'info'

  const emptyForm = () => ({
    id: '',
    name: '',
    metric: 'cpu',
    operator: '>',
    threshold: 85,
    durationSec: 60,
    level: 2,
    silenceSec: 600,
    enabled: true,
    remark: ''
  })

  const loadRules = async () => {
    loading.value = true
    try {
      rules.value = (await fetchAlertRules()) as any[]
    } finally {
      loading.value = false
    }
  }

  const loadRecords = async () => {
    recordLoading.value = true
    try {
      const res = (await fetchAlertRecords({ ...recordQuery })) as any
      records.value = res?.list || []
      recordTotal.value = res?.total || 0
    } finally {
      recordLoading.value = false
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
      await saveAlertRule({ ...form })
      formVisible.value = false
      loadRules()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除规则「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteAlertRule(row.id)
    loadRules()
  }

  const onAck = async (row: any) => {
    await ackAlertRecord(row.id)
    loadRecords()
  }

  watch(tab, (v) => (v === 'record' ? loadRecords() : loadRules()))

  onMounted(loadRules)
</script>

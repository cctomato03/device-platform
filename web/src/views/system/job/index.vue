<!-- 定时任务（内置处理器 + HTTP 回调，支持手动执行与执行日志） -->
<template>
  <div class="job-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:job:create'" @click="openDialog('add', null)" v-ripple>
              新增任务
            </ElButton>
            <ElTag type="info">支持 6 段 cron（秒 分 时 日 月 周）</ElTag>
          </ElSpace>
        </template>
        <template #right>
          <ElButton @click="logsVisible = true">执行日志</ElButton>
        </template>
      </ArtTableHeader>

      <ArtTable :loading="loading" :columns="columns" :data="tableData" />
    </ElCard>

    <!-- 任务弹窗 -->
    <ElDialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增任务' : '编辑任务'"
      width="620px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
        <ElFormItem label="任务名称" prop="name">
          <ElInput v-model.trim="form.name" placeholder="如：清理过期登录会话" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="任务类型">
          <ElRadioGroup v-model="form.jobType" @change="onJobTypeChange">
            <ElRadio v-for="t in metaTypes" :key="t.value" :value="t.value" :disabled="!t.enabled">
              {{ t.label }}
              <ElTooltip v-if="!t.enabled" :content="`${t.description}（需在配置文件打开 scheduler.shell_enabled）`">
                <ArtSvgIcon icon="ri:question-line" class="ml-0.5 text-g-400" />
              </ElTooltip>
            </ElRadio>
          </ElRadioGroup>
          <div class="type-desc">{{ currentTypeDesc }}</div>
        </ElFormItem>

        <!-- 类型 1：内置处理器 -->
        <ElFormItem v-if="form.jobType === 1" label="处理器" prop="handler">
          <ElSelect v-model="form.handler" placeholder="选择内置处理器" filterable>
            <ElOption
              v-for="h in metaHandlers"
              :key="h.value"
              :label="`${h.label}（${h.value}）`"
              :value="h.value"
            >
              <span>{{ h.label }}</span>
              <span class="ml-2 text-xs text-g-400">{{ h.description }}</span>
            </ElOption>
          </ElSelect>
        </ElFormItem>

        <!-- 类型 2：HTTP 调用 -->
        <template v-else-if="form.jobType === 2">
          <ElFormItem label="请求方法">
            <ElSelect v-model="form.httpMethod">
              <ElOption label="POST" value="POST" />
              <ElOption label="GET" value="GET" />
              <ElOption label="PUT" value="PUT" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="回调地址" prop="target">
            <ElInput v-model.trim="form.target" placeholder="https://example.com/api/hook" />
          </ElFormItem>
        </template>

        <!-- 类型 3：Shell / Python 脚本（自定义执行内容，脚本存库，改完即生效） -->
        <template v-else>
          <ElFormItem label="解释器">
            <ElSelect v-model="shell.interpreter">
              <ElOption v-for="i in interpreters" :key="i" :label="i" :value="i" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="脚本内容">
            <ElInput
              v-model="shell.script"
              type="textarea"
              :rows="8"
              placeholder="# 例：清理 7 天前的备份 / 调用外部脚本&#10;find /data/backup -type f -mtime +7 -delete&#10;echo done"
              class="font-mono"
            />
            <div class="type-desc">
              脚本在服务进程所在机器执行；超时按上方「秒超时」强制结束，输出会写入执行日志。
            </div>
          </ElFormItem>
        </template>
        <ElFormItem label="cron 表达式" prop="cron">
          <ElInput v-model.trim="form.cron" placeholder="0 */10 * * * *（每 10 分钟）" />
        </ElFormItem>
        <ElFormItem label="参数(JSON)">
          <ElInput
            v-model="form.params"
            type="textarea"
            :rows="2"
            placeholder='{"days":7}'
          />
        </ElFormItem>
        <ElFormItem label="超时/重试">
          <ElSpace>
            <ElInputNumber v-model="form.timeoutSec" :min="1" :max="3600" />
            <span class="text-xs text-g-500">秒超时</span>
            <ElInputNumber v-model="form.retryTimes" :min="0" :max="5" />
            <span class="text-xs text-g-500">次重试</span>
          </ElSpace>
        </ElFormItem>
        <ElFormItem label="状态">
          <ElRadioGroup v-model="form.status">
            <ElRadio :value="1">启用</ElRadio>
            <ElRadio :value="2">停用</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="form.remark" type="textarea" :rows="2" maxlength="200" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 执行日志 -->
    <ElDrawer v-model="logsVisible" title="任务执行日志" size="760px" @open="loadLogs">
      <ArtTable
        :loading="logsLoading"
        :columns="logColumns"
        :data="logData"
        :pagination="logsPagination"
        @pagination:size-change="handleLogSizeChange"
        @pagination:current-change="handleLogCurrentChange"
      />
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import { useTable } from '@/hooks/core/useTable'
  import {
    fetchGetJobList,
    fetchGetJobMeta,
    fetchCreateJob,
    fetchUpdateJob,
    fetchSetJobStatus,
    fetchRunJob,
    fetchDeleteJob,
    fetchGetJobLogs
  } from '@/api/business'
  import {
    ElTag,
    ElSwitch,
    ElMessageBox,
    ElMessage,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'SystemJob' })

  const loading = ref(false)
  const tableData = ref<WebBasic.Job[]>([])
  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editingId = ref<string | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  // 任务元信息由后端下发（类型 / 内置处理器 / 是否允许 Shell 脚本），前端不再写死
  const metaTypes = ref<WebBasic.JobTypeOption[]>([])
  const metaHandlers = ref<WebBasic.JobHandlerOption[]>([])
  const interpreters = ref<string[]>(['sh', 'bash', 'python3'])
  // Shell 任务的执行内容（保存时序列化进 params）
  const shell = reactive({ interpreter: 'sh', script: '' })

  const form = reactive({
    name: '',
    jobType: 1,
    handler: 'cleanSession',
    target: '',
    httpMethod: 'POST',
    cron: '0 */10 * * * *',
    params: '{}',
    timeoutSec: 60,
    retryTimes: 0,
    concurrent: 1,
    status: 2,
    remark: ''
  })

  /** 当前任务类型的说明文字（来自后端 meta） */
  const currentTypeDesc = computed(
    () => metaTypes.value.find((t) => t.value === form.jobType)?.description || ''
  )

  const rules: FormRules = {
    name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
    cron: [{ required: true, message: '请输入 cron 表达式', trigger: 'blur' }],
    handler: [{ required: true, message: '请选择处理器', trigger: 'change' }],
    target: [{ required: true, message: '请输入回调地址', trigger: 'blur' }]
  }

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetJobList()
    } finally {
      loading.value = false
    }
  }

  /** 读取任务元信息；失败时退回仅「内置处理器 + HTTP」两种类型 */
  const loadMeta = async () => {
    try {
      const meta = await fetchGetJobMeta()
      metaTypes.value = meta.types || []
      metaHandlers.value = meta.handlers || []
      if (meta.interpreters?.length) interpreters.value = meta.interpreters
    } catch {
      metaTypes.value = [
        { value: 1, label: '内置处理器', description: '', enabled: true },
        { value: 2, label: 'HTTP 调用', description: '', enabled: true }
      ]
    }
  }

  onMounted(() => {
    loadData()
    loadMeta()
  })

  const onJobTypeChange = () => {
    formRef.value?.clearValidate()
  }

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'name', label: '任务名称', minWidth: 170 },
    {
      prop: 'handler',
      label: '执行方式',
      minWidth: 200,
      formatter: (row: WebBasic.Job) => {
        if (row.jobType === 2) return `${row.httpMethod} ${row.target}`
        if (row.jobType === 3) {
          try {
            const p = JSON.parse(row.params || '{}')
            const first = String(p.script || '').split('\n')[0] || ''
            return `${p.interpreter || 'sh'}: ${first}`
          } catch {
            return 'Shell 脚本'
          }
        }
        return `内置：${row.handler}`
      }
    },
    {
      prop: 'cron',
      label: 'cron',
      width: 160,
      formatter: (row: WebBasic.Job) => h('code', {}, row.cron)
    },
    { prop: 'params', label: '参数', minWidth: 140, showOverflowTooltip: true },
    {
      prop: 'status',
      label: '状态',
      width: 90,
      formatter: (row: WebBasic.Job) =>
        h(ElSwitch, {
          modelValue: row.status === 1,
          'onUpdate:modelValue': (val: any) => toggleStatus(row, val)
        })
    },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.Job) =>
        h(ArtButtonMore, {
          list: [
            { key: 'run', label: '立即执行一次', icon: 'ri:play-line', auth: 'system:job:run' },
            { key: 'edit', label: '编辑任务', icon: 'ri:edit-2-line', auth: 'system:job:update' },
            {
              key: 'delete',
              label: '删除任务',
              icon: 'ri:delete-bin-4-line',
              color: '#f56c6c',
              auth: 'system:job:delete'
            }
          ] as ButtonMoreItem[],
          onClick: (item: ButtonMoreItem) => {
            if (item.key === 'run') runJob(row)
            else if (item.key === 'edit') openDialog('edit', row)
            else handleDelete(row)
          }
        })
    }
  ])

  const toggleStatus = async (row: WebBasic.Job, val: boolean) => {
    const status = val ? 1 : 2
    await fetchSetJobStatus(row.id, status)
    row.status = status
  }

  const runJob = async (row: WebBasic.Job) => {
    try {
      await ElMessageBox.confirm(`确定立即执行任务「${row.name}」吗？`, '手动执行', {
        confirmButtonText: '执行',
        cancelButtonText: '取消',
        type: 'info'
      })
    } catch {
      return
    }
    await fetchRunJob(row.id)
  }

  const openDialog = (type: 'add' | 'edit', row: WebBasic.Job | null) => {
    dialogType.value = type
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    // Shell 任务：把 params 里的脚本回填到表单
    shell.interpreter = 'sh'
    shell.script = ''
    if (row?.jobType === 3) {
      try {
        const p = JSON.parse(row.params || '{}')
        shell.interpreter = p.interpreter || 'sh'
        shell.script = p.script || ''
      } catch {
        /* 参数不是合法 JSON 时保持空脚本，用户重新填写 */
      }
    }
    Object.assign(form, {
      name: row?.name ?? '',
      jobType: row?.jobType ?? 1,
      handler: row?.handler || 'cleanSession',
      target: row?.target ?? '',
      httpMethod: row?.httpMethod || 'POST',
      cron: row?.cron || '0 */10 * * * *',
      params: row?.params || '{}',
      timeoutSec: row?.timeoutSec ?? 60,
      retryTimes: row?.retryTimes ?? 0,
      concurrent: row?.concurrent ?? 1,
      status: row?.status ?? 2,
      remark: row?.remark ?? ''
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    if (!formRef.value) return
    const valid = await formRef.value.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      // Shell 任务：脚本与解释器打包进 params，后端直接执行
      const payload = { ...form }
      if (form.jobType === 3) {
        if (!shell.script.trim()) {
          ElMessage.warning('请填写脚本内容')
          submitting.value = false
          return
        }
        payload.params = JSON.stringify({ interpreter: shell.interpreter, script: shell.script })
      }
      if (dialogType.value === 'add') {
        await fetchCreateJob(payload)
      } else if (editingId.value) {
        await fetchUpdateJob(editingId.value, payload)
      }
      dialogVisible.value = false
      loadData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: WebBasic.Job) => {
    try {
      await ElMessageBox.confirm(`确定删除任务「${row.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteJob(row.id)
    loadData()
  }

  /* ------------- 执行日志 ------------- */
  const logsVisible = ref(false)

  const {
    columns: logColumns,
    data: logData,
    loading: logsLoading,
    pagination: logsPagination,
    getData: loadLogs,
    handleSizeChange: handleLogSizeChange,
    handleCurrentChange: handleLogCurrentChange
  } = useTable({
    core: {
      immediate: false,
      apiFn: fetchGetJobLogs,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { prop: 'jobName', label: '任务', minWidth: 140 },
        {
          prop: 'triggerType',
          label: '触发',
          width: 90,
          formatter: (row: WebBasic.JobLog) =>
            ['-', '手动', '定时', '启动'][row.triggerType] || '-'
        },
        {
          prop: 'status',
          label: '结果',
          width: 90,
          formatter: (row: WebBasic.JobLog) =>
            h(
              ElTag,
              { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
              () => (row.status === 1 ? '成功' : '失败')
            )
        },
        { prop: 'costMs', label: '耗时', width: 90, formatter: (row: WebBasic.JobLog) => `${row.costMs} ms` },
        { prop: 'result', label: '结果摘要', minWidth: 220, showOverflowTooltip: true },
        { prop: 'startTime', label: '执行时间', width: 180, formatter: (row) => formatTime(row.startTime) }
      ]
    }
  })
</script>

<style scoped>
.type-desc {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--art-gray-600);
}
</style>

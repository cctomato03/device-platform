<!-- 通知公告（发布后自动给本租户所有启用用户发送站内信） -->
<template>
  <div class="notice-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:notice:create'" type="primary" @click="openDialog(null)" v-ripple>
              新增公告
            </ElButton>
            <ElInput
              v-model="keyword"
              placeholder="搜索标题"
              clearable
              class="!w-52"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            />
            <ElSelect v-model="statusFilter" placeholder="状态" clearable class="!w-32" @change="handleSearch">
              <ElOption label="草稿" :value="0" />
              <ElOption label="已发布" :value="1" />
              <ElOption label="已关闭" :value="2" />
            </ElSelect>
            <ElButton @click="handleSearch">查询</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable
        :loading="loading"
        :columns="columns"
        :data="data"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="editingId ? '编辑公告' : '新增公告'"
      width="680px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="80px">
        <ElFormItem label="标题" prop="title">
          <ElInput v-model.trim="form.title" maxlength="200" show-word-limit placeholder="请输入公告标题" />
        </ElFormItem>
        <ElFormItem label="类型">
          <ElRadioGroup v-model="form.type">
            <ElRadio :value="1">通知</ElRadio>
            <ElRadio :value="2">公告</ElRadio>
            <ElRadio :value="3">维护</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="级别">
          <ElRadioGroup v-model="form.level">
            <ElRadio :value="1">普通</ElRadio>
            <ElRadio :value="2">重要</ElRadio>
            <ElRadio :value="3">紧急</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="正文" prop="content">
          <ElInput
            v-model="form.content"
            type="textarea"
            :rows="8"
            maxlength="20000"
            show-word-limit
            placeholder="支持纯文本 / Markdown"
          />
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="form.remark" type="textarea" :rows="2" maxlength="500" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import {
    fetchCreateNotice,
    fetchDeleteNotice,
    fetchGetNoticeList,
    fetchPublishNotice,
    fetchUpdateNotice
  } from '@/api/notice'
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'Notice' })

  const keyword = ref('')
  const statusFilter = ref<number | undefined>(undefined)

  const dialogVisible = ref(false)
  const submitting = ref(false)
  const editingId = ref<string | null>(null)
  const formRef = ref<FormInstance>()

  const form = reactive({ title: '', type: 1, level: 1, content: '', remark: '' })

  const rules: FormRules = {
    title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
    content: [{ required: true, message: '请输入正文', trigger: 'blur' }]
  }

  const typeLabels: Record<number, string> = { 1: '通知', 2: '公告', 3: '维护' }
  const levelLabels: Record<number, string> = { 1: '普通', 2: '重要', 3: '紧急' }
  const statusMeta: Record<number, { text: string; type: 'info' | 'success' | 'warning' }> = {
    0: { text: '草稿', type: 'info' },
    1: { text: '已发布', type: 'success' },
    2: { text: '已关闭', type: 'warning' }
  }

  const {
    columns,
    columnChecks,
    data,
    loading,
    pagination,
    getData,
    replaceSearchParams,
    handleSizeChange,
    handleCurrentChange,
    refreshData
  } = useTable({
    core: {
      apiFn: fetchGetNoticeList,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { prop: 'title', label: '标题', minWidth: 220, showOverflowTooltip: true },
        {
          prop: 'type',
          label: '类型',
          width: 90,
          formatter: (row: WebBasic.Notice) => typeLabels[row.type] ?? '-'
        },
        {
          prop: 'level',
          label: '级别',
          width: 90,
          formatter: (row: WebBasic.Notice) =>
            h(
              ElTag,
              {
                type: row.level === 3 ? 'danger' : row.level === 2 ? 'warning' : 'info',
                size: 'small'
              },
              () => levelLabels[row.level] ?? '-'
            )
        },
        {
          prop: 'status',
          label: '状态',
          width: 100,
          formatter: (row: WebBasic.Notice) => {
            const meta = statusMeta[row.status] ?? { text: '-', type: 'info' as const }
            return h(ElTag, { type: meta.type, size: 'small' }, () => meta.text)
          }
        },
        { prop: 'publisher', label: '发布人', width: 130 },
        {
          prop: 'publishTime',
          label: '发布时间',
          width: 180,
          formatter: (row: WebBasic.Notice) => (row.publishTime ? formatTime(row.publishTime) : '-')
        },
        {
          prop: 'operation',
          label: '操作',
          width: 80,
          fixed: 'right',
          formatter: (row: WebBasic.Notice) =>
            h(ArtButtonMore, {
              list: noticeActions(row),
              onClick: (item: ButtonMoreItem) => {
                if (item.key === 'publish') handlePublish(row)
                else if (item.key === 'edit') openDialog(row)
                else handleDelete(row)
              }
            })
        }
      ]
    }
  })

  /** 行内操作按钮（按公告状态动态组装，避免"已发布还能再发布"） */
  const noticeActions = (row: WebBasic.Notice): ButtonMoreItem[] => {
    const items: ButtonMoreItem[] = []
    if (row.status !== 1) {
      items.push({ key: 'publish', label: '发布公告', icon: 'ri:send-plane-line', auth: 'system:notice:publish' })
    }
    if (row.status !== 2) {
      items.push({ key: 'edit', label: '编辑公告', icon: 'ri:edit-2-line', auth: 'system:notice:update' })
    }
    items.push({
      key: 'delete',
      label: '删除公告',
      icon: 'ri:delete-bin-4-line',
      color: '#f56c6c',
      auth: 'system:notice:delete'
    })
    return items
  }

  const handleSearch = () => {
    replaceSearchParams({
      keyword: keyword.value || undefined,
      status: statusFilter.value === undefined ? undefined : statusFilter.value
    })
    getData()
  }

  const openDialog = (row: WebBasic.Notice | null) => {
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    Object.assign(form, {
      title: row?.title ?? '',
      type: row?.type ?? 1,
      level: row?.level ?? 1,
      content: row?.content ?? '',
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
      if (editingId.value) await fetchUpdateNotice(editingId.value, { ...form })
      else await fetchCreateNotice({ ...form })
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handlePublish = async (row: WebBasic.Notice) => {
    try {
      await ElMessageBox.confirm(`发布后会给本租户所有启用用户发送站内信，确定发布「${row.title}」吗？`, '发布确认', {
        confirmButtonText: '确定发布',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    const res = await fetchPublishNotice(row.id)
    ElMessage.success(`发布成功，已推送 ${res?.pushed ?? 0} 条站内信`)
    refreshData()
  }

  const handleDelete = async (row: WebBasic.Notice) => {
    try {
      await ElMessageBox.confirm(`确定删除公告「${row.title}」吗？相关站内信会一并删除。`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteNotice(row.id)
    refreshData()
  }
</script>

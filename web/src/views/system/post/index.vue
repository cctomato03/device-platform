<!-- 岗位管理（web-basic 真实数据） -->
<template>
  <div class="post-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:post:create'" @click="openDialog('add', null)" v-ripple>
              新增岗位
            </ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable :loading="loading" :columns="columns" :data="tableData" />
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增岗位' : '编辑岗位'"
      width="520px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="90px">
        <ElFormItem label="岗位名称" prop="name">
          <ElInput v-model.trim="form.name" placeholder="如：开发工程师" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="岗位编码" prop="code">
          <ElInput v-model.trim="form.code" placeholder="如：dev" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="排序">
          <ElInputNumber v-model="form.sort" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
        <ElFormItem label="状态">
          <ElRadioGroup v-model="form.status">
            <ElRadio :value="1">正常</ElRadio>
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
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import {
    fetchGetPostList,
    fetchCreatePost,
    fetchUpdatePost,
    fetchDeletePost
  } from '@/api/business'
  import { ElTag, ElMessageBox, ElMessage, type FormInstance, type FormRules } from 'element-plus'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'Post' })

  const loading = ref(false)
  const tableData = ref<WebBasic.Post[]>([])
  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editingId = ref<string | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  const form = reactive({ name: '', code: '', sort: 0, status: 1, remark: '' })

  const rules: FormRules = {
    name: [{ required: true, message: '请输入岗位名称', trigger: 'blur' }],
    code: [{ required: true, message: '请输入岗位编码', trigger: 'blur' }]
  }

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetPostList()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'name', label: '岗位名称', minWidth: 160 },
    { prop: 'code', label: '岗位编码', minWidth: 140 },
    { prop: 'sort', label: '排序', width: 90 },
    {
      prop: 'status',
      label: '状态',
      width: 100,
      formatter: (row: WebBasic.Post) =>
        h(
          ElTag,
          { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
          () => (row.status === 1 ? '正常' : '停用')
        )
    },
    { prop: 'remark', label: '备注', minWidth: 200 },
    { prop: 'createTime', label: '创建时间', width: 180, formatter: (row) => formatTime(row.createTime) },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.Post) =>
        h(ArtButtonMore, {
          list: [
            { key: 'edit', label: '编辑岗位', icon: 'ri:edit-2-line', auth: 'system:post:update' },
            {
              key: 'delete',
              label: '删除岗位',
              icon: 'ri:delete-bin-4-line',
              color: '#f56c6c',
              auth: 'system:post:delete'
            }
          ],
          onClick: (item: ButtonMoreItem) =>
            item.key === 'edit' ? openDialog('edit', row) : handleDelete(row)
        })
    }
  ])

  const openDialog = (type: 'add' | 'edit', row: WebBasic.Post | null) => {
    dialogType.value = type
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    Object.assign(form, {
      name: row?.name ?? '',
      code: row?.code ?? '',
      sort: row?.sort ?? 0,
      status: row?.status ?? 1,
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
      if (dialogType.value === 'add') {
        await fetchCreatePost({ ...form })
      } else if (editingId.value) {
        await fetchUpdatePost(editingId.value, { ...form })
      }
      dialogVisible.value = false
      loadData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: WebBasic.Post) => {
    try {
      await ElMessageBox.confirm(`确定删除岗位「${row.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeletePost(row.id)
    loadData()
  }
</script>
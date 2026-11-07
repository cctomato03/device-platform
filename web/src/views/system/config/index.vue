<!-- 参数配置（web-basic 真实数据） -->
<template>
  <div class="config-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:config:create'" @click="openDialog('add', null)" v-ripple>
              新增参数
            </ElButton>
            <ElInput v-model="keyword" placeholder="搜索参数名称 / 键名" clearable class="!w-56" />
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable :loading="loading" :columns="columns" :data="filteredData" />
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增参数' : '编辑参数'"
      width="560px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
        <ElFormItem label="参数名称" prop="name">
          <ElInput v-model.trim="form.name" placeholder="如：系统名称" maxlength="100" />
        </ElFormItem>
        <ElFormItem label="参数键名" prop="key">
          <ElInput
            v-model.trim="form.key"
            placeholder="如：sys.name"
            :disabled="dialogType === 'edit'"
            maxlength="100"
          />
        </ElFormItem>
        <ElFormItem label="参数值" prop="value">
          <ElInput v-model="form.value" type="textarea" :rows="2" maxlength="1000" />
        </ElFormItem>
        <ElFormItem label="是否公开">
          <ElSwitch v-model="form.isPublic" />
          <span class="ml-2 text-xs text-g-500">公开参数无需登录即可读取（登录页 / 门户使用）</span>
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
    fetchGetConfigList,
    fetchCreateConfig,
    fetchUpdateConfig,
    fetchDeleteConfig
  } from '@/api/business'
  import { ElTag, ElMessageBox, ElMessage, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'Config' })

  const loading = ref(false)
  const tableData = ref<WebBasic.Config[]>([])
  const keyword = ref('')
  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editingId = ref<string | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  const form = reactive({ name: '', key: '', value: '', isPublic: false, remark: '' })

  const rules: FormRules = {
    name: [{ required: true, message: '请输入参数名称', trigger: 'blur' }],
    key: [{ required: true, message: '请输入参数键名', trigger: 'blur' }]
  }

  const filteredData = computed(() => {
    const kw = keyword.value.trim().toLowerCase()
    if (!kw) return tableData.value
    return tableData.value.filter(
      (c) => c.name.toLowerCase().includes(kw) || c.key.toLowerCase().includes(kw)
    )
  })

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetConfigList()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'name', label: '参数名称', minWidth: 170 },
    { prop: 'key', label: '参数键名', minWidth: 180 },
    {
      prop: 'value',
      label: '参数值',
      minWidth: 200,
      showOverflowTooltip: true
    },
    {
      prop: 'isPublic',
      label: '是否公开',
      width: 110,
      formatter: (row: WebBasic.Config) =>
        h(
          ElTag,
          { type: row.isPublic ? 'success' : 'info', size: 'small' },
          () => (row.isPublic ? '公开' : '内部')
        )
    },
    { prop: 'remark', label: '备注', minWidth: 160 },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.Config) =>
        h(ArtButtonMore, {
          list: [
            { key: 'edit', label: '编辑参数', icon: 'ri:edit-2-line', auth: 'system:config:update' },
            {
              key: 'delete',
              label: '删除参数',
              icon: 'ri:delete-bin-4-line',
              color: '#f56c6c',
              auth: 'system:config:delete'
            }
          ],
          onClick: (item: ButtonMoreItem) =>
            item.key === 'edit' ? openDialog('edit', row) : handleDelete(row)
        })
    }
  ])

  const openDialog = (type: 'add' | 'edit', row: WebBasic.Config | null) => {
    dialogType.value = type
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    Object.assign(form, {
      name: row?.name ?? '',
      key: row?.key ?? '',
      value: row?.value ?? '',
      isPublic: row?.isPublic ?? false,
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
        await fetchCreateConfig({ ...form })
      } else if (editingId.value) {
        await fetchUpdateConfig(editingId.value, { ...form })
      }
      dialogVisible.value = false
      loadData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: WebBasic.Config) => {
    try {
      await ElMessageBox.confirm(`确定删除参数「${row.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteConfig(row.id)
    loadData()
  }
</script>
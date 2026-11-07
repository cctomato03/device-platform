<!-- 租户管理（多租户隔离，web-basic 真实数据） -->
<template>
  <div class="tenant-page art-full-height">
    <ElAlert
      class="mb-3"
      type="info"
      :closable="false"
      title="多租户说明：平台级菜单/字典共享，业务数据按 tenant_id 隔离；此处维护租户及其有效期与用户上限。"
    />

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:tenant:create'" @click="openDialog('add', null)" v-ripple>
              新增租户
            </ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable :loading="loading" :columns="columns" :data="tableData" />
    </ElCard>

    <ElDialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增租户' : '编辑租户'"
      width="600px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
        <ElFormItem label="租户名称" prop="name">
          <ElInput v-model.trim="form.name" placeholder="如：某某科技有限公司" maxlength="100" />
        </ElFormItem>
        <ElFormItem label="租户编码" prop="code">
          <ElInput
            v-model.trim="form.code"
            placeholder="如：acme"
            :disabled="dialogType === 'edit'"
            maxlength="64"
          />
        </ElFormItem>
        <ElFormItem label="联系人">
          <ElInput v-model.trim="form.contactName" placeholder="联系人" maxlength="32" />
        </ElFormItem>
        <ElFormItem label="联系电话">
          <ElInput v-model.trim="form.contactPhone" placeholder="联系电话" maxlength="20" />
        </ElFormItem>
        <ElFormItem label="用户上限">
          <ElInputNumber v-model="form.maxUsers" :min="0" :max="100000" controls-position="right" />
          <span class="ml-2 text-xs text-g-500">0 表示不限制</span>
        </ElFormItem>
        <ElFormItem label="到期时间">
          <ElDatePicker
            v-model="form.expireAt"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="留空表示永久"
            class="!w-full"
          />
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
    fetchGetTenantList,
    fetchCreateTenant,
    fetchUpdateTenant,
    fetchDeleteTenant
  } from '@/api/business'
  import { ElTag, ElMessageBox, ElMessage, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'Tenant' })

  const loading = ref(false)
  const tableData = ref<WebBasic.Tenant[]>([])
  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editingId = ref<string | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  const form = reactive({
    name: '',
    code: '',
    contactName: '',
    contactPhone: '',
    maxUsers: 0,
    expireAt: '',
    status: 1,
    remark: ''
  })

  const rules: FormRules = {
    name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }],
    code: [
      { required: true, message: '请输入租户编码', trigger: 'blur' },
      { pattern: /^[a-z][\w-]*$/, message: '小写字母开头，可含数字、下划线、横线', trigger: 'blur' }
    ]
  }

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetTenantList()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'name', label: '租户名称', minWidth: 180 },
    { prop: 'code', label: '租户编码', width: 140 },
    { prop: 'contactName', label: '联系人', width: 120 },
    { prop: 'contactPhone', label: '联系电话', width: 140 },
    {
      prop: 'maxUsers',
      label: '用户上限',
      width: 110,
      formatter: (row: WebBasic.Tenant) => (row.maxUsers > 0 ? row.maxUsers : '不限')
    },
    {
      prop: 'expireAt',
      label: '到期时间',
      width: 180,
      formatter: (row: WebBasic.Tenant) => row.expireAt || '永久'
    },
    {
      prop: 'status',
      label: '状态',
      width: 90,
      formatter: (row: WebBasic.Tenant) =>
        h(
          ElTag,
          { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
          () => (row.status === 1 ? '正常' : '停用')
        )
    },
    {
      prop: 'operation',
      label: '操作',
      width: 80,
      fixed: 'right',
      formatter: (row: WebBasic.Tenant) =>
        h(ArtButtonMore, {
          list: [
            { key: 'edit', label: '编辑租户', icon: 'ri:edit-2-line', auth: 'system:tenant:update' },
            {
              key: 'delete',
              label: '删除租户',
              icon: 'ri:delete-bin-4-line',
              color: '#f56c6c',
              auth: 'system:tenant:delete'
            }
          ],
          onClick: (item: ButtonMoreItem) =>
            item.key === 'edit' ? openDialog('edit', row) : handleDelete(row)
        })
    }
  ])

  const openDialog = (type: 'add' | 'edit', row: WebBasic.Tenant | null) => {
    dialogType.value = type
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    Object.assign(form, {
      name: row?.name ?? '',
      code: row?.code ?? '',
      contactName: row?.contactName ?? '',
      contactPhone: row?.contactPhone ?? '',
      maxUsers: row?.maxUsers ?? 0,
      expireAt: row?.expireAt ?? '',
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
        await fetchCreateTenant({ ...form })
      } else if (editingId.value) {
        await fetchUpdateTenant(editingId.value, { ...form })
      }
      dialogVisible.value = false
      loadData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: WebBasic.Tenant) => {
    try {
      await ElMessageBox.confirm(`确定删除租户「${row.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteTenant(row.id)
    loadData()
  }
</script>
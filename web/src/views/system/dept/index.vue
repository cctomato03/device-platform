<!-- 部门管理（树形结构，web-basic 真实数据） -->
<template>
  <div class="dept-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:dept:create'" @click="openDialog('add', null)" v-ripple>
              新增顶级部门
            </ElButton>
            <ElButton @click="toggleExpand">{{ isExpanded ? '收起' : '展开' }}</ElButton>
            <ElInput v-model="keyword" placeholder="搜索部门名称" clearable class="!w-56" />
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable
        ref="tableRef"
        row-key="id"
        :loading="loading"
        :columns="columns"
        :data="filteredData"
        :tree-props="{ children: 'children' }"
        :default-expand-all="true"
      />
    </ElCard>

    <!-- 部门弹窗 -->
    <ElDialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增部门' : '编辑部门'"
      width="560px"
      align-center
      destroy-on-close
    >
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="90px">
        <ElFormItem label="上级部门">
          <ElSelect v-model="form.parentId" placeholder="顶级部门" clearable filterable>
            <ElOption label="顶级部门" value="0" />
            <ElOption
              v-for="d in parentOptions"
              :key="d.id"
              :label="d.label"
              :value="d.id"
              :style="{ paddingLeft: `${8 + d.depth * 14}px` }"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="部门名称" prop="name">
          <ElInput v-model.trim="form.name" placeholder="请输入部门名称" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="部门编码" prop="code">
          <ElInput v-model.trim="form.code" placeholder="如 RD" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="负责人">
          <ElInput v-model.trim="form.leader" placeholder="负责人姓名" maxlength="32" />
        </ElFormItem>
        <ElFormItem label="联系电话">
          <ElInput v-model.trim="form.phone" placeholder="联系电话" maxlength="20" />
        </ElFormItem>
        <ElFormItem label="邮箱" prop="email">
          <ElInput v-model.trim="form.email" placeholder="邮箱" maxlength="64" />
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
    fetchGetDeptTree,
    fetchCreateDept,
    fetchUpdateDept,
    fetchDeleteDept
  } from '@/api/business'
  import { ElTag, ElMessageBox, ElMessage, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'Dept' })

  const loading = ref(false)
  const isExpanded = ref(true)
  const tableRef = ref()
  const keyword = ref('')
  const tableData = ref<WebBasic.Dept[]>([])

  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editingId = ref<string | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  const form = reactive({
    parentId: '0',
    name: '',
    code: '',
    leader: '',
    phone: '',
    email: '',
    sort: 0,
    status: 1,
    remark: ''
  })

  const rules: FormRules = {
    name: [{ required: true, message: '请输入部门名称', trigger: 'blur' }],
    email: [{ type: 'email', message: '邮箱格式不正确', trigger: 'blur' }]
  }

  const loadData = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetDeptTree()
    } finally {
      loading.value = false
    }
  }

  onMounted(loadData)

  const filterTree = (nodes: WebBasic.Dept[], kw: string): WebBasic.Dept[] => {
    if (!kw) return nodes
    const out: WebBasic.Dept[] = []
    for (const n of nodes) {
      const children = n.children?.length ? filterTree(n.children, kw) : []
      if (n.name.toLowerCase().includes(kw.toLowerCase()) || children.length) {
        out.push({ ...n, children })
      }
    }
    return out
  }

  const filteredData = computed(() => filterTree(tableData.value, keyword.value.trim()))

  const parentOptions = computed(() => {
    const flat: Array<{ id: string; label: string; depth: number }> = []
    const walk = (nodes: WebBasic.Dept[], depth: number) => {
      nodes.forEach((n) => {
        flat.push({ id: n.id, label: n.name, depth })
        if (n.children?.length) walk(n.children, depth + 1)
      })
    }
    walk(tableData.value, 0)
    return flat
  })

  const { columnChecks, columns } = useTableColumns(() => [
    { prop: 'name', label: '部门名称', minWidth: 220 },
    { prop: 'code', label: '编码', width: 120 },
    { prop: 'leader', label: '负责人', width: 120 },
    { prop: 'phone', label: '联系电话', width: 140 },
    {
      prop: 'status',
      label: '状态',
      width: 90,
      formatter: (row: WebBasic.Dept) =>
        h(
          ElTag,
          { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
          () => (row.status === 1 ? '正常' : '停用')
        )
    },
    { prop: 'sort', label: '排序', width: 80 },
    {
      prop: 'operation',
      label: '操作',
      width: 90,
      fixed: 'right',
      formatter: (row: WebBasic.Dept) =>
        h(ArtButtonMore, {
          list: [
            { key: 'addChild', label: '新增子部门', icon: 'ri:add-line', auth: 'system:dept:create' },
            { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'system:dept:update' },
            { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'system:dept:delete' }
          ],
          onClick: (item: ButtonMoreItem) => handleMore(item, row)
        })
    }
  ])

  const handleMore = async (item: ButtonMoreItem, row: WebBasic.Dept) => {
    if (item.key === 'addChild') {
      openDialog('add', row)
    } else if (item.key === 'edit') {
      openDialog('edit', row)
    } else if (item.key === 'delete') {
      await handleDelete(row)
    }
  }

  const openDialog = (type: 'add' | 'edit', row: WebBasic.Dept | null) => {
    dialogType.value = type
    editingId.value = row?.id ?? null
    formRef.value?.clearValidate()
    if (type === 'edit' && row) {
      Object.assign(form, {
        parentId: row.parentId || '0',
        name: row.name,
        code: row.code,
        leader: row.leader,
        phone: row.phone,
        email: row.email,
        sort: row.sort,
        status: row.status,
        remark: row.remark ?? ''
      })
    } else {
      Object.assign(form, {
        parentId: row?.id ?? '0',
        name: '',
        code: '',
        leader: '',
        phone: '',
        email: '',
        sort: 0,
        status: 1,
        remark: ''
      })
    }
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    if (!formRef.value) return
    const valid = await formRef.value.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...form, parentId: form.parentId || '0' }
      if (dialogType.value === 'add') {
        await fetchCreateDept(payload)
      } else if (editingId.value) {
        await fetchUpdateDept(editingId.value, payload)
      }
      dialogVisible.value = false
      loadData()
    } finally {
      submitting.value = false
    }
  }

  const handleDelete = async (row: WebBasic.Dept) => {
    try {
      await ElMessageBox.confirm(`确定删除部门「${row.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteDept(row.id)
    loadData()
  }

  const toggleExpand = () => {
    isExpanded.value = !isExpanded.value
    nextTick(() => {
      const table = tableRef.value?.elTableRef
      if (!table) return
      const walk = (rows: WebBasic.Dept[]) => {
        rows.forEach((row) => {
          if (row.children?.length) {
            table.toggleRowExpansion(row, isExpanded.value)
            walk(row.children)
          }
        })
      }
      walk(filteredData.value)
    })
  }
</script>

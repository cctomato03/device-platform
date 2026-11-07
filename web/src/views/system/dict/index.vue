<!-- 字典管理（左侧字典类型 + 右侧字典项，web-basic 真实数据） -->
<template>
  <div class="dict-page art-full-height flex !flex-row gap-3">
    <!-- 字典类型 -->
    <ElCard class="art-table-card !w-96 shrink-0">
      <!-- 这里是「类型导航」而非表格，因此不使用 ArtTableHeader（表格设置只属于右侧字典项表格），
           只保留标题 + 新增 + 刷新三个动作，避免两个表格设置面板互相干扰 -->
      <div class="type-panel-header flex-cb mb-3 pb-3">
        <span class="type-panel-title">字典类型</span>
        <div class="flex-c gap-1">
          <ElButton v-auth="'system:dict:create'" @click="openTypeDialog('add', null)" v-ripple>
            新增
          </ElButton>
          <ElButton :loading="loadingTypes" circle text @click="loadTypes">
            <ArtSvgIcon icon="ri:refresh-line" />
          </ElButton>
        </div>
      </div>

      <ElInput v-model="typeKeyword" placeholder="搜索字典名称 / 编码" clearable class="mb-3" />

      <ElScrollbar height="calc(100vh - 320px)">
        <div
          v-for="t in filteredTypes"
          :key="t.id"
          class="dict-type-item"
          :class="{ active: currentType?.id === t.id }"
          @click="selectType(t)"
        >
          <div class="flex-1 overflow-hidden">
            <p class="truncate font-medium">{{ t.name }}</p>
            <p class="truncate text-xs text-g-500">{{ t.code }}</p>
          </div>
          <ElDropdown trigger="click" @command="(cmd: string) => handleTypeCommand(cmd, t)">
            <span class="text-g-500 px-1">⋯</span>
            <template #dropdown>
              <ElDropdownMenu>
                <ElDropdownItem command="edit">编辑</ElDropdownItem>
                <ElDropdownItem command="delete">删除</ElDropdownItem>
              </ElDropdownMenu>
            </template>
          </ElDropdown>
        </div>
        <ElEmpty v-if="!filteredTypes.length" description="暂无字典类型" />
      </ElScrollbar>
    </ElCard>

    <!-- 字典项 -->
    <ElCard class="art-table-card flex-1">
      <ArtTableHeader
        v-model:columns="columnChecks"
        :loading="loadingData"
        layout="refresh,size,fullscreen,columns,settings"
        @refresh="loadDictData"
      >
        <template #left>
          <ElSpace wrap>
            <ElTag v-if="currentType" type="info">{{ currentType.name }}（{{ currentType.code }}）</ElTag>
            <ElButton
              v-auth="'system:dict:create'"
              :disabled="!currentType"
              @click="openDataDialog('add', null)"
              v-ripple
            >
              新增字典项
            </ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>

      <ArtTable :loading="loadingData" :columns="dataColumns" :data="dictData" />
    </ElCard>

    <!-- 类型弹窗 -->
    <ElDialog
      v-model="typeDialogVisible"
      :title="typeDialogType === 'add' ? '新增字典类型' : '编辑字典类型'"
      width="480px"
      align-center
      destroy-on-close
    >
      <ElForm ref="typeFormRef" :model="typeForm" :rules="typeRules" label-width="90px">
        <ElFormItem label="类型名称" prop="name">
          <ElInput v-model.trim="typeForm.name" placeholder="如：用户性别" maxlength="100" />
        </ElFormItem>
        <ElFormItem label="字典编码" prop="code">
          <ElInput
            v-model.trim="typeForm.code"
            placeholder="如：sys_user_sex"
            :disabled="typeDialogType === 'edit'"
            maxlength="100"
          />
        </ElFormItem>
        <ElFormItem label="状态">
          <ElRadioGroup v-model="typeForm.status">
            <ElRadio :value="1">正常</ElRadio>
            <ElRadio :value="2">停用</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="typeForm.remark" type="textarea" :rows="2" maxlength="200" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="typeDialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitType">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 字典项弹窗 -->
    <ElDialog
      v-model="dataDialogVisible"
      :title="dataDialogType === 'add' ? '新增字典项' : '编辑字典项'"
      width="480px"
      align-center
      destroy-on-close
    >
      <ElForm ref="dataFormRef" :model="dataForm" :rules="dataRules" label-width="90px">
        <ElFormItem label="标签" prop="label">
          <ElInput v-model.trim="dataForm.label" placeholder="如：男" maxlength="100" />
        </ElFormItem>
        <ElFormItem label="键值" prop="value">
          <ElInput v-model.trim="dataForm.value" placeholder="如：1" maxlength="100" />
        </ElFormItem>
        <ElFormItem label="排序">
          <ElInputNumber v-model="dataForm.sort" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
        <ElFormItem label="标签类型">
          <ElSelect v-model="dataForm.tagType" placeholder="Element Plus Tag 类型" clearable>
            <ElOption label="primary" value="primary" />
            <ElOption label="success" value="success" />
            <ElOption label="info" value="info" />
            <ElOption label="warning" value="warning" />
            <ElOption label="danger" value="danger" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="状态">
          <ElRadioGroup v-model="dataForm.status">
            <ElRadio :value="1">正常</ElRadio>
            <ElRadio :value="2">停用</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="dataForm.remark" type="textarea" :rows="2" maxlength="200" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dataDialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitData">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import {
    fetchGetDictTypes,
    fetchCreateDictType,
    fetchUpdateDictType,
    fetchDeleteDictType,
    fetchGetDictData,
    fetchCreateDictData,
    fetchUpdateDictData,
    fetchDeleteDictData
  } from '@/api/business'
  import { ElTag, ElMessageBox, ElMessage, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'Dict' })

  const types = ref<WebBasic.DictType[]>([])
  const dictData = ref<WebBasic.DictData[]>([])
  const currentType = ref<WebBasic.DictType | null>(null)
  const typeKeyword = ref('')
  const loadingTypes = ref(false)
  const loadingData = ref(false)
  const submitting = ref(false)

  const filteredTypes = computed(() => {
    const kw = typeKeyword.value.trim().toLowerCase()
    if (!kw) return types.value
    return types.value.filter(
      (t) => t.name.toLowerCase().includes(kw) || t.code.toLowerCase().includes(kw)
    )
  })

  /* ---------------- 类型 ---------------- */
  const typeDialogVisible = ref(false)
  const typeDialogType = ref<'add' | 'edit'>('add')
  const typeEditingId = ref<string | null>(null)
  const typeFormRef = ref<FormInstance>()
  const typeForm = reactive({ name: '', code: '', status: 1, remark: '' })
  const typeRules: FormRules = {
    name: [{ required: true, message: '请输入类型名称', trigger: 'blur' }],
    code: [{ required: true, message: '请输入字典编码', trigger: 'blur' }]
  }

  const loadTypes = async () => {
    loadingTypes.value = true
    try {
      types.value = await fetchGetDictTypes()
      if (!currentType.value && types.value.length) selectType(types.value[0])
    } finally {
      loadingTypes.value = false
    }
  }

  const selectType = (t: WebBasic.DictType) => {
    currentType.value = t
    loadDictData()
  }

  const openTypeDialog = (type: 'add' | 'edit', row: WebBasic.DictType | null) => {
    typeDialogType.value = type
    typeEditingId.value = row?.id ?? null
    typeFormRef.value?.clearValidate()
    Object.assign(typeForm, {
      name: row?.name ?? '',
      code: row?.code ?? '',
      status: row?.status ?? 1,
      remark: row?.remark ?? ''
    })
    typeDialogVisible.value = true
  }

  const submitType = async () => {
    if (!typeFormRef.value) return
    const valid = await typeFormRef.value.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      if (typeDialogType.value === 'add') {
        await fetchCreateDictType({ ...typeForm })
      } else if (typeEditingId.value) {
        await fetchUpdateDictType(typeEditingId.value, { ...typeForm })
      }
      typeDialogVisible.value = false
      loadTypes()
    } finally {
      submitting.value = false
    }
  }

  const handleTypeCommand = async (cmd: string, t: WebBasic.DictType) => {
    if (cmd === 'edit') {
      openTypeDialog('edit', t)
      return
    }
    try {
      await ElMessageBox.confirm(`确定删除字典类型「${t.name}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteDictType(t.id)
    if (currentType.value?.id === t.id) {
      currentType.value = null
      dictData.value = []
    }
    loadTypes()
  }

  /* ---------------- 字典项 ---------------- */
  const dataDialogVisible = ref(false)
  const dataDialogType = ref<'add' | 'edit'>('add')
  const dataEditingId = ref<string | null>(null)
  const dataFormRef = ref<FormInstance>()
  const dataForm = reactive({
    label: '',
    value: '',
    sort: 0,
    tagType: '',
    status: 1,
    remark: ''
  })
  const dataRules: FormRules = {
    label: [{ required: true, message: '请输入标签', trigger: 'blur' }],
    value: [{ required: true, message: '请输入键值', trigger: 'blur' }]
  }

  const loadDictData = async () => {
    if (!currentType.value) return
    loadingData.value = true
    try {
      dictData.value = await fetchGetDictData(currentType.value.code)
    } finally {
      loadingData.value = false
    }
  }

  const openDataDialog = (type: 'add' | 'edit', row: WebBasic.DictData | null) => {
    dataDialogType.value = type
    dataEditingId.value = row?.id ?? null
    dataFormRef.value?.clearValidate()
    Object.assign(dataForm, {
      label: row?.label ?? '',
      value: row?.value ?? '',
      sort: row?.sort ?? 0,
      tagType: row?.tagType ?? '',
      status: row?.status ?? 1,
      remark: ''
    })
    dataDialogVisible.value = true
  }

  const submitData = async () => {
    if (!dataFormRef.value || !currentType.value) return
    const valid = await dataFormRef.value.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...dataForm, dictCode: currentType.value.code }
      if (dataDialogType.value === 'add') {
        await fetchCreateDictData(payload)
      } else if (dataEditingId.value) {
        await fetchUpdateDictData(dataEditingId.value, payload)
      }
      dataDialogVisible.value = false
      loadDictData()
    } finally {
      submitting.value = false
    }
  }

  const handleDeleteData = async (row: WebBasic.DictData) => {
    try {
      await ElMessageBox.confirm(`确定删除字典项「${row.label}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteDictData(row.id)
    loadDictData()
  }

  const { columns: dataColumns, columnChecks } = useTableColumns(() => [
    { prop: 'label', label: '标签', minWidth: 140 },
    { prop: 'value', label: '键值', minWidth: 120 },
    { prop: 'sort', label: '排序', width: 80 },
    {
      prop: 'tagType',
      label: '标签样式',
      width: 120,
      formatter: (row: WebBasic.DictData) =>
        row.tagType
          ? h(ElTag, { type: row.tagType as any, size: 'small' }, () => row.tagType)
          : '-'
    },
    {
      prop: 'status',
      label: '状态',
      width: 90,
      formatter: (row: WebBasic.DictData) =>
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
      formatter: (row: WebBasic.DictData) =>
        h(ArtButtonMore, {
          list: [
            { key: 'edit', label: '编辑字典项', icon: 'ri:edit-2-line', auth: 'system:dict:update' },
            {
              key: 'delete',
              label: '删除字典项',
              icon: 'ri:delete-bin-4-line',
              color: '#f56c6c',
              auth: 'system:dict:delete'
            }
          ],
          onClick: (item: ButtonMoreItem) =>
            item.key === 'edit' ? openDataDialog('edit', row) : handleDeleteData(row)
        })
    }
  ])

  onMounted(loadTypes)
</script>

<style scoped>
  .dict-type-item {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 10px;
    margin-bottom: 6px;
    cursor: pointer;
    border-radius: 8px;
    transition: all 0.2s;
  }

  .dict-type-item:hover {
    background: var(--art-hover-color, rgb(0 0 0 / 4%));
  }

  .dict-type-item.active {
    background: var(--art-theme-color-12, rgb(64 128 255 / 12%));
  }
</style>
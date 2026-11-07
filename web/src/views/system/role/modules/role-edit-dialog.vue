<template>
  <ElDialog
    v-model="visible"
    :title="dialogType === 'add' ? '新增角色' : '编辑角色'"
    width="520px"
    align-center
    destroy-on-close
  >
    <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
      <ElFormItem label="角色名称" prop="name">
        <ElInput v-model.trim="form.name" placeholder="请输入角色名称" maxlength="64" />
      </ElFormItem>
      <ElFormItem label="角色编码" prop="code">
        <ElInput
          v-model.trim="form.code"
          placeholder="如 ops / auditor"
          :disabled="dialogType === 'edit'"
          maxlength="64"
        />
      </ElFormItem>
      <ElFormItem label="数据范围" prop="dataScope">
        <ElSelect v-model="form.dataScope" placeholder="请选择数据范围">
          <ElOption :value="1" label="全部数据" />
          <ElOption :value="2" label="本部门数据" />
          <ElOption :value="3" label="本部门及以下" />
          <ElOption :value="4" label="仅本人数据" />
          <ElOption :value="5" label="自定义部门" />
        </ElSelect>
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
      <ElFormItem label="描述">
        <ElInput
          v-model="form.remark"
          type="textarea"
          :rows="3"
          maxlength="200"
          show-word-limit
          placeholder="请输入角色描述"
        />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import { fetchCreateRole, fetchUpdateRole } from '@/api/system-manage'

  type RoleListItem = Api.SystemManage.RoleListItem

  interface Props {
    modelValue: boolean
    dialogType: 'add' | 'edit'
    roleData?: RoleListItem
  }

  const props = withDefaults(defineProps<Props>(), {
    modelValue: false,
    dialogType: 'add',
    roleData: undefined
  })
  const emit = defineEmits<{
    (e: 'update:modelValue', value: boolean): void
    (e: 'success'): void
  }>()

  const visible = computed({
    get: () => props.modelValue,
    set: (value) => emit('update:modelValue', value)
  })

  const formRef = ref<FormInstance>()
  const submitting = ref(false)

  const form = reactive({
    name: '',
    code: '',
    dataScope: 4,
    sort: 0,
    status: 1,
    remark: ''
  })

  const rules = reactive<FormRules>({
    name: [
      { required: true, message: '请输入角色名称', trigger: 'blur' },
      { min: 2, max: 64, message: '长度在 2 到 64 个字符', trigger: 'blur' }
    ],
    code: [
      { required: true, message: '请输入角色编码', trigger: 'blur' },
      { pattern: /^[a-zA-Z][\w-]*$/, message: '以字母开头，可包含字母数字下划线和横线', trigger: 'blur' }
    ],
    dataScope: [{ required: true, message: '请选择数据范围', trigger: 'change' }]
  })

  watch(
    () => props.modelValue,
    (val) => {
      if (!val) return
      formRef.value?.clearValidate()
      if (props.dialogType === 'edit' && props.roleData) {
        form.name = props.roleData.roleName
        form.code = props.roleData.roleCode
        form.dataScope = props.roleData.dataScope ?? 4
        form.sort = props.roleData.sort ?? 0
        form.status = props.roleData.enabled ? 1 : 2
        form.remark = props.roleData.description || ''
      } else {
        Object.assign(form, { name: '', code: '', dataScope: 4, sort: 0, status: 1, remark: '' })
      }
    }
  )

  const handleSubmit = async () => {
    if (!formRef.value) return
    const valid = await formRef.value.validate().catch(() => false)
    if (!valid) return

    submitting.value = true
    try {
      const payload = {
        name: form.name,
        code: form.code,
        dataScope: form.dataScope,
        sort: form.sort,
        status: form.status,
        remark: form.remark
      }
      if (props.dialogType === 'add') {
        await fetchCreateRole(payload)
      } else {
        await fetchUpdateRole(props.roleData?.roleId as string, payload)
      }
      emit('success')
      visible.value = false
    } finally {
      submitting.value = false
    }
  }
</script>

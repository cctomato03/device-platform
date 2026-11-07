<template>
  <ElDialog
    v-model="dialogVisible"
    :title="type === 'add' ? '新增用户' : '编辑用户'"
    width="640px"
    align-center
    destroy-on-close
  >
    <ElForm ref="formRef" :model="form" :rules="rules" label-width="90px">
      <ElFormItem label="用户名" prop="username">
        <ElInput
          v-model.trim="form.username"
          placeholder="请输入登录账号"
          :disabled="type === 'edit'"
          maxlength="32"
        />
      </ElFormItem>
      <ElFormItem label="昵称" prop="nickname">
        <ElInput v-model.trim="form.nickname" placeholder="请输入昵称" maxlength="64" />
      </ElFormItem>
      <ElFormItem label="所属部门" prop="deptId">
        <ElSelect v-model="form.deptId" placeholder="请选择部门" clearable filterable>
          <ElOption
            v-for="d in deptOptions"
            :key="d.id"
            :label="d.label"
            :value="d.id"
            :style="{ paddingLeft: `${8 + d.depth * 14}px` }"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="角色" prop="roleIds">
        <ElSelect v-model="form.roleIds" placeholder="请选择角色" multiple clearable>
          <ElOption v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="手机号" prop="mobile">
        <ElInput v-model.trim="form.mobile" placeholder="请输入手机号" maxlength="20" />
      </ElFormItem>
      <ElFormItem label="邮箱" prop="email">
        <ElInput v-model.trim="form.email" placeholder="请输入邮箱" maxlength="64" />
      </ElFormItem>
      <ElFormItem label="性别">
        <ElRadioGroup v-model="form.sex">
          <ElRadio :value="1">男</ElRadio>
          <ElRadio :value="2">女</ElRadio>
          <ElRadio :value="0">未知</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem label="状态">
        <ElRadioGroup v-model="form.status">
          <ElRadio :value="1">正常</ElRadio>
          <ElRadio :value="2">停用</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem v-if="type === 'add'" label="初始密码" prop="password">
        <ElInput
          v-model.trim="form.password"
          type="password"
          show-password
          placeholder="留空则由后端生成随机密码"
          maxlength="32"
        />
      </ElFormItem>
      <ElFormItem label="备注">
        <ElInput v-model="form.remark" type="textarea" :rows="2" maxlength="200" show-word-limit />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <div class="dialog-footer">
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import { fetchCreateUser, fetchUpdateUser, fetchGetAllRoles } from '@/api/system-manage'
  import { fetchGetDeptTree } from '@/api/business'

  interface Props {
    visible: boolean
    type: 'add' | 'edit'
    userData?: Partial<Api.SystemManage.UserListItem>
  }
  interface Emits {
    (e: 'update:visible', value: boolean): void
    (e: 'success'): void
  }

  const props = defineProps<Props>()
  const emit = defineEmits<Emits>()

  defineOptions({ name: 'UserDialog' })

  const dialogVisible = computed({
    get: () => props.visible,
    set: (val) => emit('update:visible', val)
  })

  const formRef = ref<FormInstance>()
  const submitting = ref(false)

  const form = reactive({
    username: '',
    nickname: '',
    deptId: '' as string | number | '',
    roleIds: [] as Array<string | number>,
    mobile: '',
    email: '',
    sex: 1,
    status: 1,
    password: '',
    remark: ''
  })

  const rules: FormRules = {
    username: [
      { required: true, message: '请输入登录账号', trigger: 'blur' },
      { min: 3, max: 32, message: '长度 3 ~ 32 个字符', trigger: 'blur' }
    ],
    email: [{ type: 'email', message: '邮箱格式不正确', trigger: 'blur' }],
    mobile: [{ pattern: /^1[3-9]\d{9}$/, message: '手机号格式不正确', trigger: 'blur' }]
  }

  // 角色下拉
  const roleOptions = ref<Array<{ id: string | number; name: string }>>([])
  // 部门下拉（树拍平）
  const deptOptions = ref<Array<{ id: string | number; label: string; depth: number }>>([])

  const flattenDept = (
    nodes: WebBasic.Dept[],
    depth = 0
  ): Array<{ id: string; label: string; depth: number }> =>
    nodes.flatMap((n) => [
      { id: n.id, label: n.name, depth },
      ...flattenDept(n.children || [], depth + 1)
    ])

  const loadOptions = async () => {
    try {
      const [roles, depts] = await Promise.all([fetchGetAllRoles(), fetchGetDeptTree()])
      roleOptions.value = roles.map((r) => ({ id: r.roleId, name: r.roleName }))
      deptOptions.value = flattenDept(depts)
    } catch {
      /* 错误已由 http 层提示 */
    }
  }

  // 打开弹窗时同步数据
  watch(
    () => props.visible,
    async (val) => {
      if (!val) return
      await loadOptions()
      formRef.value?.clearValidate()
      if (props.type === 'edit' && props.userData) {
        const u = props.userData
        form.username = u.userName || ''
        form.nickname = u.nickName || ''
        form.deptId = u.deptId || ''
        form.roleIds = u.roleIds || []
        form.mobile = u.userPhone || ''
        form.email = u.userEmail || ''
        form.sex = u.sex ?? 1
        form.status = Number(u.status) === 2 ? 2 : 1
        form.password = ''
        form.remark = u.remark || ''
      } else {
        Object.assign(form, {
          username: '',
          nickname: '',
          deptId: '',
          roleIds: [],
          mobile: '',
          email: '',
          sex: 1,
          status: 1,
          password: '',
          remark: ''
        })
      }
    }
  )

  const handleSubmit = async () => {
    if (!formRef.value) return
    const valid = await formRef.value.validate().catch(() => false)
    if (!valid) return

    submitting.value = true
    try {
      const payload: Record<string, any> = {
        nickname: form.nickname,
        deptId: form.deptId || 0,
        roleIds: form.roleIds,
        mobile: form.mobile,
        email: form.email,
        sex: form.sex,
        status: form.status,
        remark: form.remark
      }
      if (props.type === 'add') {
        payload.username = form.username
        if (form.password) payload.password = form.password
        await fetchCreateUser(payload)
      } else {
        await fetchUpdateUser(props.userData?.id as string, payload)
      }
      emit('success')
      dialogVisible.value = false
    } catch (error) {
      console.error('[UserDialog] submit failed', error)
    } finally {
      submitting.value = false
    }
  }
</script>

<style scoped>
  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>

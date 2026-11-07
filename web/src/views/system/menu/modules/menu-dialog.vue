<template>
  <ElDialog
    v-model="visible"
    :title="type === 'add' ? '新增菜单' : '编辑菜单'"
    width="680px"
    align-center
    destroy-on-close
  >
    <ElForm ref="formRef" :model="form" :rules="rules" label-width="110px">
      <ElFormItem label="上级菜单">
        <ElSelect v-model="form.parentId" placeholder="顶级菜单" clearable filterable>
          <ElOption label="顶级菜单" value="0" />
          <ElOption
            v-for="d in parentOptions"
            :key="d.id"
            :label="d.label"
            :value="d.id"
            :style="{ paddingLeft: `${8 + d.depth * 14}px` }"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="菜单类型" prop="type">
        <ElRadioGroup v-model="form.type">
          <ElRadioButton :value="1">目录</ElRadioButton>
          <ElRadioButton :value="2">菜单</ElRadioButton>
          <ElRadioButton :value="3">按钮</ElRadioButton>
          <ElRadioButton :value="4">接口</ElRadioButton>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem label="名称" prop="name">
        <ElInput v-model.trim="form.name" placeholder="如：用户管理" maxlength="64" />
      </ElFormItem>
      <ElFormItem v-if="form.type === 1 || form.type === 2" label="路由地址" prop="path">
        <ElInput v-model.trim="form.path" placeholder="目录用 /system，菜单用 user（相对父级）" />
      </ElFormItem>
      <ElFormItem v-if="form.type === 2" label="组件路径" prop="component">
        <ElInput
          v-model.trim="form.component"
          placeholder="如：system/user/index（views 目录下的相对路径）"
        />
      </ElFormItem>
      <ElFormItem v-if="form.type === 1 || form.type === 2" label="图标">
        <div class="flex w-full items-center gap-2">
          <ElSelect
            v-model="form.icon"
            filterable
            allow-create
            clearable
            default-first-option
            placeholder="选择或输入 Iconify 名称，如 ep:user / ri:user-line"
            class="flex-1"
          >
            <ElOption v-for="ic in ICON_OPTIONS" :key="ic.value" :label="ic.label" :value="ic.value">
              <div class="flex items-center gap-2">
                <ArtSvgIcon :icon="ic.value" class="text-base" />
                <span>{{ ic.label }}</span>
                <span class="ml-auto text-xs text-g-500">{{ ic.value }}</span>
              </div>
            </ElOption>
          </ElSelect>
          <ArtSvgIcon v-if="form.icon" :icon="form.icon" class="text-xl text-theme" />
        </div>
        <div class="mt-1 text-xs text-g-500">
          图标集：Element Plus（ep:）/ Remix Icon（ri:）；更多名称见 https://icon-sets.iconify.design
        </div>
      </ElFormItem>
      <ElFormItem v-if="form.type !== 1" label="权限标识" prop="permission">
        <ElInput v-model.trim="form.permission" placeholder="如：system:user:create" />
      </ElFormItem>
      <ElFormItem v-if="form.type === 4" label="接口方法">
        <ElSelect v-model="form.apiMethod" placeholder="请求方法" clearable>
          <ElOption label="GET" value="GET" />
          <ElOption label="POST" value="POST" />
          <ElOption label="PUT" value="PUT" />
          <ElOption label="DELETE" value="DELETE" />
        </ElSelect>
      </ElFormItem>
      <ElFormItem v-if="form.type === 4" label="接口路径">
        <ElInput v-model.trim="form.apiPath" placeholder="如：/api/v1/system/users" />
      </ElFormItem>
      <ElFormItem v-if="form.type === 1 || form.type === 2" label="排序">
        <ElInputNumber v-model="form.sort" :min="0" :max="9999" controls-position="right" />
      </ElFormItem>
      <ElFormItem v-if="form.type === 1 || form.type === 2" label="显示/缓存">
        <ElSpace>
          <ElCheckbox v-model="form.visible">在菜单中显示</ElCheckbox>
          <ElCheckbox v-model="form.keepAlive">页面缓存</ElCheckbox>
          <ElCheckbox v-model="form.affix">固定页签</ElCheckbox>
        </ElSpace>
      </ElFormItem>
      <ElFormItem label="状态">
        <ElRadioGroup v-model="form.status">
          <ElRadio :value="1">正常</ElRadio>
          <ElRadio :value="2">停用</ElRadio>
        </ElRadioGroup>
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
  import { fetchCreateMenu, fetchUpdateMenu } from '@/api/system-manage'

  interface Props {
    visible: boolean
    type: 'add' | 'edit'
    editData?: WebBasic.Menu | null
    parentOptions?: Array<{ id: string; label: string; depth: number }>
    defaultParentId?: string
  }

  /** 常用图标（可在下拉里直接选，也可手动输入任意 Iconify 名称） */
const ICON_OPTIONS = [
  { label: '用户', value: 'ep:user' },
  { label: '角色/头像', value: 'ep:avatar' },
  { label: '菜单', value: 'ep:menu' },
  { label: '组织/部门', value: 'ep:office-building' },
  { label: '岗位', value: 'ep:postcard' },
  { label: '权限/密钥', value: 'ep:key' },
  { label: '字典/笔记', value: 'ep:notebook' },
  { label: '参数/设置', value: 'ep:operation' },
  { label: '系统配置', value: 'ep:setting' },
  { label: '文件夹/文件', value: 'ep:folder' },
  { label: '定时任务', value: 'ep:timer' },
  { label: '监控', value: 'ep:monitor' },
  { label: '在线/连接', value: 'ep:connection' },
  { label: '服务器/CPU', value: 'ep:cpu' },
  { label: '日志/记录', value: 'ep:tickets' },
  { label: '仪表盘', value: 'ep:odometer' },
  { label: '首页', value: 'ep:home-filled' },
  { label: '数据', value: 'ep:data-line' },
  { label: '图表', value: 'ep:trend-charts' },
  { label: '标签', value: 'ep:collection-tag' },
  { label: '盒子/租户', value: 'ep:box' },
  { label: '管理', value: 'ep:management' },
  { label: '指南', value: 'ep:guide' },
  { label: '通知', value: 'ep:bell' }
]
const props = withDefaults(defineProps<Props>(), {
    editData: null,
    parentOptions: () => [],
    defaultParentId: '0'
  })
  const emit = defineEmits<{
    (e: 'update:visible', value: boolean): void
    (e: 'success'): void
  }>()

  const visible = computed({
    get: () => props.visible,
    set: (value) => emit('update:visible', value)
  })

  const formRef = ref<FormInstance>()
  const submitting = ref(false)

  const emptyForm = {
    parentId: '0',
    type: 2,
    name: '',
    path: '',
    component: '',
    permission: '',
    icon: '',
    apiMethod: '',
    apiPath: '',
    sort: 0,
    visible: true,
    keepAlive: true,
    affix: false,
    status: 1
  }

  const form = reactive({ ...emptyForm })

  const rules = reactive<FormRules>({
    name: [{ required: true, message: '请输入菜单名称', trigger: 'blur' }],
    path: [
      {
        validator: (_r, _v, cb) => {
          if ((form.type === 1 || form.type === 2) && !form.path) {
            cb(new Error('请输入路由地址'))
          } else {
            cb()
          }
        },
        trigger: 'blur'
      }
    ],
    component: [
      {
        validator: (_r, _v, cb) => {
          if (form.type === 2 && !form.component) cb(new Error('请输入组件路径'))
          else cb()
        },
        trigger: 'blur'
      }
    ]
  })

  watch(
    () => props.visible,
    (val) => {
      if (!val) return
      formRef.value?.clearValidate()
      if (props.type === 'edit' && props.editData) {
        const m = props.editData
        Object.assign(form, {
          parentId: m.parentId || '0',
          type: m.type,
          name: m.name,
          path: m.path,
          component: m.component,
          permission: m.permission,
          icon: m.icon,
          apiMethod: m.apiMethod || '',
          apiPath: m.apiPath || '',
          sort: m.sort,
          visible: m.visible,
          keepAlive: m.keepAlive,
          affix: m.affix,
          status: m.status
        })
      } else {
        Object.assign(form, { ...emptyForm, parentId: props.defaultParentId || '0' })
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
        parentId: form.parentId || '0',
        type: form.type,
        name: form.name,
        path: form.type === 3 ? '' : form.path,
        component: form.type === 2 ? form.component : '',
        permission: form.type === 1 ? '' : form.permission,
        icon: form.type === 3 || form.type === 4 ? '' : form.icon,
        apiMethod: form.type === 4 ? form.apiMethod : '',
        apiPath: form.type === 4 ? form.apiPath : '',
        sort: form.sort,
        visible: form.visible,
        keepAlive: form.keepAlive,
        affix: form.affix,
        status: form.status
      }
      if (props.type === 'add') {
        await fetchCreateMenu(payload)
      } else {
        await fetchUpdateMenu(props.editData?.id as string, payload)
      }
      emit('success')
      visible.value = false
    } finally {
      submitting.value = false
    }
  }
</script>

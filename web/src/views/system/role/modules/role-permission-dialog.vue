<template>
  <ElDialog
    v-model="visible"
    :title="`菜单权限 - ${roleData?.roleName || ''}`"
    width="560px"
    align-center
    class="el-dialog-border"
    destroy-on-close
  >
    <div class="mb-2 flex items-center justify-between">
      <ElInput v-model="filterText" placeholder="搜索菜单名称" clearable class="!w-56" />
      <ElSpace>
        <ElButton @click="toggleExpandAll">{{ isExpandAll ? '全部收起' : '全部展开' }}</ElButton>
        <ElButton @click="toggleSelectAll">{{ isSelectAll ? '取消全选' : '全部选择' }}</ElButton>
      </ElSpace>
    </div>

    <ElScrollbar height="60vh" v-loading="loading">
      <ElTree
        ref="treeRef"
        :data="menuTree"
        show-checkbox
        node-key="id"
        :filter-node-method="filterNode"
        :default-expand-all="isExpandAll"
        :props="{ label: 'name', children: 'children' }"
        @check="handleTreeCheck"
      >
        <template #default="{ data }">
          <span class="flex items-center gap-2">
            <span>{{ data.name }}</span>
            <ElTag v-if="data.type === 3" size="small" type="info">按钮</ElTag>
            <span v-if="data.permission" class="text-xs text-g-500">{{ data.permission }}</span>
          </span>
        </template>
      </ElTree>
    </ElScrollbar>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="savePermission">保存</ElButton>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import { fetchGetMenuTree, fetchGetRoleGranted, fetchGrantRole } from '@/api/system-manage'
  import { ElMessage } from 'element-plus'
  import type { ElTree } from 'element-plus'

  type RoleListItem = Api.SystemManage.RoleListItem

  interface Props {
    modelValue: boolean
    roleData?: RoleListItem
  }

  const props = withDefaults(defineProps<Props>(), {
    modelValue: false,
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

  const treeRef = ref<InstanceType<typeof ElTree>>()
  const menuTree = ref<WebBasic.Menu[]>([])
  const filterText = ref('')
  const isExpandAll = ref(true)
  const isSelectAll = ref(false)
  const loading = ref(false)
  const saving = ref(false)

  /** 收集所有节点 ID */
  const collectIds = (nodes: WebBasic.Menu[]): string[] =>
    nodes.flatMap((n) => [n.id, ...collectIds(n.children || [])])

  /** 收集叶子节点 ID */
  const collectLeafIds = (nodes: WebBasic.Menu[]): string[] =>
    nodes.flatMap((n) => (n.children?.length ? collectLeafIds(n.children) : [n.id]))

  /** 搜索过滤 */
  const filterNode = (value: string, data: any) => {
    if (!value) return true
    return String(data.name).includes(value) || String(data.permission || '').includes(value)
  }

  watch(filterText, (val) => {
    treeRef.value?.filter(val)
  })

  /** 打开弹窗：加载菜单树 + 已授权菜单 */
  watch(
    () => props.modelValue,
    async (val) => {
      if (!val) return
      loading.value = true
      try {
        const [tree, granted] = await Promise.all([
          fetchGetMenuTree(),
          props.roleData?.roleId ? fetchGetRoleGranted(props.roleData.roleId) : Promise.resolve([])
        ])
        menuTree.value = tree
        await nextTick()
        // 只回填叶子节点，父节点由树自动半选，避免整棵目录被误勾选
        const grantedSet = new Set(granted.map(String))
        const leafKeys = collectLeafIds(tree).filter((id) => grantedSet.has(id))
        treeRef.value?.setCheckedKeys(leafKeys, false)
        isSelectAll.value = false
      } finally {
        loading.value = false
      }
    }
  )

  const toggleExpandAll = () => {
    const tree = treeRef.value
    if (!tree) return
    const nodes = (tree as any).store.nodesMap
    Object.values(nodes).forEach((node: any) => {
      node.expanded = !isExpandAll.value
    })
    isExpandAll.value = !isExpandAll.value
  }

  const toggleSelectAll = () => {
    if (!treeRef.value) return
    if (!isSelectAll.value) {
      treeRef.value.setCheckedKeys(collectIds(menuTree.value), false)
    } else {
      treeRef.value.setCheckedKeys([], false)
    }
    isSelectAll.value = !isSelectAll.value
  }

  const handleTreeCheck = () => {
    const tree = treeRef.value
    if (!tree) return
    const checked = tree.getCheckedKeys()
    const all = collectIds(menuTree.value)
    isSelectAll.value = checked.length === all.length && all.length > 0
  }

  /** 保存授权（含半选父级目录，保证菜单可正常渲染） */
  const savePermission = async () => {
    const tree = treeRef.value
    if (!tree || !props.roleData?.roleId) return
    saving.value = true
    try {
      const menuIds = [...tree.getCheckedKeys(), ...tree.getHalfCheckedKeys()]
      await fetchGrantRole(props.roleData.roleId, { menuIds: menuIds as string[] })
        emit('success')
      visible.value = false
    } finally {
      saving.value = false
    }
  }
</script>

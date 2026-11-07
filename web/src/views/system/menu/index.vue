<!-- 菜单管理页面（web-basic 真实数据版：目录 / 菜单 / 按钮 / 接口四层） -->
<template>
  <div class="menu-page art-full-height">
    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="getMenuList">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:menu:create'" @click="handleAdd('0')" v-ripple>
              新增顶级菜单
            </ElButton>
            <ElButton @click="toggleExpand">{{ isExpanded ? '收起' : '展开' }}</ElButton>
            <ElInput
              v-model="keyword"
              placeholder="搜索菜单名称 / 权限标识"
              clearable
              class="!w-60"
            />
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
        :default-expand-all="false"
      />

      <MenuDialog
        v-model:visible="dialogVisible"
        :type="dialogType"
        :edit-data="editData"
        :parent-options="parentOptions"
        :default-parent-id="defaultParentId"
        @success="getMenuList"
      />
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import ArtSvgIcon from '@/components/core/base/art-svg-icon/index.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTableColumns } from '@/hooks/core/useTableColumns'
  import { fetchGetMenuTree, fetchDeleteMenu } from '@/api/system-manage'
  import MenuDialog from './modules/menu-dialog.vue'
  import { ElTag, ElMessageBox, ElMessage } from 'element-plus'

  defineOptions({ name: 'Menus' })

  const loading = ref(false)
  const isExpanded = ref(false)
  const tableRef = ref()
  const keyword = ref('')

  const dialogVisible = ref(false)
  const dialogType = ref<'add' | 'edit'>('add')
  const editData = ref<WebBasic.Menu | null>(null)
  const defaultParentId = ref('0')

  const tableData = ref<WebBasic.Menu[]>([])

  /** 递归过滤（保留命中节点的祖先链） */
  const filterTree = (nodes: WebBasic.Menu[], kw: string): WebBasic.Menu[] => {
    if (!kw) return nodes
    const lower = kw.toLowerCase()
    const out: WebBasic.Menu[] = []
    for (const n of nodes) {
      const hit =
        n.name.toLowerCase().includes(lower) ||
        (n.permission || '').toLowerCase().includes(lower) ||
        (n.path || '').toLowerCase().includes(lower)
      const children = n.children?.length ? filterTree(n.children, kw) : []
      if (hit || children.length) out.push({ ...n, children })
    }
    return out
  }

  const filteredData = computed(() => filterTree(tableData.value, keyword.value.trim()))

  /** 上级菜单下拉（目录/菜单才可作为父级） */
  const parentOptions = computed(() => {
    const flat: Array<{ id: string; label: string; depth: number }> = []
    const walk = (nodes: WebBasic.Menu[], depth: number) => {
      nodes.forEach((n) => {
        if (n.type === 1 || n.type === 2) {
          flat.push({ id: n.id, label: `${n.name}（${n.path || '-'}）`, depth })
          if (n.children?.length) walk(n.children, depth + 1)
        }
      })
    }
    walk(tableData.value, 0)
    return flat
  })

  const getMenuList = async () => {
    loading.value = true
    try {
      tableData.value = await fetchGetMenuTree()
    } finally {
      loading.value = false
    }
  }

  onMounted(getMenuList)

  const TYPE_TAG: Record<number, { type: 'primary' | 'success' | 'info' | 'warning'; text: string }> =
    {
      1: { type: 'info', text: '目录' },
      2: { type: 'primary', text: '菜单' },
      3: { type: 'success', text: '按钮' },
      4: { type: 'warning', text: '接口' }
    }

  const { columnChecks, columns } = useTableColumns(() => [
    {
      prop: 'name',
      label: '菜单名称',
      minWidth: 220,
      // 注意：树形表格的展开箭头由 el-table 插入到单元格首部，
      // 这里必须用 inline-flex（而不是 block 的 div），否则箭头与名称会分行显示。
      formatter: (row: WebBasic.Menu) =>
        h('span', { class: 'inline-flex items-center gap-2 align-middle' }, [
          row.icon
            ? h(ArtSvgIcon, { icon: row.icon, class: 'text-theme shrink-0 text-base' })
            : h(ArtSvgIcon, { icon: 'ep:minus', class: 'shrink-0 text-base text-g-400' }),
          h('span', { class: 'leading-5' }, row.name)
        ])
    },
    {
      prop: 'type',
      label: '类型',
      width: 90,
      formatter: (row: WebBasic.Menu) => {
        const cfg = TYPE_TAG[row.type] || { type: 'info' as const, text: '未知' }
        return h(ElTag, { type: cfg.type, size: 'small' }, () => cfg.text)
      }
    },
    {
      prop: 'permission',
      label: '权限标识',
      minWidth: 190,
      formatter: (row: WebBasic.Menu) => row.permission || '-'
    },
    {
      prop: 'path',
      label: '路由 / 接口',
      minWidth: 190,
      formatter: (row: WebBasic.Menu) =>
        row.type === 4 ? `${row.apiMethod} ${row.apiPath}` : row.path || '-'
    },
    {
      prop: 'component',
      label: '组件路径',
      minWidth: 180,
      formatter: (row: WebBasic.Menu) => row.component || '-'
    },
    { prop: 'sort', label: '排序', width: 80 },
    {
      prop: 'status',
      label: '状态',
      width: 90,
      formatter: (row: WebBasic.Menu) =>
        h(
          ElTag,
          { type: row.status === 1 ? 'success' : 'danger', size: 'small' },
          () => (row.status === 1 ? '正常' : '停用')
        )
    },
    {
      prop: 'operation',
      label: '操作',
      width: 90,
      fixed: 'right',
      formatter: (row: WebBasic.Menu) =>
        h(ArtButtonMore, {
          list: [
            ...(row.type === 1 || row.type === 2
              ? [{ key: 'addChild', label: '新增子项', icon: 'ri:add-line', auth: 'system:menu:create' }]
              : []),
            { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'system:menu:update' },
            { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'system:menu:delete' }
          ],
          onClick: (item: ButtonMoreItem) => handleMoreClick(item, row)
        })
    }
  ])

  const handleMoreClick = async (item: ButtonMoreItem, row: WebBasic.Menu) => {
    switch (item.key) {
      case 'addChild':
        handleAdd(row.id)
        break
      case 'edit':
        dialogType.value = 'edit'
        editData.value = row
        dialogVisible.value = true
        break
      case 'delete':
        await handleDelete(row)
        break
    }
  }

  const handleAdd = (parentId: string) => {
    dialogType.value = 'add'
    editData.value = null
    defaultParentId.value = parentId
    dialogVisible.value = true
  }

  const handleDelete = async (row: WebBasic.Menu) => {
    try {
      await ElMessageBox.confirm(`确定删除「${row.name}」吗？存在子菜单时将无法删除。`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteMenu(row.id)
    getMenuList()
  }

  const toggleExpand = () => {
    isExpanded.value = !isExpanded.value
    nextTick(() => {
      const table = tableRef.value?.elTableRef
      if (!table) return
      const walk = (rows: WebBasic.Menu[]) => {
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

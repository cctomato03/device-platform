<!-- 用户管理页面（web-basic 真实数据版） -->
<!-- art-full-height 自动计算出页面剩余高度；art-table-card 系统表格卡片样式 -->
<template>
  <div class="user-page art-full-height">
    <!-- 搜索栏 -->
    <UserSearch v-model="searchForm" @search="handleSearch" @reset="resetSearchParams" />

    <ElCard class="art-table-card">
      <!-- 表格头部 -->
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'system:user:create'" @click="showDialog('add')" v-ripple>
              新增用户
            </ElButton>
            <ElButton
              v-auth="'system:user:delete'"
              :disabled="!selectedRows.length"
              @click="deleteUsers(selectedRows)"
            >
              批量删除
            </ElButton>
            <ElButton v-auth="'system:user:export'" :loading="exporting" @click="handleExport">
              导出
            </ElButton>
            <ElButton v-auth="'system:user:import'" :loading="importing" @click="triggerImport">
              导入
            </ElButton>
            <ElButton v-auth="'system:user:import'" text @click="downloadTemplate">
              下载模板
            </ElButton>
            <input
              ref="fileInputRef"
              type="file"
              accept=".xlsx,.xls"
              class="hidden"
              @change="handleImport"
            />
          </ElSpace>
        </template>
      </ArtTableHeader>

      <!-- 表格 -->
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @selection-change="handleSelectionChange"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />

      <!-- 用户弹窗 -->
      <UserDialog
        v-model:visible="dialogVisible"
        :type="dialogType"
        :user-data="currentUserData"
        @success="refreshData"
      />
    </ElCard>

    <!-- 导入结果 -->
    <ElDialog v-model="importResultVisible" title="导入结果" width="560px" align-center>
      <div v-if="importResult">
        <ElAlert
          :type="importResult.failed ? 'warning' : 'success'"
          :closable="false"
          show-icon
          :title="`共 ${importResult.total} 行，成功 ${importResult.success} 行，失败 ${importResult.failed} 行`"
        />
        <ul v-if="importResult.errors?.length" class="import-errors">
          <li v-for="(msg, i) in importResult.errors" :key="i">{{ msg }}</li>
        </ul>
      </div>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import {
    fetchGetUserList,
    fetchDeleteUser,
    fetchChangeUserStatus,
    fetchResetUserPassword
  } from '@/api/system-manage'
  import UserSearch from './modules/user-search.vue'
  import UserDialog from './modules/user-dialog.vue'
  import { ElTag, ElMessageBox, ElMessage, ElSwitch } from 'element-plus'
  import { formatTime, downloadFile, uploadFile } from '@/utils/web-basic'
  import { checkPassword } from '@/utils/password'
  import defaultAvatar from '@/assets/images/user/avatar.webp'

  defineOptions({ name: 'User' })

  type UserListItem = Api.SystemManage.UserListItem

  // 弹窗相关
  const dialogType = ref<'add' | 'edit'>('add')

  // Excel 导出 / 导入
  const exporting = ref(false)
  const importing = ref(false)
  const fileInputRef = ref<HTMLInputElement>()
  const importResultVisible = ref(false)
  const importResult = ref<{ total: number; success: number; failed: number; errors: string[] }>()

  /** 导出条件与列表筛选保持一致 */
  const exportParams = () => ({
    username: searchForm.value.userName || undefined,
    mobile: searchForm.value.userPhone || undefined,
    status:
      searchForm.value.status === undefined || searchForm.value.status === ''
        ? undefined
        : Number(searchForm.value.status)
  })

  const handleExport = async () => {
    exporting.value = true
    try {
      await downloadFile('/api/v1/system/users/export', exportParams(), '用户列表.xlsx')
    } catch (err: any) {
      ElMessage.error(err?.message || '导出失败')
    } finally {
      exporting.value = false
    }
  }

  const triggerImport = () => fileInputRef.value?.click()

  const downloadTemplate = async () => {
    try {
      await downloadFile('/api/v1/system/users/import-template', {}, '用户导入模板.xlsx')
    } catch (err: any) {
      ElMessage.error(err?.message || '模板下载失败')
    }
  }

  const handleImport = async (e: Event) => {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    importing.value = true
    try {
      const res = await uploadFile<{ total: number; success: number; failed: number; errors: string[] }>(
        '/api/v1/system/users/import',
        file
      )
      importResult.value = res ?? { total: 0, success: 0, failed: 0, errors: [] }
      importResultVisible.value = true
      if (importResult.value.success > 0) refreshData()
    } catch (err: any) {
      ElMessage.error(err?.message || '导入失败')
    } finally {
      importing.value = false
    }
  }
  const dialogVisible = ref(false)
  const currentUserData = ref<Partial<UserListItem>>({})

  // 选中行
  const selectedRows = ref<UserListItem[]>([])
  const statusLoading = ref<string | number | null>(null)

  // 搜索表单
  const searchForm = ref<{
    userName?: string
    userPhone?: string
    status?: string
  }>({
    userName: undefined,
    userPhone: undefined,
    status: undefined
  })

  const {
    columns,
    columnChecks,
    data,
    loading,
    pagination,
    getData,
    replaceSearchParams,
    resetSearchParams,
    handleSizeChange,
    handleCurrentChange,
    refreshData
  } = useTable({
    core: {
      apiFn: fetchGetUserList,
      apiParams: {
        current: 1,
        size: 20
      },
      columnsFactory: () => [
        { type: 'selection' },
        { type: 'index', width: 60, label: '序号' },
        {
          prop: 'userInfo',
          label: '用户',
          width: 260,
          formatter: (row) =>
            h('div', { class: 'user flex-c' }, [
              h('img', {
                class: 'size-9.5 rounded-md object-cover',
                src: row.avatar || defaultAvatar,
                alt: row.userName
              }),
              h('div', { class: 'ml-2' }, [
                h('p', { class: 'user-name' }, row.userName),
                h('p', { class: 'email' }, row.userEmail || '-')
              ])
            ])
        },
        { prop: 'nickName', label: '昵称', minWidth: 110 },
        { prop: 'deptName', label: '部门', minWidth: 120 },
        {
          prop: 'userRoles',
          label: '角色',
          minWidth: 150,
          formatter: (row) => {
            const roles = (row.userRoles || []).filter((r: any) => typeof r === 'string')
            if (!roles.length) return h('span', { class: 'text-g-500' }, '未分配')
            return h(
              'div',
              { class: 'flex gap-1 flex-wrap' },
              roles.map((r: string) => h(ElTag, { type: 'info', size: 'small' }, () => r))
            )
          }
        },
        { prop: 'userPhone', label: '手机号', width: 130 },
        {
          prop: 'status',
          label: '状态',
          width: 100,
          formatter: (row) =>
            h(ElSwitch, {
              modelValue: Number(row.status),
              activeValue: 1,
              inactiveValue: 0,
              loading: statusLoading.value === row.id,
              onChange: (val: string | number | boolean) => toggleStatus(row, Number(val))
            })
        },
        { prop: 'createTime', label: '创建时间', width: 180, sortable: true, formatter: (row) => formatTime(row.createTime) },
        {
          prop: 'operation',
          label: '操作',
          width: 110,
          fixed: 'right',
          formatter: (row) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑用户', icon: 'ri:edit-2-line', auth: 'system:user:update' },
                { key: 'resetPwd', label: '重置密码', icon: 'ri:lock-password-line', auth: 'system:user:reset-pwd' },
                { key: 'delete', label: '删除用户', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'system:user:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMoreClick(item, row)
            })
        }
      ]
    }
  })

  const handleSearch = (params: Record<string, any>) => {
    replaceSearchParams(params)
    getData()
  }

  const showDialog = (type: 'add' | 'edit', row?: UserListItem) => {
    dialogType.value = type
    currentUserData.value = row || {}
    nextTick(() => {
      dialogVisible.value = true
    })
  }

  /** 删除（批量） */
  const deleteUsers = async (rows: UserListItem[]) => {
    if (!rows.length) return
    const names = rows.map((r) => r.userName).join('、')
    try {
      await ElMessageBox.confirm(`确定删除用户「${names}」吗？该操作不可恢复。`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteUser(rows.map((r) => r.id))
    refreshData()
  }

  /** 启用/停用 */
  const toggleStatus = async (row: UserListItem, status: number) => {
    statusLoading.value = row.id
    try {
      await fetchChangeUserStatus(row.id, status)
      row.status = String(status)
    } catch {
      /* 失败已提示，保持原状态 */
    } finally {
      statusLoading.value = null
    }
  }

  /** 行选择 */
  const handleSelectionChange = (selection: UserListItem[]) => {
    selectedRows.value = selection
  }

  /** 更多操作 */
  const handleMoreClick = async (item: ButtonMoreItem, row: UserListItem) => {
    switch (item.key) {
      case 'edit':
        showDialog('edit', row)
        break
      case 'resetPwd':
        await resetPassword(row)
        break
      case 'delete':
        await deleteUsers([row])
        break
    }
  }

  /** 重置密码 */
  const resetPassword = async (row: UserListItem) => {
    const { value } = await ElMessageBox.prompt(
      `请输入用户「${row.userName}」的新密码（留空则随机生成）`,
      '重置密码',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputPlaceholder: '留空自动生成强密码；手填需满足：大写+小写+数字，≥8 位',
        inputValidator: (val) => !val || !checkPassword(val)
      }
    )
    const res = await fetchResetUserPassword(row.id, value || undefined)
    ElMessageBox.alert(`新密码：${res?.password || value}`, '重置成功', { confirmButtonText: '我已记录' })
  }
</script>
<style scoped>
  .import-errors {
    max-height: 240px;
    margin: 12px 0 0;
    overflow: auto;
    font-size: 13px;
    line-height: 1.7;
    color: var(--el-color-danger);
  }
</style>
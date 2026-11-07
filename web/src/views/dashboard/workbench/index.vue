<!-- 工作台：顶部 4 张统计卡片 + 下方「待办 / 消息 / 通知」三合一表格 -->
<template>
  <div class="workbench art-full-height">
    <!-- 欢迎条 -->
    <ElCard class="mb-3">
      <div class="flex-cb flex-wrap gap-4">
        <div class="flex-c">
          <ElAvatar :size="52" :src="userStore.info.avatar || undefined">
            {{ (userStore.info.nickName || userStore.info.userName || 'U').slice(0, 1) }}
          </ElAvatar>
          <div class="ml-4">
            <p class="text-lg font-semibold">
              {{ greeting }}，{{ userStore.info.nickName || userStore.info.userName }}！
            </p>
            <p class="mt-1 text-sm text-g-500">
              {{ userStore.info.deptName || '未分配部门' }} ·
              <span v-for="(r, i) in userStore.info.roles || []" :key="r">
                {{ r }}<span v-if="i < (userStore.info.roles || []).length - 1">、</span>
              </span>
              <span v-if="!(userStore.info.roles || []).length">未分配角色</span>
            </p>
          </div>
        </div>
        <div class="flex-c gap-6">
          <div class="text-center">
            <p class="text-2xl font-semibold text-theme">{{ summary?.onlineCount ?? '-' }}</p>
            <p class="text-xs text-g-500">在线会话</p>
          </div>
          <div class="text-center">
            <p class="text-2xl font-semibold">{{ summary?.todayLogins ?? '-' }}</p>
            <p class="text-xs text-g-500">今日登录</p>
          </div>
          <div class="text-center">
            <p class="text-2xl font-semibold">{{ summary?.todayOpers ?? '-' }}</p>
            <p class="text-xs text-g-500">今日操作</p>
          </div>
        </div>
      </div>
    </ElCard>

    <ElAlert v-if="loadFailed" type="error" :closable="false" class="mb-3">
      <template #title>
        <span class="mr-2">概览数据加载失败，可能是网络或服务异常。</span>
        <ElButton link type="primary" :loading="loading" @click="loadData">重新加载</ElButton>
      </template>
    </ElAlert>

    <!-- 顶部四张统计卡片 -->
    <ElRow :gutter="12">
      <ElCol v-for="card in statCards" :key="card.label" :xs="12" :sm="12" :md="6">
        <ElCard class="mb-3 stat-card">
          <div class="flex-cb">
            <div class="min-w-0 flex-1">
              <p class="text-sm text-g-500">{{ card.label }}</p>
              <div class="stat-value">
                <ElSkeleton v-if="loading && !summary" animated>
                  <template #template>
                    <ElSkeletonItem variant="text" style="width: 55%; height: 30px" />
                  </template>
                </ElSkeleton>
                <p v-else class="text-2xl font-semibold" :class="{ 'text-g-400': !summary }">
                  {{ card.value }}
                </p>
              </div>
              <p class="mt-1 truncate text-xs text-g-400">{{ card.tip }}</p>
            </div>
            <div class="stat-icon" :style="{ background: card.bg, color: card.color }">
              <ArtSvgIcon :icon="card.icon" class="text-2xl" />
            </div>
          </div>
        </ElCard>
      </ElCol>
    </ElRow>

    <!-- 待办 / 消息 / 通知：一个卡片内切换 -->
    <ElCard class="my-work-card">
      <ElTabs v-model="activeTab" @tab-change="onTabChange">
        <ElTabPane name="todo">
          <template #label>
            <span class="tab-label">
              我的待办
              <ElBadge v-if="todoSummary.pending > 0" :value="todoSummary.pending" type="danger" />
            </span>
          </template>

          <div class="flex-cb mb-3">
            <ElSpace wrap>
              <ElButton type="primary" @click="todoDialog = true" v-ripple>
                <ArtSvgIcon icon="ri:add-line" class="mr-1" />新建待办
              </ElButton>
              <ElRadioGroup v-model="todoScope" @change="loadTodos">
                <ElRadioButton value="pending">待处理</ElRadioButton>
                <ElRadioButton value="completed">已完成</ElRadioButton>
              </ElRadioGroup>
              <ElTag v-if="todoSummary.overdue > 0" type="danger" effect="light">
                逾期 {{ todoSummary.overdue }}
              </ElTag>
            </ElSpace>
            <ElButton link type="primary" @click="loadTodos">刷新</ElButton>
          </div>

          <ElTable :data="todos" v-loading="todoLoading" size="default" empty-text="暂无待办">
            <ElTableColumn label="待办事项" min-width="240">
              <template #default="{ row }">
                <span :class="row.status === 2 ? 'text-g-400 line-through' : ''">{{ row.title }}</span>
                <ElTag v-if="row.bizType" size="small" type="info" effect="plain" class="ml-2">
                  {{ row.bizType }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="优先级" width="100">
              <template #default="{ row }">
                <ElTag :type="priorityOf(row.priority).type" size="small" effect="light">
                  {{ priorityOf(row.priority).label }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="截止时间" width="180">
              <template #default="{ row }">
                <span :class="isOverdue(row) ? 'text-danger' : ''">
                  {{ row.dueTime ? formatTime(row.dueTime) : '-' }}
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="来源" width="100">
              <template #default="{ row }">
                {{ ({ 1: '手工', 2: '系统推送', 3: '业务系统' } as any)[row.source || 1] || '手工' }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <ElButton
                  v-if="row.status === 1"
                  link
                  type="primary"
                  @click="completeTodo(row)"
                >
                  完成
                </ElButton>
                <ElButton v-else link type="primary" @click="reopenTodo(row)">恢复</ElButton>
                <ElButton v-if="row.linkUrl" link type="primary" @click="openLink(row)">查看业务</ElButton>
                <ElButton link type="danger" @click="removeTodo(row)">删除</ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>

        <ElTabPane name="message">
          <template #label>
            <span class="tab-label">
              我的消息
              <ElBadge v-if="workSummary.unreadMessages > 0" :value="workSummary.unreadMessages" type="danger" />
            </span>
          </template>

          <div class="flex-cb mb-3">
            <ElButton @click="markAllRead">全部已读</ElButton>
            <ElButton link type="primary" @click="loadMessages">刷新</ElButton>
          </div>

          <ElTable :data="messages" v-loading="messageLoading" size="default" empty-text="暂无消息">
            <ElTableColumn label="标题" min-width="260">
              <template #default="{ row }">
                <span class="c-p" :class="row.isRead ? 'text-g-500' : 'font-medium'" @click="openMessage(row)">
                  {{ row.title }}
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="类型" width="110">
              <template #default="{ row }">
                {{ ({ 1: '系统', 2: '公告', 3: '任务' } as any)[row.type || 1] || '系统' }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="状态" width="100">
              <template #default="{ row }">
                <ElTag :type="row.isRead ? 'info' : 'danger'" size="small" effect="light">
                  {{ row.isRead ? '已读' : '未读' }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="时间" width="180">
              <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <ElButton v-if="!row.isRead" link type="primary" @click="readMessage(row)">标记已读</ElButton>
                <ElButton link type="danger" @click="removeMessage(row)">删除</ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>

        <ElTabPane name="notice">
          <template #label>
            <span class="tab-label">
              通知公告
              <ElBadge v-if="workSummary.unreadNotices > 0" :value="workSummary.unreadNotices" type="danger" />
            </span>
          </template>

          <div class="flex-cb mb-3">
            <span class="text-xs text-g-500">已发布的公告会出现在这里，点标题查看详情并自动标记已读</span>
            <ElButton link type="primary" @click="loadNotices">刷新</ElButton>
          </div>

          <ElTable :data="notices" v-loading="noticeLoading" size="default" empty-text="暂无通知">
            <ElTableColumn label="标题" min-width="280">
              <template #default="{ row }">
                <span class="flex-c gap-2 c-p" @click="openNotice(row)">
                  <span v-if="!row.isRead" class="inline-block size-2 rounded-full bg-danger"></span>
                  <span :class="row.isRead ? 'text-g-500' : 'font-medium'">{{ row.title }}</span>
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="类型" width="100">
              <template #default="{ row }">
                {{ ({ 1: '通知', 2: '公告', 3: '维护' } as any)[row.type || 1] || '通知' }}
              </template>
            </ElTableColumn>
            <ElTableColumn label="级别" width="100">
              <template #default="{ row }">
                <ElTag :type="levelOf(row.level).type" size="small" effect="light">
                  {{ levelOf(row.level).label }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="发布人" width="140">
              <template #default="{ row }">{{ row.publisher || '系统' }}</template>
            </ElTableColumn>
            <ElTableColumn label="发布时间" width="180">
              <template #default="{ row }">{{ formatTime(row.publishTime || row.createTime) }}</template>
            </ElTableColumn>
          </ElTable>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <!-- 待办详情/新建 -->
    <ElDialog v-model="todoDialog" title="新建待办" width="560px" align-center destroy-on-close>
      <ElForm ref="todoFormRef" :model="todoForm" :rules="todoRules" label-width="90px">
        <ElFormItem label="标题" prop="title">
          <ElInput v-model.trim="todoForm.title" maxlength="200" show-word-limit />
        </ElFormItem>
        <ElFormItem label="优先级">
          <ElRadioGroup v-model="todoForm.priority">
            <ElRadio :value="1">低</ElRadio>
            <ElRadio :value="2">普通</ElRadio>
            <ElRadio :value="3">高</ElRadio>
            <ElRadio :value="4">紧急</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElFormItem label="截止时间">
          <ElDatePicker
            v-model="todoForm.dueTime"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="可不填"
            class="!w-full"
          />
        </ElFormItem>
        <ElFormItem label="说明">
          <ElInput v-model="todoForm.content" type="textarea" :rows="3" maxlength="2000" show-word-limit />
        </ElFormItem>
        <ElFormItem label="业务链接">
          <ElInput v-model.trim="todoForm.linkUrl" placeholder="可选：/system/user?id=1" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="todoDialog = false">取消</ElButton>
        <ElButton type="primary" :loading="todoSubmitting" @click="submitTodo">确定</ElButton>
      </template>
    </ElDialog>

    <!-- 消息 / 通知详情 -->
    <ElDialog v-model="detailVisible" :title="detail.title || '详情'" width="620px" align-center>
      <div class="detail-meta">
        <ElTag size="small" type="info">{{ detail.kind }}</ElTag>
        <span v-if="detail.time">{{ detail.time }}</span>
        <span v-if="detail.publisher">发布人：{{ detail.publisher }}</span>
      </div>
      <div class="detail-content">{{ detail.content || '（无正文）' }}</div>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { useRouter, useRoute } from 'vue-router'
  import { useUserStore } from '@/store/modules/user'
  import { fetchGetDashboardSummary } from '@/api/business'
  import {
    fetchGetMyTodos,
    fetchCreateMyTodo,
    fetchUpdateMyTodoStatus,
    fetchDeleteMyTodo,
    fetchMyTodoSummary,
    fetchGetMyMessages,
    fetchMarkMessageRead,
    fetchMarkAllMessagesRead,
    fetchDeleteMyMessage,
    fetchGetMyNotices,
    fetchMarkNoticeRead,
    fetchMySummary
  } from '@/api/workbench'
  import { formatSize, formatTime } from '@/utils/web-basic'
  import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'Workbench' })

  const router = useRouter()
  const route = useRoute()
  const userStore = useUserStore()

  const loading = ref(false)
  const loadFailed = ref(false)
  const summary = ref<WebBasic.DashboardSummary | null>(null)
  const workSummary = reactive<WebBasic.WorkbenchSummary>({
    unreadMessages: 0,
    pendingTodos: 0,
    overdueTodos: 0,
    unreadNotices: 0
  })
  const todoSummary = reactive<WebBasic.TodoSummary>({ pending: 0, overdue: 0, completed: 0 })

  const greeting = computed(() => {
    const h = new Date().getHours()
    if (h < 6) return '凌晨好'
    if (h < 12) return '早上好'
    if (h < 14) return '中午好'
    if (h < 18) return '下午好'
    return '晚上好'
  })

  const statCards = computed(() => [
    {
      label: '用户总数',
      value: summary.value?.users ?? '-',
      tip: `${summary.value?.onlineCount ?? 0} 个在线会话`,
      icon: 'ri:user-3-line',
      color: '#409eff',
      bg: 'rgba(64,158,255,0.12)'
    },
    {
      label: '角色 / 部门',
      value: `${summary.value?.roles ?? '-'} / ${summary.value?.depts ?? '-'}`,
      tip: `${summary.value?.posts ?? 0} 个岗位 · ${summary.value?.tenants ?? 0} 个租户`,
      icon: 'ri:team-line',
      color: '#67c23a',
      bg: 'rgba(103,194,58,0.12)'
    },
    {
      label: '文件总量',
      value: summary.value?.files ?? '-',
      tip: `占用 ${formatSize(summary.value?.fileSize ?? 0)}`,
      icon: 'ri:folder-3-line',
      color: '#e6a23c',
      bg: 'rgba(230,162,60,0.12)'
    },
    {
      label: '今日登录',
      value: summary.value?.todayLogins ?? '-',
      tip: `今日操作 ${summary.value?.todayOpers ?? 0} 次`,
      icon: 'ri:line-chart-line',
      color: '#f56c6c',
      bg: 'rgba(245,108,108,0.12)'
    }
  ])

  const loadData = async () => {
    loading.value = true
    loadFailed.value = false
    try {
      summary.value = await fetchGetDashboardSummary()
    } catch {
      loadFailed.value = true
    } finally {
      loading.value = false
    }
  }

  /* ------------------------------ 待办 / 消息 / 通知 ------------------------------ */
  const activeTab = ref<'todo' | 'message' | 'notice'>('todo')
  const todoScope = ref<'pending' | 'completed'>('pending')
  const todos = ref<WebBasic.Todo[]>([])
  const messages = ref<WebBasic.Message[]>([])
  const notices = ref<WebBasic.MyNotice[]>([])
  const todoLoading = ref(false)
  const messageLoading = ref(false)
  const noticeLoading = ref(false)

  type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

  const priorityOf = (p?: number): { label: string; type: TagType } =>
    ({
      1: { label: '低', type: 'info' },
      2: { label: '普通', type: 'success' },
      3: { label: '高', type: 'warning' },
      4: { label: '紧急', type: 'danger' }
    } as Record<number, { label: string; type: TagType }>)[p || 2] || { label: '普通', type: 'success' }

  const levelOf = (l?: number): { label: string; type: TagType } =>
    ({
      1: { label: '普通', type: 'info' },
      2: { label: '重要', type: 'warning' },
      3: { label: '紧急', type: 'danger' }
    } as Record<number, { label: string; type: TagType }>)[l || 1] || { label: '普通', type: 'info' }

  const isOverdue = (row: WebBasic.Todo) =>
    row.status !== 2 && !!row.dueTime && new Date(row.dueTime).getTime() < Date.now()

  const loadWorkSummary = async () => {
    try {
      Object.assign(workSummary, (await fetchMySummary()) || {})
      Object.assign(todoSummary, (await fetchMyTodoSummary()) || {})
    } catch {
      /* 统计失败不影响列表 */
    }
  }

  const loadTodos = async () => {
    todoLoading.value = true
    try {
      const res = await fetchGetMyTodos({
        page: 1,
        pageSize: 10,
        status: todoScope.value === 'pending' ? 1 : 2
      })
      todos.value = res?.list || []
      await loadWorkSummary()
    } finally {
      todoLoading.value = false
    }
  }

  const loadMessages = async () => {
    messageLoading.value = true
    try {
      const res = await fetchGetMyMessages({ page: 1, pageSize: 10 })
      messages.value = res?.list || []
      await loadWorkSummary()
    } finally {
      messageLoading.value = false
    }
  }

  const loadNotices = async () => {
    noticeLoading.value = true
    try {
      const res = await fetchGetMyNotices({ page: 1, pageSize: 10 })
      notices.value = res?.list || []
      await loadWorkSummary()
    } finally {
      noticeLoading.value = false
    }
  }

  const onTabChange = (name: string | number) => {
    if (name === 'todo') loadTodos()
    else if (name === 'message') loadMessages()
    else loadNotices()
  }

  /* 待办操作 */
  const completeTodo = async (row: WebBasic.Todo) => {
    await fetchUpdateMyTodoStatus(row.id, 2)
    loadTodos()
  }

  const reopenTodo = async (row: WebBasic.Todo) => {
    await fetchUpdateMyTodoStatus(row.id, 1)
    loadTodos()
  }

  const openLink = (row: WebBasic.Todo) => {
    if (!row.linkUrl) return
    if (row.linkUrl.startsWith('/')) window.location.hash = `#${row.linkUrl}`
    else window.open(row.linkUrl, '_blank')
  }

  const removeTodo = async (row: WebBasic.Todo) => {
    try {
      await ElMessageBox.confirm(`确定删除待办「${row.title}」吗？`, '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteMyTodo(row.id)
    loadTodos()
  }

  /* 新建待办 */
  const todoDialog = ref(false)
  const todoSubmitting = ref(false)
  const todoFormRef = ref<FormInstance>()
  const todoForm = reactive({ title: '', content: '', priority: 2, dueTime: '', linkUrl: '' })
  const todoRules: FormRules = {
    title: [{ required: true, message: '请输入待办标题', trigger: 'blur' }]
  }

  watch(todoDialog, (v) => {
    if (v) Object.assign(todoForm, { title: '', content: '', priority: 2, dueTime: '', linkUrl: '' })
  })

  const submitTodo = async () => {
    const valid = await todoFormRef.value?.validate().catch(() => false)
    if (!valid) return
    todoSubmitting.value = true
    try {
      await fetchCreateMyTodo({ ...todoForm, dueTime: todoForm.dueTime || undefined })
      todoDialog.value = false
      todoScope.value = 'pending'
      loadTodos()
    } finally {
      todoSubmitting.value = false
    }
  }

  /* 消息操作 */
  const readMessage = async (row: WebBasic.Message) => {
    await fetchMarkMessageRead(row.id)
    loadMessages()
  }

  const markAllRead = async () => {
    await fetchMarkAllMessagesRead()
    loadMessages()
  }

  const removeMessage = async (row: WebBasic.Message) => {
    try {
      await ElMessageBox.confirm('确定删除这条消息吗？', '删除确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
    await fetchDeleteMyMessage(row.id)
    loadMessages()
  }

  const openMessage = (row: WebBasic.Message) => {
    detail.value = {
      title: row.title,
      content: row.content,
      kind: ({ 1: '系统消息', 2: '公告提醒', 3: '任务提醒' } as any)[row.type || 1] || '系统消息',
      time: formatTime(row.createTime)
    }
    detailVisible.value = true
    if (!row.isRead) fetchMarkMessageRead(row.id).then(loadMessages).catch(() => {})
  }

  /* 通知操作 */
  const openNotice = (row: WebBasic.MyNotice) => {
    detail.value = {
      title: row.title,
      content: row.content,
      kind: ({ 1: '通知', 2: '公告', 3: '维护' } as any)[row.type || 1] || '通知',
      time: formatTime(row.publishTime || row.createTime),
      publisher: row.publisher
    }
    detailVisible.value = true
    if (!row.isRead) fetchMarkNoticeRead(row.id).then(loadNotices).catch(() => {})
  }

  const detailVisible = ref(false)
  const detail = ref<{ title?: string; content?: string; kind?: string; time?: string; publisher?: string }>({})

  onMounted(() => {
    // 顶栏铃铛点「查看全部」会带 ?tab=xxx 进来，直接定位到对应页签
    const tab = String(route.query.tab || '')
    if (tab === 'todo' || tab === 'message' || tab === 'notice') activeTab.value = tab
    loadData()
    loadWorkSummary()
    onTabChange(activeTab.value)
  })
</script>

<style scoped>
  .stat-value {
    display: flex;
    align-items: center;
    min-height: 38px;
    margin-top: 6px;
  }

  .stat-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border-radius: 12px;
  }

  .tab-label {
    display: inline-flex;
    gap: 6px;
    align-items: center;
  }

  .detail-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: center;
    margin-bottom: 12px;
    font-size: 12px;
    color: var(--art-gray-600);
  }

  .detail-content {
    font-size: 14px;
    line-height: 1.9;
    white-space: pre-wrap;
    color: var(--art-gray-800);
  }
</style>

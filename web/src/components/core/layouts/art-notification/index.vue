<!-- 通知中心（顶栏铃铛）：整合「通知 / 消息 / 待办」三类真实数据 -->
<template>
  <div
    class="art-notification-panel art-card-sm !shadow-xl"
    :style="{
      transform: show ? 'scaleY(1)' : 'scaleY(0.9)',
      opacity: show ? 1 : 0
    }"
    v-show="visible"
    @click.stop
  >
    <div class="flex-cb px-3.5 mt-3.5">
      <span class="text-base font-medium text-g-800">{{ $t('notice.title') }}</span>
      <span
        class="text-xs text-g-800 px-1.5 py-1 c-p select-none rounded hover:bg-g-200"
        @click="markAllRead"
      >
        {{ $t('notice.btnRead') }}
      </span>
    </div>

    <ul class="box-border flex items-end w-full h-12.5 px-3.5 border-b-d">
      <li
        v-for="(item, index) in tabList"
        :key="item.key"
        class="h-12 leading-12 mr-5 overflow-hidden text-[13px] text-g-700 c-p select-none"
        :class="{ 'bar-active': tabIndex === index }"
        @click="changeTab(index)"
      >
        {{ item.name }} ({{ item.count }})
      </li>
    </ul>

    <div class="w-full h-[calc(100%-95px)]">
      <div class="h-[calc(100%-60px)] overflow-y-scroll scrollbar-thin">
        <!-- 通知 -->
        <ul v-show="tabIndex === 0">
          <li
            v-for="item in notices"
            :key="item.id"
            class="box-border flex-c px-3.5 py-3.5 c-p last:border-b-0 hover:bg-g-200/60"
            @click="goToTab('notice')"
          >
            <div class="size-9 leading-9 text-center rounded-lg flex-cc bg-theme/10">
              <ArtSvgIcon class="text-lg !bg-transparent text-theme" icon="ri:notification-3-line" />
            </div>
            <div class="w-[calc(100%-45px)] ml-3.5">
              <h4 class="text-sm font-normal leading-5.5 text-g-900 truncate">{{ item.title }}</h4>
              <p class="mt-1.5 text-xs text-g-500">{{ formatTime(item.publishTime || item.createTime) }}</p>
            </div>
          </li>
        </ul>

        <!-- 消息 -->
        <ul v-show="tabIndex === 1">
          <li
            v-for="item in messages"
            :key="item.id"
            class="box-border flex-c px-3.5 py-3.5 c-p last:border-b-0 hover:bg-g-200/60"
            @click="goToTab('message')"
          >
            <div class="size-9 leading-9 text-center rounded-lg flex-cc bg-success/10">
              <ArtSvgIcon class="text-lg !bg-transparent text-success" icon="ri:message-2-line" />
            </div>
            <div class="w-[calc(100%-45px)] ml-3.5">
              <h4 class="text-sm font-normal leading-5.5 text-g-900 truncate">
                {{ item.title }}
                <span v-if="item.isRead === 0" class="ml-1 text-xs text-danger">未读</span>
              </h4>
              <p class="mt-1.5 text-xs text-g-500">{{ formatTime(item.createTime) }}</p>
            </div>
          </li>
        </ul>

        <!-- 待办 -->
        <ul v-show="tabIndex === 2">
          <li
            v-for="item in todos"
            :key="item.id"
            class="box-border px-3.5 py-3.5 c-p last:border-b-0 hover:bg-g-200/60"
            @click="goToTab('todo')"
          >
            <h4 class="text-sm font-normal leading-5.5 text-g-900 truncate">
              <span
                v-if="item.priority >= 3"
                class="inline-block mr-1.5 px-1 text-xs rounded bg-danger/10 text-danger"
                >{{ item.priority === 4 ? '紧急' : '高' }}</span
              >
              {{ item.title }}
            </h4>
            <p class="mt-1.5 text-xs text-g-500">
              {{ item.dueTime ? `截止 ${formatTime(item.dueTime)}` : '无截止时间' }}
            </p>
          </li>
        </ul>

        <!-- 空状态 -->
        <div
          v-show="currentEmpty"
          class="relative top-25 h-full text-g-500 text-center !bg-transparent"
        >
          <ArtSvgIcon icon="system-uicons:inbox" class="text-5xl" />
          <p class="mt-3.5 text-xs !bg-transparent"
            >{{ $t('notice.text[0]') }}{{ tabList[tabIndex].name }}</p
          >
        </div>
      </div>

      <div class="relative box-border w-full px-3.5">
        <ElButton class="w-full mt-3" @click="viewAll" v-ripple>
          {{ $t('notice.viewAll') }}
        </ElButton>
      </div>
    </div>

    <div class="h-25"></div>
  </div>
</template>

<script setup lang="ts">
  import { useRouter } from 'vue-router'
  import { ElButton } from 'element-plus'
  import { formatTime } from '@/utils/web-basic'
  import {
    fetchGetMyMessages,
    fetchGetMyNotices,
    fetchGetMyTodos,
    fetchMarkAllMessagesRead,
    fetchMySummary
  } from '@/api/workbench'

  defineOptions({ name: 'ArtNotification' })

  interface Props {
    value: boolean
  }
  const props = withDefaults(defineProps<Props>(), { value: false })
  const show = computed(() => props.value)
  const visible = computed(() => props.value)
  const router = useRouter()

  const notices = ref<WebBasic.MyNotice[]>([])
  const messages = ref<WebBasic.Message[]>([])
  const todos = ref<WebBasic.Todo[]>([])
  const summary = reactive<WebBasic.WorkbenchSummary>({
    unreadMessages: 0,
    pendingTodos: 0,
    overdueTodos: 0,
    unreadNotices: 0
  })
  const tabIndex = ref(0)

  const tabList = computed(() => [
    { key: 'notice', name: '通知', count: summary.unreadNotices },
    { key: 'message', name: '消息', count: summary.unreadMessages },
    { key: 'todo', name: '待办', count: summary.pendingTodos }
  ])

  const currentEmpty = computed(() => {
    if (tabIndex.value === 0) return notices.value.length === 0
    if (tabIndex.value === 1) return messages.value.length === 0
    return todos.value.length === 0
  })

  /** 拉取三类数据（都只取最近 5 条，点「查看全部」进对应页面） */
  const load = async () => {
    try {
      const [sum, noticeRes, msgRes, todoRes] = await Promise.all([
        fetchMySummary().catch(() => null),
        fetchGetMyNotices({ page: 1, pageSize: 5 }).catch(() => null),
        fetchGetMyMessages({ page: 1, pageSize: 5 }).catch(() => null),
        fetchGetMyTodos({ page: 1, pageSize: 5, status: 1 }).catch(() => null)
      ])
      if (sum) Object.assign(summary, sum)
      notices.value = (noticeRes?.list || []) as WebBasic.MyNotice[]
      messages.value = (msgRes?.list || []) as WebBasic.Message[]
      todos.value = (todoRes?.list || []) as WebBasic.Todo[]
    } catch {
      /* 顶栏面板失败不影响主流程 */
    }
  }

  // 每次打开面板刷新一次，保证数据是新的
  watch(
    () => props.value,
    (v) => {
      if (v) load()
    },
    { immediate: true }
  )

  const changeTab = (index: number) => {
    tabIndex.value = index
  }

  /** 三个分类都收敛到工作台的页签里，不再单独开页面 */
  const tabOf = { notice: 'notice', message: 'message', todo: 'todo' } as const

  const goToTab = (key: keyof typeof tabOf) => {
    router.push({ path: '/dashboard', query: { tab: tabOf[key] } })
  }

  const viewAll = () => {
    const key = tabList.value[tabIndex.value].key as keyof typeof tabOf
    goToTab(key)
  }

  /** 一键已读：把未读站内信全部标记为已读（通知按打开才已读，避免“已读但没看”） */
  const markAllRead = async () => {
    if (summary.unreadMessages > 0) {
      try {
        await fetchMarkAllMessagesRead()
      } catch {
        return
      }
    }
    await load()
  }
</script>

<style scoped>
  @reference '@styles/core/tailwind.css';

  /* 通知面板：固定在小卡片尺寸并贴着顶栏右上角，不允许撑满整宽 */
  .art-notification-panel {
    @apply absolute
    top-14.5
    right-5
    w-90
    h-125
    overflow-hidden
    transition-all
    duration-300
    origin-top
    will-change-[top,left]
    max-[640px]:top-[65px]
    max-[640px]:right-0
    max-[640px]:w-full
    max-[640px]:h-[80vh];
  }

  .bar-active {
    color: var(--theme-color) !important;
    border-bottom: 2px solid var(--theme-color);
  }

  .scrollbar-thin::-webkit-scrollbar {
    width: 5px !important;
  }

  .dark .scrollbar-thin::-webkit-scrollbar-track {
    background-color: var(--default-box-color);
  }

  .dark .scrollbar-thin::-webkit-scrollbar-thumb {
    background-color: #222 !important;
  }
</style>

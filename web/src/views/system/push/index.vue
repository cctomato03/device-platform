<!-- 消息与待办：后台手工发起（也用于联调业务系统对接） -->
<template>
  <div class="push-page art-full-height">
    <ElRow :gutter="16">
      <ElCol :xs="24" :lg="12">
        <ElCard shadow="never">
          <template #header>
            <div class="flex-cb">
              <span class="font-medium">推送站内信</span>
              <ElTag size="small" type="info" effect="plain">用户端：工作中心 → 消息中心</ElTag>
            </div>
          </template>

          <ElForm ref="msgFormRef" :model="msgForm" :rules="msgRules" label-width="90px">
            <ElFormItem label="接收人">
              <ElSelect
                v-model="msgForm.userIds"
                multiple
                filterable
                collapse-tags
                collapse-tags-tooltip
                placeholder="不选表示推送给全部启用用户"
                class="!w-full"
              >
                <ElOption
                  v-for="u in users"
                  :key="u.id"
                  :label="`${u.nickname || u.username}（${u.username}）`"
                  :value="u.id"
                />
              </ElSelect>
            </ElFormItem>
            <ElFormItem label="标题" prop="title">
              <ElInput v-model.trim="msgForm.title" maxlength="200" show-word-limit />
            </ElFormItem>
            <ElFormItem label="内容">
              <ElInput v-model="msgForm.content" type="textarea" :rows="4" maxlength="20000" show-word-limit />
            </ElFormItem>
            <ElFormItem label="类型">
              <ElSelect v-model="msgForm.type" class="!w-40">
                <ElOption label="系统消息" :value="1" />
                <ElOption label="公告提醒" :value="2" />
                <ElOption label="任务提醒" :value="3" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem label="幂等键">
              <ElInput v-model.trim="msgForm.bizKey" placeholder="可选：同一 bizKey 不会重复推送" />
            </ElFormItem>
            <ElFormItem>
              <ElButton type="primary" :loading="msgSending" v-auth="'system:push:create'" @click="sendMessage">
                发送
              </ElButton>
              <ElButton @click="resetMsg">重置</ElButton>
            </ElFormItem>
          </ElForm>

          <ElAlert
            v-if="lastResult"
            :type="lastResult.ok ? 'success' : 'error'"
            :closable="false"
            show-icon
            :title="lastResult.text"
          />
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :lg="12">
        <ElCard shadow="never">
          <template #header>
            <div class="flex-cb">
              <span class="font-medium">派发待办</span>
              <ElTag size="small" type="info" effect="plain">用户端：工作中心 → 待办中心</ElTag>
            </div>
          </template>

          <ElForm ref="todoFormRef" :model="todoForm" :rules="todoRules" label-width="90px">
            <ElFormItem label="负责人" prop="userId">
              <ElSelect v-model="todoForm.userId" filterable placeholder="选择用户" class="!w-full">
                <ElOption
                  v-for="u in users"
                  :key="u.id"
                  :label="`${u.nickname || u.username}（${u.username}）`"
                  :value="u.id"
                />
              </ElSelect>
            </ElFormItem>
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
                class="!w-full"
              />
            </ElFormItem>
            <ElFormItem label="说明">
              <ElInput v-model="todoForm.content" type="textarea" :rows="3" maxlength="2000" show-word-limit />
            </ElFormItem>
            <ElFormItem label="业务链接">
              <ElInput v-model.trim="todoForm.linkUrl" placeholder="如 /order/detail?id=1001" />
            </ElFormItem>
            <ElFormItem label="幂等键">
              <ElInput v-model.trim="todoForm.bizKey" placeholder="可选：业务系统重试不会重复建单" />
            </ElFormItem>
            <ElFormItem>
              <ElButton type="primary" :loading="todoSending" v-auth="'system:push:create'" @click="sendTodo">
                派发
              </ElButton>
              <ElButton @click="resetTodo">重置</ElButton>
            </ElFormItem>
          </ElForm>

          <ElAlert
            type="info"
            :closable="false"
            show-icon
            title="业务系统对接"
            description="外部系统可直接调用 POST /api/v1/open/messages 与 /api/v1/open/todos（请求头 X-API-KEY），字段与上面表单一致。"
          />
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchPushMessages, fetchPushTodo } from '@/api/workbench'
  import { fetchGetUserList } from '@/api/system-manage'
  import { ElTag, ElMessage, type FormInstance, type FormRules } from 'element-plus'

  defineOptions({ name: 'MessagePush' })

  const users = ref<Array<{ id: string; username: string; nickname?: string }>>([])
  const lastResult = ref<{ ok: boolean; text: string } | null>(null)

  const loadUsers = async () => {
    try {
      const res = await fetchGetUserList({ current: 1, size: 200 } as any)
      users.value = (res?.records || []).map((u: any) => ({
        id: u.id,
        username: u.username,
        nickname: u.nickname
      }))
    } catch {
      users.value = []
    }
  }

  /* ------------------------------ 站内信 ------------------------------ */
  const msgFormRef = ref<FormInstance>()
  const msgSending = ref(false)
  const msgForm = reactive({
    userIds: [] as string[],
    title: '',
    content: '',
    type: 1,
    bizKey: ''
  })
  const msgRules: FormRules = {
    title: [{ required: true, message: '请输入标题', trigger: 'blur' }]
  }

  const resetMsg = () => {
    Object.assign(msgForm, { userIds: [], title: '', content: '', type: 1, bizKey: '' })
    lastResult.value = null
  }

  const sendMessage = async () => {
    const valid = await msgFormRef.value?.validate().catch(() => false)
    if (!valid) return
    msgSending.value = true
    try {
      const res = await fetchPushMessages({
        userIds: msgForm.userIds.length ? msgForm.userIds : undefined,
        title: msgForm.title,
        content: msgForm.content,
        type: msgForm.type,
        bizKey: msgForm.bizKey || undefined
      })
      const created = res?.created ?? 0
      lastResult.value = {
        ok: true,
        text: created > 0 ? `已推送 ${created} 条站内信` : '没有新增推送（幂等键重复或没有可推送用户）'
      }
      ElMessage.success(lastResult.value.text)
    } catch (e: any) {
      lastResult.value = { ok: false, text: e?.message || '推送失败' }
    } finally {
      msgSending.value = false
    }
  }

  /* ------------------------------ 待办 ------------------------------ */
  const todoFormRef = ref<FormInstance>()
  const todoSending = ref(false)
  const todoForm = reactive({
    userId: '',
    title: '',
    content: '',
    priority: 2,
    dueTime: '',
    linkUrl: '',
    bizKey: ''
  })
  const todoRules: FormRules = {
    userId: [{ required: true, message: '请选择负责人', trigger: 'change' }],
    title: [{ required: true, message: '请输入待办标题', trigger: 'blur' }]
  }

  const resetTodo = () => {
    Object.assign(todoForm, {
      userId: '',
      title: '',
      content: '',
      priority: 2,
      dueTime: '',
      linkUrl: '',
      bizKey: ''
    })
  }

  const sendTodo = async () => {
    const valid = await todoFormRef.value?.validate().catch(() => false)
    if (!valid) return
    todoSending.value = true
    try {
      await fetchPushTodo({ ...todoForm, dueTime: todoForm.dueTime || undefined, bizKey: todoForm.bizKey || undefined })
      ElMessage.success('待办已派发')
      resetTodo()
    } finally {
      todoSending.value = false
    }
  }

  onMounted(loadUsers)
</script>

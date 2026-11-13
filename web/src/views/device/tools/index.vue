<!-- 排查工具：ping / traceroute / tcpdump / netstat -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="8">
        <ElCard class="mb-3" header="工具参数">
          <ElForm label-width="90px">
            <ElFormItem label="工具">
              <ElSelect v-model="form.action" style="width: 100%">
                <ElOption label="ping（连通性）" value="ping" :disabled="!meta.ping" />
                <ElOption label="traceroute（路径）" value="traceroute" :disabled="!meta.traceroute" />
                <ElOption label="netstat（连接）" value="netstat" :disabled="!meta.netstat" />
                <ElOption label="tcpdump（抓包）" value="tcpdump" :disabled="!meta.tcpdump" />
              </ElSelect>
            </ElFormItem>

            <ElFormItem v-if="form.action !== 'netstat'" :label="form.action === 'tcpdump' ? '过滤表达式' : '目标地址'">
              <ElInput
                v-model="form.target"
                :placeholder="form.action === 'tcpdump' ? '例如 host 10.0.0.1 and port 80' : '例如 8.8.8.8 或 www.baidu.com'"
              />
            </ElFormItem>

            <ElFormItem v-if="form.action === 'tcpdump'" label="网卡">
              <ElSelect v-model="form.iface" clearable style="width: 100%">
                <ElOption v-for="i in interfaces" :key="i.name" :label="i.name" :value="i.name" />
              </ElSelect>
            </ElFormItem>

            <ElFormItem v-if="form.action === 'netstat'" label="端口">
              <ElInputNumber v-model="form.port" :min="0" :max="65535" controls-position="right" />
            </ElFormItem>

            <ElFormItem v-if="form.action !== 'netstat'" label="次数/包数">
              <ElInputNumber v-model="form.count" :min="1" :max="200" controls-position="right" />
            </ElFormItem>

            <ElFormItem>
              <ElButton v-auth="'device:tool:run'" type="primary" :loading="running" @click="onRun">执行</ElButton>
              <ElButton @click="output = ''">清空输出</ElButton>
            </ElFormItem>
          </ElForm>

          <ElAlert v-if="!meta.enabled" type="warning" :closable="false">
            排查工具已在配置中关闭（system.tools_enabled=false）。
          </ElAlert>
          <ElAlert v-else-if="form.action === 'tcpdump' && !meta.root" type="warning" :closable="false">
            tcpdump 需要 root 权限，请以 root 运行平台服务（或给二进制加 CAP_NET_RAW）。
          </ElAlert>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="16">
        <ElCard class="mb-3">
          <template #header>
            <div class="flex-cb">
              <span>执行结果</span>
              <ElSpace>
                <ElTag v-if="result?.command" type="info" size="small">{{ result.command }}</ElTag>
                <ElTag v-if="result" :type="result.exitCode === 0 ? 'success' : 'danger'" size="small">
                  exit {{ result.exitCode }} · {{ result.costMS }}ms
                </ElTag>
              </ElSpace>
            </div>
          </template>
          <pre v-loading="running" class="term">{{ output || '（暂无输出）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage } from 'element-plus'
  import { fetchInterfaces, fetchToolMeta, runTool } from '@/api/platform'

  defineOptions({ name: 'DeviceTools' })

  const running = ref(false)
  const output = ref('')
  const result = ref<any>(null)
  const meta = ref<Record<string, any>>({ enabled: true })
  const interfaces = ref<any[]>([])
  const form = reactive<Record<string, any>>({
    action: 'ping',
    target: '',
    iface: '',
    count: 4,
    port: 0
  })

  const onRun = async () => {
    if (form.action !== 'netstat' && !form.target) {
      ElMessage.warning(form.action === 'tcpdump' ? '请填写过滤表达式' : '请填写目标地址')
      return
    }
    running.value = true
    output.value = '执行中…'
    try {
      const res = (await runTool({ ...form })) as any
      result.value = res
      output.value = res?.output || '（无输出）'
      if (res?.error) output.value += `\n\n[错误] ${res.error}`
    } finally {
      running.value = false
    }
  }

  onMounted(async () => {
    meta.value = (await fetchToolMeta()) as any
    try {
      interfaces.value = (await fetchInterfaces()) as any[]
      if (interfaces.value.length) form.iface = interfaces.value[0].name
    } catch {
      interfaces.value = []
    }
  })
</script>

<style lang="scss" scoped>
  .term {
    min-height: 380px;
    max-height: 560px;
    padding: 12px;
    overflow: auto;
    font-family: 'JetBrains Mono', Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.7;
    white-space: pre-wrap;
    border-radius: 6px;
    background: #1e1e1e;
    color: #d4d4d4;
  }
</style>

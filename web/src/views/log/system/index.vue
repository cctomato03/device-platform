<!-- 系统日志：平台运行日志（文件尾部实时查看） -->
<template>
  <div class="art-full-height">
    <ElCard>
      <template #header>
        <div class="flex-cb">
          <span>系统日志</span>
          <ElSpace>
            <ElTag v-if="info?.dir" size="small" type="info">{{ info.dir }}</ElTag>
            <ElSelect v-model="lines" size="small" style="width: 130px" @change="loadData">
              <ElOption label="最近 200 行" :value="200" />
              <ElOption label="最近 500 行" :value="500" />
              <ElOption label="最近 1000 行" :value="1000" />
              <ElOption label="最近 2000 行" :value="2000" />
            </ElSelect>
            <ElCheckbox v-model="autoRefresh">自动刷新</ElCheckbox>
            <ElButton size="small" :loading="loading" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        当前读取：<code>{{ info?.file || '-' }}</code>（日志目录 <code>{{ info?.dir || '-' }}</code>，
        共 {{ (info?.files || []).length }} 个日志文件）
      </ElAlert>

      <pre v-loading="loading" class="log-box">{{ text || '（暂无日志）' }}</pre>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchSystemLog } from '@/api/platform'

  defineOptions({ name: 'LogSystem' })

  const loading = ref(false)
  const lines = ref(500)
  const info = ref<any>(null)
  const text = ref('')
  const autoRefresh = ref(false)
  let timer: any = null

  const loadData = async () => {
    loading.value = true
    try {
      const res = (await fetchSystemLog(lines.value)) as any
      info.value = res
      text.value = (res?.lines || []).join('\n')
    } finally {
      loading.value = false
    }
  }

  watch(autoRefresh, (v) => {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    if (v) timer = setInterval(loadData, 5000)
  })

  onMounted(loadData)
  onUnmounted(() => timer && clearInterval(timer))
</script>

<style lang="scss" scoped>
  .log-box {
    height: calc(100vh - 280px);
    min-height: 320px;
    padding: 12px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    border-radius: 6px;
    background: #1e1e1e;
    color: #d4d4d4;
  }
</style>

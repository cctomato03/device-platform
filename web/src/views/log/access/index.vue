<!-- 访问日志：所有 HTTP 访问记录（IP / 方法 / 路径 / 状态码 / 耗时） -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>访问日志</span>
          <ElSpace>
            <ElInput v-model="query.ip" placeholder="按 IP 过滤" clearable size="small" style="width: 150px" />
            <ElInput v-model="query.path" placeholder="按路径过滤" clearable size="small" style="width: 170px" />
            <ElSelect v-model="query.status" size="small" style="width: 130px" clearable placeholder="状态码">
              <ElOption label="200" :value="200" />
              <ElOption label="401" :value="401" />
              <ElOption label="403" :value="403" />
              <ElOption label="500" :value="500" />
            </ElSelect>
            <ElButton size="small" type="primary" @click="search">查询</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="accessTime" label="时间" width="170" />
        <ElTableColumn prop="ip" label="来源 IP" width="140" />
        <ElTableColumn prop="method" label="方法" width="90">
          <template #default="{ row }">
            <ElTag size="small" :type="row.method === 'GET' ? 'info' : 'warning'">{{ row.method }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="path" label="路径" min-width="240" show-overflow-tooltip />
        <ElTableColumn label="状态" width="90">
          <template #default="{ row }">
            <ElTag size="small" :type="row.status < 400 ? 'success' : row.status < 500 ? 'warning' : 'danger'">
              {{ row.status }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="耗时" width="100">
          <template #default="{ row }">{{ row.costMs }} ms</template>
        </ElTableColumn>
        <ElTableColumn prop="traceId" label="TraceId" width="170" show-overflow-tooltip />
        <ElTableColumn prop="userAgent" label="User-Agent" min-width="180" show-overflow-tooltip />
      </ElTable>

      <div class="mt-3 flex justify-end">
        <ElPagination
          v-model:current-page="query.current"
          v-model:page-size="query.size"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { fetchAccessLogs } from '@/api/platform'

  defineOptions({ name: 'LogAccess' })

  const loading = ref(false)
  const rows = ref<any[]>([])
  const total = ref(0)
  const query = reactive<Record<string, any>>({ ip: '', path: '', status: '', current: 1, size: 20 })

  const loadData = async () => {
    loading.value = true
    try {
      const res = (await fetchAccessLogs({ ...query })) as any
      rows.value = res?.list || []
      total.value = res?.total || 0
    } finally {
      loading.value = false
    }
  }

  const search = () => {
    query.current = 1
    loadData()
  }

  onMounted(loadData)
</script>

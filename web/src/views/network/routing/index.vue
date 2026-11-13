<!-- 动态路由：BGP / OSPF（FRR）配置与邻居状态 -->
<template>
  <div class="art-full-height">
    <ElCard class="mb-3">
      <template #header>
        <div class="flex-cb">
          <span>动态路由（FRR）</span>
          <ElSpace>
            <ElTag size="small" :type="status?.active ? 'success' : 'info'">
              FRR {{ status?.active ? '运行中' : '未运行' }}
            </ElTag>
            <ElButton v-auth="'network:routing:install'" size="small" @click="onInstall">安装依赖</ElButton>
            <ElButton v-auth="'network:routing:apply'" size="small" type="warning" @click="onApply">下发配置</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElTabs v-model="tab">
        <ElTabPane label="BGP" name="bgp">
          <ElForm label-width="120px" class="mt-2">
            <ElFormItem label="启用 BGP">
              <ElSwitch v-model="bgp.enabled" />
            </ElFormItem>
            <ElFormItem label="本地 AS 号" required>
              <ElInputNumber v-model="bgp.asn" :min="1" :max="4294967295" />
            </ElFormItem>
            <ElFormItem label="Router ID">
              <ElInput v-model="bgp.routerId" placeholder="1.1.1.1" />
            </ElFormItem>
            <ElFormItem label="宣告网段">
              <ElInput v-model="bgp.networks" placeholder="10.10.0.0/24（多个用逗号分隔）" />
            </ElFormItem>
            <ElFormItem label="邻居">
              <div class="w-full">
                <div v-for="(n, idx) in bgpNeighbors" :key="idx" class="mb-2 flex gap-2">
                  <ElInput v-model="n.addr" placeholder="对端地址（10.0.0.2）" size="small" />
                  <ElInputNumber v-model="n.remoteAs" :min="1" :max="4294967295" size="small" />
                  <ElButton size="small" link type="danger" @click="bgpNeighbors.splice(idx, 1)">移除</ElButton>
                </div>
                <ElButton size="small" @click="bgpNeighbors.push({ addr: '', remoteAs: 65002 })">+ 添加邻居</ElButton>
              </div>
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'network:routing:update'" type="primary" :loading="saving" @click="save('bgp')">
                保存 BGP 配置
              </ElButton>
            </ElFormItem>
          </ElForm>
        </ElTabPane>

        <ElTabPane label="OSPF" name="ospf">
          <ElForm label-width="120px" class="mt-2">
            <ElFormItem label="启用 OSPF">
              <ElSwitch v-model="ospf.enabled" />
            </ElFormItem>
            <ElFormItem label="Router ID" required>
              <ElInput v-model="ospf.routerId" placeholder="1.1.1.1" />
            </ElFormItem>
            <ElFormItem label="区域 Area">
              <ElInput v-model="ospf.area" placeholder="0" />
            </ElFormItem>
            <ElFormItem label="启用网段" required>
              <ElInput v-model="ospf.networks" placeholder="192.168.1.0/24（多个用逗号分隔）" />
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'network:routing:update'" type="primary" :loading="saving" @click="save('ospf')">
                保存 OSPF 配置
              </ElButton>
            </ElFormItem>
          </ElForm>
        </ElTabPane>
      </ElTabs>

      <ElCard v-if="results.length" class="mt-3" shadow="never" header="下发结果">
        <ResultList :results="results" />
      </ElCard>
    </ElCard>

    <ElRow :gutter="12">
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="BGP 邻居状态">
          <pre class="code-box">{{ (status?.bgpSummary || []).join('\n') || '（暂无数据）' }}</pre>
        </ElCard>
      </ElCol>
      <ElCol :xs="24" :md="12">
        <ElCard class="mb-3" header="OSPF 邻居状态">
          <pre class="code-box">{{ (status?.ospfNeighbor || []).join('\n') || '（暂无数据）' }}</pre>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import {
    applyRoutingConfigs,
    fetchRoutingConfigs,
    installFRR,
    saveRoutingConfig,
    type ApplyResult
  } from '@/api/ops'
  import ResultList from '@/views/network/components/ResultList.vue'

  defineOptions({ name: 'NetworkRouting' })

  const tab = ref('bgp')
  const saving = ref(false)
  const status = ref<any>(null)
  const results = ref<ApplyResult[]>([])
  const bgp = reactive<Record<string, any>>({ enabled: false, asn: 65001, routerId: '', networks: '' })
  const ospf = reactive<Record<string, any>>({ enabled: false, routerId: '', networks: '', area: '0' })
  const bgpNeighbors = ref<any[]>([])

  const loadData = async () => {
    const res = (await fetchRoutingConfigs()) as any
    status.value = res?.status || null
    for (const cfg of res?.list || []) {
      if (cfg.protocol === 'bgp') {
        Object.assign(bgp, {
          enabled: !!cfg.enabled,
          asn: cfg.asn || 65001,
          routerId: cfg.routerId || '',
          networks: cfg.networks || ''
        })
        try {
          bgpNeighbors.value = JSON.parse(cfg.neighbors || '[]')
        } catch {
          bgpNeighbors.value = []
        }
      }
      if (cfg.protocol === 'ospf') {
        Object.assign(ospf, {
          enabled: !!cfg.enabled,
          routerId: cfg.routerId || '',
          networks: cfg.networks || '',
          area: cfg.area || '0'
        })
      }
    }
  }

  const save = async (proto: 'bgp' | 'ospf') => {
    saving.value = true
    try {
      if (proto === 'bgp') {
        await saveRoutingConfig({ ...bgp, protocol: 'bgp', neighbors: bgpNeighbors.value })
      } else {
        await saveRoutingConfig({ ...ospf, protocol: 'ospf' })
      }
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onApply = async () => {
    const res = (await applyRoutingConfigs()) as any
    results.value = res?.results || []
    await loadData()
  }

  const onInstall = async () => {
    const res = (await installFRR()) as any
    results.value = res?.results || []
    await loadData()
  }

  onMounted(loadData)
</script>

<style lang="scss" scoped>
  .code-box {
    max-height: 260px;
    padding: 10px;
    overflow: auto;
    font-family: Consolas, Monaco, monospace;
    font-size: 12px;
    line-height: 1.6;
    border-radius: 6px;
    background: var(--art-main-bg-color);
  }
</style>

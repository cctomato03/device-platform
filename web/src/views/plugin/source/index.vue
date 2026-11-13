<!-- 插件源：在线索引地址与签名密钥管理 -->
<template>
  <div class="art-full-height">
    <ElCard class="art-table-card">
      <template #header>
        <div class="flex-cb">
          <span>插件源（在线市场索引）</span>
          <ElSpace>
            <ElButton v-auth="'plugin:source:create'" size="small" type="primary" @click="openForm()">新增源</ElButton>
            <ElButton size="small" @click="loadData">刷新</ElButton>
          </ElSpace>
        </div>
      </template>

      <ElAlert type="info" :closable="false" class="mb-3">
        索引是一个 HTTPS JSON（含 <code>plugins</code> 数组）；配置了「签名密钥」后，平台会用
        HMAC-SHA256 校验响应头 <code>X-Index-Signature</code>，校验不通过直接拒绝，
        插件包还会按索引里的 <code>sha256</code> 二次校验。
      </ElAlert>

      <ElTable v-loading="loading" :data="rows" size="small">
        <ElTableColumn prop="name" label="名称" width="160" />
        <ElTableColumn prop="url" label="索引地址" min-width="260" show-overflow-tooltip />
        <ElTableColumn label="签名校验" width="110">
          <template #default="{ row }">
            <ElTag size="small" :type="row.url ? 'success' : 'info'">
              {{ hasTrustKey(row) ? '已配置' : '未配置' }}
            </ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn label="启用" width="80">
          <template #default="{ row }">
            <ElTag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="itemCount" label="条目" width="80" />
        <ElTableColumn prop="lastSyncAt" label="最近同步" width="170" />
        <ElTableColumn prop="lastError" label="最近错误" min-width="180" show-overflow-tooltip />
        <ElTableColumn label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <ElButton v-auth="'plugin:source:update'" link type="primary" size="small" @click="openForm(row)">
              编辑
            </ElButton>
            <ElButton v-auth="'plugin:source:delete'" link type="danger" size="small" @click="onDelete(row)">
              删除
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog v-model="formVisible" :title="form.id ? '编辑插件源' : '新增插件源'" width="560px">
      <ElForm :model="form" label-width="110px">
        <ElFormItem label="名称" required>
          <ElInput v-model="form.name" placeholder="官方插件源" />
        </ElFormItem>
        <ElFormItem label="索引地址">
          <ElInput v-model="form.url" placeholder="https://example.com/plugin-index.json" />
        </ElFormItem>
        <ElFormItem label="签名密钥">
          <ElInput v-model="form.trustKey" type="password" show-password placeholder="留空表示不校验签名（编辑时留空=保持不变）" />
          <p class="text-xs text-g-400 mt-1">
            与索引发布端一致：<code>hex(HMAC-SHA256(索引原始字节, 密钥))</code> 写入响应头 X-Index-Signature
          </p>
        </ElFormItem>
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled" />
        </ElFormItem>
        <ElFormItem label="备注">
          <ElInput v-model="form.remark" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="formVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="saving" @click="onSave">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessageBox } from 'element-plus'
  import { deleteMarketSource, fetchMarketSources, saveMarketSource } from '@/api/ops'

  defineOptions({ name: 'PluginSource' })

  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<any[]>([])
  const formVisible = ref(false)
  const form = reactive<Record<string, any>>({})

  const hasTrustKey = (row: any) => !!(row as any).trustKeyConfigured

  const emptyForm = () => ({ id: '', name: '', url: '', trustKey: '', enabled: true, remark: '' })

  const loadData = async () => {
    loading.value = true
    try {
      rows.value = (await fetchMarketSources()) as any[]
    } finally {
      loading.value = false
    }
  }

  const openForm = (row?: any) => {
    Object.keys(form).forEach((k) => delete form[k])
    Object.assign(form, emptyForm(), row ? { ...row, trustKey: '' } : {})
    formVisible.value = true
  }

  const onSave = async () => {
    saving.value = true
    try {
      await saveMarketSource({ ...form })
      formVisible.value = false
      await loadData()
    } finally {
      saving.value = false
    }
  }

  const onDelete = async (row: any) => {
    await ElMessageBox.confirm(`确认删除插件源「${row.name}」？`, '删除确认', { type: 'warning' })
    await deleteMarketSource(row.id)
    await loadData()
  }

  onMounted(loadData)
</script>

<!-- 本地设置：设备名 / 时区 / NTP / DNS / 流量嗅探开关 -->
<template>
  <div class="art-full-height">
    <ElRow :gutter="12">
      <ElCol :xs="24" :md="14">
        <ElCard class="mb-3" header="设备本地设置">
          <ElForm label-width="110px">
            <ElFormItem label="设备名称">
              <ElInput v-model="form.name" placeholder="例如 edge-gw-01" />
            </ElFormItem>
            <ElFormItem label="时区">
              <ElSelect v-model="form.timezone" filterable allow-create style="width: 100%">
                <ElOption v-for="tz in timezones" :key="tz" :label="tz" :value="tz" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem label="NTP 同步">
              <ElSwitch v-model="form.ntpEnabled" />
              <span class="ml-2 text-xs text-g-400">开启后由 systemd-timesyncd / chrony 自动校时</span>
            </ElFormItem>
            <ElFormItem label="NTP 服务器">
              <ElInput v-model="form.ntpServers" placeholder="ntp.aliyun.com,pool.ntp.org" />
            </ElFormItem>
            <ElFormItem label="DNS">
              <ElInput v-model="form.dns" placeholder="223.5.5.5,114.114.114.114" />
            </ElFormItem>
            <ElFormItem label="流量嗅探">
              <ElSwitch v-model="form.sniffEnabled" />
              <span class="ml-2 text-xs text-g-400">开启后可配合「排查工具 → tcpdump」采集报文</span>
            </ElFormItem>
            <ElFormItem v-if="form.sniffEnabled" label="嗅探网卡">
              <ElSelect v-model="form.sniffIface" clearable style="width: 100%">
                <ElOption v-for="i in interfaces" :key="i.name" :label="i.name" :value="i.name" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem label="备注">
              <ElInput v-model="form.remark" type="textarea" :rows="2" placeholder="设备位置/负责人等" />
            </ElFormItem>
            <ElFormItem>
              <ElButton v-auth="'device:setting:update'" type="primary" :loading="saving" @click="onSave">保存设置</ElButton>
              <ElButton @click="loadData">重置</ElButton>
            </ElFormItem>
          </ElForm>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :md="10">
        <ElCard class="mb-3" header="系统时间">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="本地时间">{{ time?.localTime || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="UTC">{{ time?.utc || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="时区">{{ time?.timezone || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="NTP 开关">{{ time?.ntpEnabled ? '已开启' : '已关闭' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="已同步">{{ time?.ntpSynced ? '是' : '否' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="采集来源">{{ time?.source || '本地时钟' }}</ElDescriptionsItem>
          </ElDescriptions>
          <div class="mt-3">
            <ElButton v-auth="'device:setting:update'" size="small" type="warning" :loading="syncing" @click="onSync">
              立即校时
            </ElButton>
          </div>
        </ElCard>

        <ElCard v-if="results.length" header="下发结果">
          <div v-for="(r, i) in results" :key="i" class="mb-2 text-sm">
            <ElTag :type="r.ok ? 'success' : 'danger'" size="small">{{ r.ok ? '成功' : '失败' }}</ElTag>
            <span class="ml-2 font-medium">{{ r.item }}</span>
            <div class="text-g-400 ml-1">{{ r.message }}</div>
          </div>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage } from 'element-plus'
  import {
    fetchDeviceSettings,
    fetchDeviceTime,
    fetchInterfaces,
    syncDeviceTime,
    updateDeviceSettings,
    type ApplyResult
  } from '@/api/platform'

  defineOptions({ name: 'DeviceSetting' })

  const saving = ref(false)
  const syncing = ref(false)
  const time = ref<any>(null)
  const interfaces = ref<any[]>([])
  const results = ref<ApplyResult[]>([])
  const form = reactive<Record<string, any>>({
    name: '',
    timezone: 'Asia/Shanghai',
    ntpEnabled: true,
    ntpServers: '',
    dns: '',
    sniffEnabled: false,
    sniffIface: '',
    remark: ''
  })

  const timezones = [
    'Asia/Shanghai',
    'Asia/Hong_Kong',
    'Asia/Tokyo',
    'Asia/Singapore',
    'UTC',
    'Europe/London',
    'America/New_York'
  ]

  const loadData = async () => {
    const data = (await fetchDeviceSettings()) as any
    Object.assign(form, {
      name: data?.name || '',
      timezone: data?.timezone || 'Asia/Shanghai',
      ntpEnabled: !!data?.ntpEnabled,
      ntpServers: data?.ntpServers || '',
      dns: data?.dns || '',
      sniffEnabled: !!data?.sniffEnabled,
      sniffIface: data?.sniffIface || '',
      remark: data?.remark || ''
    })
    time.value = await fetchDeviceTime()
    try {
      interfaces.value = (await fetchInterfaces()) as any[]
    } catch {
      interfaces.value = []
    }
  }

  const onSave = async () => {
    saving.value = true
    try {
      const res = (await updateDeviceSettings({ ...form })) as any
      results.value = res?.results || []
      time.value = await fetchDeviceTime()
    } finally {
      saving.value = false
    }
  }

  const onSync = async () => {
    syncing.value = true
    try {
      const res = (await syncDeviceTime()) as any
      results.value = res?.results || []
      time.value = await fetchDeviceTime()
      ElMessage.success('校时指令已执行')
    } finally {
      syncing.value = false
    }
  }

  onMounted(loadData)
</script>

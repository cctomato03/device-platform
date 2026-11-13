<!-- 下发结果列表（多个页面共用：逐项成功/失败 + 原因） -->
<template>
  <div>
    <div v-for="(r, i) in results" :key="i" class="result-row">
      <ElTag :type="r.ok ? 'success' : 'danger'" size="small">{{ r.ok ? '成功' : '失败' }}</ElTag>
      <span class="ml-2 font-medium">{{ r.item }}</span>
      <span class="ml-2 text-g-400 break-all">{{ r.message }}</span>
    </div>
    <p v-if="!results.length" class="text-xs text-g-400">暂无结果</p>
  </div>
</template>

<script setup lang="ts">
  import type { ApplyResult } from '@/api/ops'

  defineProps<{ results: ApplyResult[] }>()
</script>

<style lang="scss" scoped>
  .result-row {
    padding: 4px 0;
    font-size: 13px;
    line-height: 1.6;
    border-bottom: 1px dashed var(--art-border-color);

    &:last-child {
      border-bottom: none;
    }
  }
</style>

<!-- 系统logo（站点设置里上传了 logo 就用它，否则回退到内置图标） -->
<template>
  <div class="flex-cc">
    <img
      :style="logoStyle"
      :src="siteLogo || defaultLogo"
      alt="logo"
      class="w-full h-full object-contain"
    />
  </div>
</template>

<script setup lang="ts">
  import defaultLogo from '@imgs/common/logo.webp'
  import { siteConfig } from '@/utils/site'

  defineOptions({ name: 'ArtLogo' })

  interface Props {
    /** logo 大小 */
    size?: number | string
  }

  const props = withDefaults(defineProps<Props>(), {
    size: 36
  })

  const siteLogo = computed(() => siteConfig.siteLogo)
  const logoStyle = computed(() => ({ width: `${props.size}px` }))
</script>

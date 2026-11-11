<!-- 登录页面 -->
<template>
  <div class="flex w-full h-screen">
    <LoginLeftView />

    <div class="relative flex-1">
      <AuthTopBar />

      <div class="auth-right-wrap">
        <div class="form">
          <h3 class="title">{{ $t('login.title') }}</h3>
          <p class="sub-title">{{ $t('login.subTitle') }}</p>
          <!-- validate-on-rule-change=false：验证码开关等规则变化不再立刻把空输入框标红，
               只有用户点击「登录」时才做校验 -->
          <ElForm
            ref="formRef"
            :model="formData"
            :rules="rules"
            :key="formKey"
            :validate-on-rule-change="false"
            @keyup.enter="handleSubmit"
            style="margin-top: 25px"
          >
            <ElFormItem prop="username">
              <ElInput
                class="custom-height"
                :placeholder="$t('login.placeholder.username')"
                v-model.trim="formData.username"
              />
            </ElFormItem>
            <ElFormItem prop="password">
              <ElInput
                class="custom-height"
                :placeholder="$t('login.placeholder.password')"
                v-model.trim="formData.password"
                type="password"
                autocomplete="off"
                show-password
              />
            </ElFormItem>

            <!-- 图形验证码（由后端参数 sys.login.captcha 控制开关） -->
            <ElFormItem v-if="captcha.enabled" prop="captchaCode">
              <div class="flex w-full items-center gap-2">
                <ElInput
                  class="custom-height"
                  :placeholder="$t('login.placeholder.captcha')"
                  v-model.trim="formData.captchaCode"
                  maxlength="6"
                  @keyup.enter="handleSubmit"
                />
                <img
                  v-if="captcha.image"
                  :src="captcha.image"
                  class="h-10 cursor-pointer rounded-md"
                  :title="$t('login.placeholder.captchaRefresh')"
                  @click="loadCaptcha"
                />
              </div>
            </ElFormItem>

            <!-- 拖拽验证（由后端参数 sys.login.slider 控制开关） -->
            <div v-if="sliderEnabled" class="relative pb-5 mt-6">
              <div
                class="relative z-[2] overflow-hidden select-none rounded-lg border border-transparent tad-300"
                :class="{ '!border-[#FF4E4F]': !isPassing && isClickPass }"
              >
                <ArtDragVerify
                  ref="dragVerify"
                  v-model:value="isPassing"
                  :text="$t('login.sliderText')"
                  textColor="var(--art-gray-700)"
                  :successText="$t('login.sliderSuccessText')"
                  progressBarBg="var(--main-color)"
                  :background="isDark ? '#26272F' : '#F1F1F4'"
                  handlerBg="var(--default-box-color)"
                />
              </div>
              <p
                class="absolute top-0 z-[1] px-px mt-2 text-xs text-[#f56c6c] tad-300"
                :class="{ 'translate-y-10': !isPassing && isClickPass }"
              >
                {{ $t('login.placeholder.slider') }}
              </p>
            </div>

            <div class="flex-cb mt-2 text-sm">
              <ElCheckbox v-model="formData.rememberPassword">{{
                $t('login.rememberPwd')
              }}</ElCheckbox>
              <RouterLink class="text-theme" :to="{ name: 'ForgetPassword' }">{{
                $t('login.forgetPwd')
              }}</RouterLink>
            </div>

            <div style="margin-top: 30px">
              <ElButton
                class="w-full custom-height"
                type="primary"
                @click="handleSubmit"
                :loading="loading"
                v-ripple
              >
                {{ $t('login.btnText') }}
              </ElButton>
            </div>

            <!-- 注册入口由站点配置控制（后台「站点设置」可一键关闭） -->
            <div v-if="siteConfig.registerEnabled" class="mt-5 text-sm text-gray-600">
              <span>{{ $t('login.noAccount') }}</span>
              <RouterLink class="text-theme" :to="{ name: 'Register' }">{{
                $t('login.register')
              }}</RouterLink>
            </div>
          </ElForm>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { useUserStore } from '@/store/modules/user'
  import { siteConfig } from '@/utils/site'
  import { useI18n } from 'vue-i18n'
  import { HttpError } from '@/utils/http/error'
  import { fetchLogin, fetchGetCaptcha } from '@/api/auth'
  import { fetchGetPublicConfig } from '@/api/business'
  import { ElNotification, type FormInstance, type FormRules } from 'element-plus'
  import { useSettingStore } from '@/store/modules/setting'

  defineOptions({ name: 'Login' })

  const settingStore = useSettingStore()
  const { isDark } = storeToRefs(settingStore)
  const { t, locale } = useI18n()
  const formKey = ref(0)

  // 监听语言切换，重置表单
  watch(locale, () => {
    formKey.value++
  })

  const dragVerify = ref()

  const userStore = useUserStore()
  const router = useRouter()
  const route = useRoute()
  const isPassing = ref(false)
  const isClickPass = ref(false)

  // 系统名称来自站点个性化配置（AppConfig.systemInfo.name 已在启动时同步）
  const systemName = computed(() => siteConfig.siteName)
  const formRef = ref<FormInstance>()

  const formData = reactive({
    username: '',
    password: '',
    captchaCode: '',
    rememberPassword: true
  })

  // 图形验证码（后端参数 sys.login.captcha 控制）
  const captcha = reactive({ enabled: false, captchaId: '', image: '' })
  // 滑块校验（后端参数 sys.login.slider 控制；关闭后直接点登录）
  const sliderEnabled = ref(true)

  const rules = computed<FormRules>(() => ({
    username: [{ required: true, message: t('login.placeholder.username'), trigger: 'blur' }],
    password: [{ required: true, message: t('login.placeholder.password'), trigger: 'blur' }],
    captchaCode: captcha.enabled
      ? [{ required: true, message: t('login.placeholder.captcha'), trigger: 'blur' }]
      : []
  }))

  const loading = ref(false)

  // 加载验证码
  const loadCaptcha = async () => {
    try {
      const res = await fetchGetCaptcha()
      captcha.enabled = res.enabled
      captcha.captchaId = res.captchaId
      captcha.image = res.image
      formData.captchaCode = ''
    } catch {
      captcha.enabled = false
    }
  }

  /** 读取登录交互开关（验证码/滑块），后台「参数配置」改完刷新页面即生效 */
  const loadLoginSwitches = async () => {
    try {
      const cfg = await fetchGetPublicConfig()
      if (cfg && cfg['sys.login.slider'] !== undefined) {
        sliderEnabled.value = `${cfg['sys.login.slider']}` !== 'false'
      }
    } catch {
      sliderEnabled.value = true
    }
  }

  onMounted(() => {
    loadCaptcha()
    loadLoginSwitches()
  })

  // 登录
  const handleSubmit = async () => {
    if (!formRef.value) return

    try {
      // 表单验证
      const valid = await formRef.value.validate()
      if (!valid) return

      // 拖拽验证（开关关闭时跳过）
      if (sliderEnabled.value && !isPassing.value) {
        isClickPass.value = true
        return
      }

      loading.value = true

      // 登录请求
      const { username, password, captchaCode } = formData

      const { token, refreshToken } = await fetchLogin({
        userName: username,
        password,
        captchaId: captcha.enabled ? captcha.captchaId : undefined,
        captchaCode: captcha.enabled ? captchaCode : undefined
      })

      // 验证token
      if (!token) {
        throw new Error('Login failed - no token received')
      }

      // 存储 token 和登录状态
      userStore.setToken(token, refreshToken)
      userStore.setLoginStatus(true)

      // 登录成功处理
      showLoginSuccessNotice()

      // 跳转目标：优先回跳 redirect，但要过滤登录/注册等免登录页，避免死循环
      const redirect = route.query.redirect as string
      const target =
        typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('/auth')
          ? redirect
          : '/'

      // 用 replace 而非 push：登录页不留历史；必须 await，跳转异常时兜底回首页
      try {
        await router.replace(target)
      } catch (navError) {
        console.error('[Login] 跳转首页失败，重试根路径:', navError)
        await router.replace('/')
      }
    } catch (error) {
      // HttpError 的错误提示已由 http 层统一弹出（后端业务消息优先），这里只处理非预期异常
      if (!(error instanceof HttpError)) {
        console.error('[Login] Unexpected error:', error)
      }
      // 登录失败后刷新验证码，避免重复使用同一码
      loadCaptcha()
    } finally {
      loading.value = false
      resetDragVerify()
    }
  }

  // 重置拖拽验证
  const resetDragVerify = () => {
    dragVerify.value?.reset()
  }

  // 登录成功提示
  const showLoginSuccessNotice = () => {
    setTimeout(() => {
      ElNotification({
        title: t('login.success.title'),
        type: 'success',
        duration: 2500,
        zIndex: 10000,
        message: `${t('login.success.message')}, ${systemName.value}!`
      })
    }, 1000)
  }
</script>

<style scoped>
  @import './style.css';
</style>

<style lang="scss" scoped>
  :deep(.el-select__wrapper) {
    height: 40px !important;
  }
</style>

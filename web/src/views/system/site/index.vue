<!-- 站点设置：网站名称 / logo / 登录页背景与文案 / 注册开关 -->
<template>
  <div class="site-page">
    <ElRow :gutter="16">
      <ElCol :xs="24" :lg="14">
        <ElCard shadow="never">
          <template #header>
            <div class="card-header">
              <span>站点个性化配置</span>
              <span class="tip">保存后登录页与浏览器标题立即生效</span>
            </div>
          </template>

          <ElForm ref="formRef" :model="form" :rules="rules" label-width="120px" v-loading="loading">
            <ElFormItem label="网站名称" prop="siteName">
              <ElInput v-model.trim="form.siteName" maxlength="64" show-word-limit placeholder="例如：XX 管理系统" />
            </ElFormItem>

            <ElFormItem label="站点 logo">
              <div class="upload-row">
                <ElUpload
                  :show-file-list="false"
                  :auto-upload="false"
                  accept="image/*"
                  :on-change="(f) => handlePick(f, 'logo')"
                >
                  <div class="upload-box">
                    <img v-if="form.siteLogo" :src="form.siteLogo" alt="logo" />
                    <div v-else class="placeholder">
                      <ArtSvgIcon icon="ri:image-add-line" />
                      <span>上传 logo</span>
                    </div>
                  </div>
                </ElUpload>
                <div class="upload-hint">
                  <p>建议 1:1 透明底 PNG/SVG，≤ 2MB；留空则使用内置图标。</p>
                  <ElButton v-if="form.siteLogo" link type="danger" @click="form.siteLogo = ''">
                    清除
                  </ElButton>
                </div>
              </div>
            </ElFormItem>

            <ElFormItem label="登录页背景图">
              <div class="upload-row">
                <ElUpload
                  :show-file-list="false"
                  :auto-upload="false"
                  accept="image/*"
                  :on-change="(f) => handlePick(f, 'bg')"
                >
                  <div class="upload-box wide">
                    <img v-if="form.loginBg" :src="form.loginBg" alt="login bg" />
                    <div v-else class="placeholder">
                      <ArtSvgIcon icon="ri:image-add-line" />
                      <span>上传背景图</span>
                    </div>
                  </div>
                </ElUpload>
                <div class="upload-hint">
                  <p>建议 1920×1080，≤ 2MB；留空则使用默认插画背景。</p>
                  <ElButton v-if="form.loginBg" link type="danger" @click="form.loginBg = ''">
                    清除
                  </ElButton>
                </div>
              </div>
            </ElFormItem>

            <ElFormItem label="登录页描述" prop="loginDesc">
              <ElInput
                v-model.trim="form.loginDesc"
                type="textarea"
                :rows="3"
                maxlength="255"
                show-word-limit
                placeholder="显示在登录页左侧的大标题，留空则使用默认文案"
              />
            </ElFormItem>

            <ElFormItem label="登录页副标题" prop="loginSubTitle">
              <ElInput
                v-model.trim="form.loginSubTitle"
                type="textarea"
                :rows="2"
                maxlength="255"
                show-word-limit
                placeholder="显示在大标题下方的一行说明，留空则使用默认文案"
              />
            </ElFormItem>

            <ElFormItem label="开放注册">
              <ElSwitch v-model="form.registerEnabled" />
              <span class="switch-tip">
                {{
                  form.registerEnabled
                    ? '开启后登录页显示「注册」入口（后端注册接口需自行实现）'
                    : '已关闭：登录页不显示注册入口，直接访问 /auth/register 会被拦回登录页'
                }}
              </span>
            </ElFormItem>

            <ElFormItem label="图形验证码">
              <ElSwitch v-model="loginPolicy.captcha" @change="saveLoginPolicy" />
              <span class="switch-tip">
                {{
                  loginPolicy.captcha
                    ? '已开启：登录页显示验证码，后端校验（防撞库）'
                    : '已关闭：登录页不显示验证码，后端也不再校验'
                }}
              </span>
            </ElFormItem>

            <ElFormItem label="滑块校验">
              <ElSwitch v-model="loginPolicy.slider" @change="saveLoginPolicy" />
              <span class="switch-tip">
                {{
                  loginPolicy.slider
                    ? '已开启：登录前需拖动滑块（纯前端人机校验）'
                    : '已关闭：无需拖动滑块，直接点「登录」即可提交'
                }}
              </span>
            </ElFormItem>

            <ElFormItem>
              <ElButton type="primary" :loading="saving" v-auth="'system:site:update'" @click="save">
                保存配置
              </ElButton>
              <ElButton @click="load">重置</ElButton>
            </ElFormItem>
          </ElForm>
        </ElCard>
      </ElCol>

      <ElCol :xs="24" :lg="10">
        <ElCard shadow="never">
          <template #header>
            <div class="card-header"><span>效果预览</span></div>
          </template>
          <div class="preview" :class="{ 'has-bg': !!form.loginBg }">
            <div
              v-if="form.loginBg"
              class="preview-bg"
              :style="{ backgroundImage: `url(${form.loginBg})` }"
            ></div>
            <div class="preview-body">
              <div class="preview-brand">
                <ArtLogo size="34" />
                <span>{{ form.siteName || '未命名站点' }}</span>
              </div>
              <p class="preview-desc">{{ form.loginDesc || '登录页左侧大标题预览' }}</p>
              <p class="preview-sub">{{ form.loginSubTitle || '登录页副标题预览' }}</p>
            </div>
          </div>
          <p class="preview-tip">
            浏览器标题格式：<code>页面名称 - {{ form.siteName || '站点名称' }}</code>
          </p>
        </ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>

<script setup lang="ts">
  import { fetchSiteConfig, fetchUpdateSiteConfig } from '@/api/site'
  import { fetchGetConfigList, fetchUpdateConfig } from '@/api/business'
  import { uploadFile } from '@/utils/upload'
  import { applySiteConfig } from '@/utils/site'
  import { ElMessage, type FormInstance, type FormRules, type UploadFile } from 'element-plus'

  defineOptions({ name: 'SiteSetting' })

  const formRef = ref<FormInstance>()
  const loading = ref(false)
  const saving = ref(false)

  const form = reactive<WebBasic.SiteConfig>({
    siteName: '',
    siteLogo: '',
    loginBg: '',
    loginDesc: '',
    loginSubTitle: '',
    registerEnabled: false
  })

  const rules: FormRules = {
    siteName: [{ required: true, message: '请输入网站名称', trigger: 'blur' }]
  }

  // 登录页交互开关（存放在 sys_config，键 sys.login.captcha / sys.login.slider）
  const LOGIN_POLICY_KEYS = { captcha: 'sys.login.captcha', slider: 'sys.login.slider' } as const
  const loginPolicy = reactive({ captcha: true, slider: true })
  const configIndex = new Map<string, WebBasic.Config>()

  /** 读取登录交互开关（参数配置里的两个内置参数） */
  const loadLoginPolicy = async () => {
    try {
      const list = await fetchGetConfigList()
      configIndex.clear()
      list.forEach((c) => configIndex.set(c.key, c))
      const cap = configIndex.get(LOGIN_POLICY_KEYS.captcha)
      const slider = configIndex.get(LOGIN_POLICY_KEYS.slider)
      loginPolicy.captcha = cap ? `${cap.value}` !== 'false' : true
      loginPolicy.slider = slider ? `${slider.value}` !== 'false' : true
    } catch {
      /* 读取失败保持默认开启，不阻塞站点配置编辑 */
    }
  }

  /** 保存登录交互开关：复用参数配置更新接口，后端会清理开关缓存 → 立即生效 */
  const saveLoginPolicy = async () => {
    if (!configIndex.size) {
      ElMessage.warning('参数配置未就绪，请稍后重试或到「参数配置」页修改')
      return
    }
    const tasks: Promise<unknown>[] = []
    const patch = (key: string, value: boolean) => {
      const row = configIndex.get(key)
      if (!row) return
      tasks.push(
        fetchUpdateConfig(
          row.id,
          {
            name: row.name,
            key: row.key,
            value: String(value),
            valueType: row.valueType || 'bool',
            isPublic: true,
            remark: row.remark || ''
          },
          { silent: true } // 两个开关一起改，只提示一次
        )
      )
    }
    patch(LOGIN_POLICY_KEYS.captcha, loginPolicy.captcha)
    patch(LOGIN_POLICY_KEYS.slider, loginPolicy.slider)
    try {
      await Promise.all(tasks)
      ElMessage.success('登录交互开关已更新，刷新登录页即可看到效果')
    } catch {
      await loadLoginPolicy()
    }
  }

  const load = async () => {
    loading.value = true
    try {
      const cfg = await fetchSiteConfig()
      form.siteName = cfg.siteName || ''
      form.siteLogo = cfg.siteLogo || ''
      form.loginBg = cfg.loginBg || ''
      form.loginDesc = cfg.loginDesc || ''
      form.loginSubTitle = cfg.loginSubTitle || ''
      form.registerEnabled = cfg.registerEnabled === true
    } finally {
      loading.value = false
    }
  }

  /** 选择图片 → 上传 → 回填地址（≤2MB，仅图片） */
  const handlePick = async (file: UploadFile, kind: 'logo' | 'bg') => {
    const raw = file.raw
    if (!raw) return
    if (!raw.type.startsWith('image/')) {
      ElMessage.warning('只能上传图片文件')
      return
    }
    if (raw.size > 2 * 1024 * 1024) {
      ElMessage.warning('图片不能超过 2MB')
      return
    }
    const uploaded = await uploadFile(raw, { bizType: 'site' })
    const url = uploaded?.url || ''
    if (!url) {
      ElMessage.error('上传失败：未返回文件地址')
      return
    }
    if (kind === 'logo') form.siteLogo = url
    else form.loginBg = url
  }

  const save = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    saving.value = true
    try {
      const saved = await fetchUpdateSiteConfig({ ...form })
      // 立即同步到运行时（标题 / 登录页 / 注册开关），无需刷新页面
      applySiteConfig(saved)
      Object.assign(form, saved)
      } finally {
      saving.value = false
    }
  }

  onMounted(async () => {
    await Promise.all([load(), loadLoginPolicy()])
  })
</script>

<style lang="scss" scoped>
  .site-page {
    .card-header {
      display: flex;
      align-items: baseline;
      gap: 12px;
      font-weight: 600;

      .tip {
        font-size: 12px;
        font-weight: 400;
        color: var(--art-gray-600);
      }
    }

    .upload-row {
      display: flex;
      gap: 16px;
      align-items: flex-start;
    }

    .upload-box {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 96px;
      height: 96px;
      overflow: hidden;
      cursor: pointer;
      background: var(--art-gray-200);
      border: 1px dashed var(--art-gray-400);
      border-radius: 8px;

      &.wide {
        width: 160px;
      }

      img {
        width: 100%;
        height: 100%;
        object-fit: contain;
      }

      .placeholder {
        display: flex;
        flex-direction: column;
        gap: 4px;
        align-items: center;
        font-size: 12px;
        color: var(--art-gray-600);
      }
    }

    .upload-hint {
      font-size: 12px;
      line-height: 1.6;
      color: var(--art-gray-600);

      p {
        margin: 0;
      }
    }

    .switch-tip {
      margin-left: 12px;
      font-size: 12px;
      color: var(--art-gray-600);
    }

    .preview {
      position: relative;
      height: 200px;
      overflow: hidden;
      background: var(--el-color-primary-light-9);
      border-radius: 8px;

      .preview-bg {
        position: absolute;
        inset: 0;
        background-position: center;
        background-size: cover;
      }

      .preview-body {
        position: relative;
        z-index: 2;
        display: flex;
        flex-direction: column;
        gap: 10px;
        justify-content: center;
        height: 100%;
        padding: 0 20px;
        background: rgb(255 255 255 / 45%);
      }

      .preview-brand {
        display: flex;
        gap: 8px;
        align-items: center;
        font-size: 16px;
        font-weight: 600;
      }

      .preview-desc {
        margin: 0;
        font-size: 13px;
        color: var(--art-gray-700);
      }

      .preview-sub {
        margin: 0;
        font-size: 12px;
        color: var(--art-gray-600);
      }
    }

    .preview-tip {
      margin-top: 12px;
      font-size: 12px;
      color: var(--art-gray-600);

      code {
        padding: 1px 6px;
        background: var(--art-gray-200);
        border-radius: 4px;
      }
    }
  }
</style>

<!-- 个人中心：当前登录用户的资料维护与密码修改 -->
<template>
  <div class="w-full h-full p-0 bg-transparent border-none shadow-none">
    <div class="relative flex-b mt-2.5 max-md:block max-md:mt-1">
      <!-- 左侧资料卡 -->
      <div class="w-112 mr-5 max-md:w-full max-md:mr-0">
        <div class="art-card-sm relative p-9 pb-6 overflow-hidden text-center">
          <img class="absolute top-0 left-0 w-full h-50 object-cover" src="@imgs/user/bg.webp" />
          <ElUpload
            class="avatar-upload"
            :show-file-list="false"
            :auto-upload="false"
            accept="image/*"
            :on-change="(f) => handleAvatarPick(f)"
          >
            <div class="avatar-wrap mt-30">
              <img
                class="relative z-10 w-20 h-20 object-cover border-2 border-white rounded-full"
                :src="profileForm.avatar || defaultAvatar"
              />
              <div class="avatar-mask">
                <ArtSvgIcon icon="ri:camera-line" class="text-lg text-white" />
                <span>更换头像</span>
              </div>
            </div>
          </ElUpload>
          <h2 class="mt-5 text-xl font-normal">{{ profile.nickname || profile.username }}</h2>
          <p class="mt-2 text-sm text-g-500">{{ profile.username }}</p>

          <div class="w-75 mx-auto mt-7.5 text-left">
            <div class="mt-2.5">
              <ArtSvgIcon icon="ri:mail-line" class="text-g-700" />
              <span class="ml-2 text-sm">{{ profile.email || '未绑定邮箱' }}</span>
            </div>
            <div class="mt-2.5">
              <ArtSvgIcon icon="ri:smartphone-line" class="text-g-700" />
              <span class="ml-2 text-sm">{{ profile.mobile || '未绑定手机号' }}</span>
            </div>
            <div class="mt-2.5">
              <ArtSvgIcon icon="ri:organization-chart" class="text-g-700" />
              <span class="ml-2 text-sm">{{ profile.deptName || '未分配部门' }}</span>
            </div>
            <div class="mt-2.5">
              <ArtSvgIcon icon="ri:shield-user-line" class="text-g-700" />
              <span class="ml-2 text-sm">{{ (profile.roles || []).join('、') || '暂无角色' }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧表单 -->
      <div class="flex-1 overflow-hidden max-md:w-full max-md:mt-3.5">
        <div class="art-card-sm">
          <h1 class="p-4 text-xl font-normal border-b border-g-300">基本设置</h1>

          <ElForm
            ref="profileFormRef"
            :model="profileForm"
            :rules="profileRules"
            class="box-border p-5 [&>.el-row_.el-form-item]:w-[calc(50%-10px)] [&>.el-row_.el-input]:w-full [&>.el-row_.el-select]:w-full"
            label-width="86px"
            label-position="top"
          >
            <ElRow>
              <ElFormItem label="昵称" prop="nickname">
                <ElInput v-model="profileForm.nickname" maxlength="64" />
              </ElFormItem>
              <ElFormItem label="性别" prop="sex" class="ml-5">
                <ElSelect v-model="profileForm.sex" placeholder="请选择">
                  <ElOption label="男" :value="1" />
                  <ElOption label="女" :value="2" />
                  <ElOption label="未知" :value="0" />
                </ElSelect>
              </ElFormItem>
            </ElRow>

            <ElRow>
              <ElFormItem label="邮箱" prop="email">
                <ElInput v-model="profileForm.email" maxlength="100" />
              </ElFormItem>
              <ElFormItem label="手机" prop="mobile" class="ml-5">
                <ElInput v-model="profileForm.mobile" maxlength="20" />
              </ElFormItem>
            </ElRow>

            <ElFormItem label="头像" prop="avatar">
              <div class="flex-c gap-3 w-full">
                <img
                  :src="profileForm.avatar || defaultAvatar"
                  class="size-12 rounded-full object-cover border border-g-300"
                />
                <ElUpload
                  :show-file-list="false"
                  :auto-upload="false"
                  accept="image/*"
                  :on-change="(f) => handleAvatarPick(f)"
                >
                  <ElButton :loading="uploadingAvatar">上传头像</ElButton>
                </ElUpload>
                <ElButton v-if="profileForm.avatar" link type="danger" @click="profileForm.avatar = ''">
                  清除
                </ElButton>
                <span class="text-xs text-g-500">支持 jpg/png/webp，≤ 20MB；上传后保存即可生效</span>
              </div>
            </ElFormItem>

            <div class="flex-c justify-end">
              <ElButton type="primary" v-ripple :loading="savingProfile" @click="saveProfile">
                保存
              </ElButton>
            </div>
          </ElForm>
        </div>

        <div class="art-card-sm my-5">
          <h1 class="p-4 text-xl font-normal border-b border-g-300">更改密码</h1>

          <ElForm
            ref="pwdFormRef"
            :model="pwdForm"
            :rules="pwdRules"
            class="box-border p-5"
            label-width="100px"
            label-position="top"
          >
            <ElFormItem label="当前密码" prop="oldPassword">
              <ElInput v-model="pwdForm.oldPassword" type="password" show-password />
            </ElFormItem>

            <ElFormItem label="新密码" prop="newPassword">
              <ElInput v-model="pwdForm.newPassword" type="password" show-password />
              <div class="pwd-tip">{{ passwordTip }}</div>
            </ElFormItem>

            <ElFormItem label="确认新密码" prop="confirmPassword">
              <ElInput v-model="pwdForm.confirmPassword" type="password" show-password />
            </ElFormItem>

            <ElAlert
              class="mb-4"
              type="warning"
              :closable="false"
              title="修改成功后当前账号的所有登录状态会立即失效，需要使用新密码重新登录。"
            />

            <div class="flex-c justify-end">
              <ElButton type="primary" v-ripple :loading="savingPwd" @click="savePassword">
                保存
              </ElButton>
            </div>
          </ElForm>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ElAlert, ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
  import { fetchChangePassword, fetchGetProfile, fetchUpdateProfile } from '@/api/auth'
  import { useUserStore } from '@/store/modules/user'
  import { uploadFile } from '@/utils/upload'
  import { passwordValidator, getPasswordPolicy, DEFAULT_PASSWORD_POLICY } from '@/utils/password'
  import defaultAvatar from '@/assets/images/user/avatar.webp'

  defineOptions({ name: 'UserCenter' })

  const userStore = useUserStore()

  const profile = reactive<WebBasic.Profile>({
    id: '',
    username: '',
    nickname: '',
    avatar: '',
    mobile: '',
    email: '',
    sex: 0,
    deptName: '',
    roles: []
  })

  const profileFormRef = ref<FormInstance>()
  const pwdFormRef = ref<FormInstance>()
  const savingProfile = ref(false)
  const savingPwd = ref(false)
  const uploadingAvatar = ref(false)
  // 密码规则（后端下发，展示给用户看，避免盲试）
  const passwordTip = ref(DEFAULT_PASSWORD_POLICY.description)
  onMounted(async () => {
    const policy = await getPasswordPolicy()
    passwordTip.value = policy.description
  })

  const profileForm = reactive({
    nickname: '',
    avatar: '',
    mobile: '',
    email: '',
    sex: 0
  })

  const pwdForm = reactive({
    oldPassword: '',
    newPassword: '',
    confirmPassword: ''
  })

  const profileRules: FormRules = {
    nickname: [{ required: true, message: '请输入昵称', trigger: 'blur' }],
    email: [{ type: 'email', message: '邮箱格式不正确', trigger: 'blur' }]
  }

  const pwdRules: FormRules = {
    oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
    newPassword: [
      { required: true, message: '请输入新密码', trigger: 'blur' },
      // 与后端 crypto.CheckStrength 完全一致的强口令校验：不合格时直接说明原因
      { validator: passwordValidator(), trigger: 'blur' }
    ],
    confirmPassword: [
      { required: true, message: '请再次输入新密码', trigger: 'blur' },
      {
        validator: (_rule: any, value: string, callback: (e?: Error) => void) => {
          if (value !== pwdForm.newPassword) callback(new Error('两次输入的密码不一致'))
          else callback()
        },
        trigger: 'blur'
      }
    ]
  }

  /** 选择头像后立即上传，拿到地址回填表单（点「保存」才写入账号） */
  const handleAvatarPick = async (file: any) => {
    const raw = file?.raw as File | undefined
    if (!raw) return
    if (!/^image\//.test(raw.type)) {
      ElMessage.warning('请选择图片文件')
      return
    }
    uploadingAvatar.value = true
    try {
      const uploaded = await uploadFile(raw, { bizType: 'avatar' })
      if (!uploaded?.url) {
        ElMessage.error('上传失败：未返回文件地址')
        return
      }
      profileForm.avatar = uploaded.url
    } finally {
      uploadingAvatar.value = false
    }
  }

  /** 加载当前登录用户资料 */
  const loadProfile = async () => {
    const data = await fetchGetProfile()
    Object.assign(profile, data)
    Object.assign(profileForm, {
      nickname: data?.nickname ?? '',
      avatar: data?.avatar ?? '',
      mobile: data?.mobile ?? '',
      email: data?.email ?? '',
      sex: data?.sex ?? 0
    })
  }

  const saveProfile = async () => {
    if (!profileFormRef.value) return
    const valid = await profileFormRef.value.validate().catch(() => false)
    if (!valid) return
    savingProfile.value = true
    try {
      await fetchUpdateProfile({ ...profileForm })
      await loadProfile()
      // 同步顶栏展示的昵称/邮箱/手机（store 中的用户信息）
      const current = userStore.getUserInfo
      userStore.setUserInfo({
        buttons: current.buttons ?? [],
        roles: current.roles ?? [],
        userId: current.userId ?? profile.id,
        userName: profileForm.nickname,
        nickName: profileForm.nickname,
        email: profileForm.email,
        mobile: profileForm.mobile,
        avatar: profileForm.avatar,
        deptName: current.deptName,
        isSuper: current.isSuper,
        dataScope: current.dataScope
      })
    } finally {
      savingProfile.value = false
    }
  }

  const savePassword = async () => {
    if (!pwdFormRef.value) return
    const valid = await pwdFormRef.value.validate().catch(() => false)
    if (!valid) return
    savingPwd.value = true
    try {
      await fetchChangePassword({
        oldPassword: pwdForm.oldPassword,
        newPassword: pwdForm.newPassword
      })
      await ElMessageBox.alert('密码已修改，请使用新密码重新登录。', '修改成功', {
        confirmButtonText: '重新登录'
      })
      // 后端已撤销该账号全部令牌，这里直接登出回到登录页
      await userStore.logOut()
    } finally {
      savingPwd.value = false
    }
  }

  onMounted(loadProfile)
</script>
<style scoped>
  .pwd-tip {
    width: 100%;
    margin-top: 4px;
    font-size: 12px;
    line-height: 1.6;
    color: var(--art-gray-600);
  }

  .avatar-upload :deep(.el-upload) {
    display: block;
  }

  .avatar-wrap {
    position: relative;
    width: 80px;
    margin: 0 auto;
  }

  .avatar-mask {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    width: 80px;
    height: 80px;
    font-size: 11px;
    color: #fff;
    background: rgb(0 0 0 / 45%);
    border-radius: 9999px;
    opacity: 0;
    transition: opacity 0.2s;
  }

  .avatar-wrap:hover .avatar-mask {
    opacity: 1;
  }
</style>

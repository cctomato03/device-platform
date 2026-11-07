<!-- 使用文档（弹窗）：从用户菜单「使用文档」打开，不再单独占一个菜单页 -->
<template>
  <ElDialog
    v-model="visible"
    title="使用文档"
    width="860px"
    top="6vh"
    align-center
    class="user-guide-dialog"
  >
    <div class="doc-wrap">
      <ElAnchor :offset="60" :bound="20" class="doc-toc">
        <ElAnchorLink v-for="s in sections" :key="s.id" :href="`#guide-${s.id}`">
          {{ s.title }}
        </ElAnchorLink>
      </ElAnchor>

      <div class="doc-body">
        <section id="guide-start">
          <h3>1. 快速开始</h3>
          <ol>
            <li>登录后先到「系统设置 → 站点设置」把网站名称、logo、登录页文案改成自己的品牌。</li>
            <li>在「组织与权限」里建好部门 / 岗位 / 角色，再新增用户并分配角色。</li>
            <li>角色的菜单权限决定用户能看到哪些菜单与按钮，保存后用户重新登录生效。</li>
          </ol>
        </section>

        <section id="guide-workbench">
          <h3>2. 工作台与工作中心</h3>
          <ul>
            <li>工作台顶部是 4 张统计卡片；下方「我的待办 / 我的消息 / 通知公告」用页签切换，不用来回跳页面。</li>
            <li><b>待办</b>：可直接新建、完成、恢复、删除；带业务链接的待办能一键跳回业务单据。</li>
            <li><b>消息</b>：站内信，可标记已读、全部已读、删除。</li>
            <li><b>通知</b>：管理员发布的公告，点标题看详情并自动标记已读。</li>
            <li>顶栏铃铛显示三类未读数量，点条目会跳到工作台对应页签。</li>
          </ul>
        </section>

        <section id="guide-org">
          <h3>3. 组织与权限</h3>
          <ul>
            <li>用户、角色、菜单、部门、岗位是权限体系的基础数据；菜单同时承载「路由 + 按钮权限」。</li>
            <li>数据权限支持「全部 / 本部门 / 本部门及以下 / 仅本人 / 自定义部门」。</li>
          </ul>
        </section>

        <section id="guide-setting">
          <h3>4. 系统设置</h3>
          <ul>
            <li><b>站点设置</b>：网站名称、logo、登录页背景与文案、注册开关、图形验证码、滑块校验。</li>
            <li><b>参数配置</b>：键值型开关（上传大小限制、是否多租户等）。</li>
            <li><b>文件管理</b>：按「年/月/日/业务」分目录；OSS 模式浏览器直传更省带宽。</li>
            <li><b>定时任务</b>：内置处理器 / HTTP 调用 / Shell 脚本三种执行方式。</li>
            <li><b>通知公告管理</b>：起草 → 发布（扇出站内信）→ 关闭。</li>
            <li><b>消息与待办</b>：手工推送站内信、派发待办，也是业务系统对接的调试入口。</li>
          </ul>
        </section>

        <section id="guide-password">
          <h3>5. 密码规则</h3>
          <ul>
            <li>长度至少 8 位，且必须同时包含<b>大写字母、小写字母、数字</b>。</li>
            <li>不能使用常见弱口令（如 12345678、password、admin123）。</li>
            <li>不能出现 3 个及以上连续相同的字符（如 aaa、111）。</li>
            <li>修改密码后当前账号所有登录状态立即失效，需要用新密码重新登录。</li>
          </ul>
        </section>

        <section id="guide-tips">
          <h3>6. 界面与快捷键</h3>
          <ul>
            <li><kbd>Ctrl</kbd> + <kbd>K</kbd>：全局搜索菜单。</li>
            <li>右上角可切换主题（明/暗）、全屏、锁屏；列表右上角「设置」可切换斑马纹 / 边框 / 密度。</li>
          </ul>
        </section>

        <section id="guide-faq">
          <h3>7. 常见问题</h3>
          <ul>
            <li><b>保存后弹两次提示</b>：已统一由接口层提示；若仍出现请反馈具体页面。</li>
            <li><b>上传大文件很慢</b>：把存储切到 OSS 并配置 Bucket 的 CORS，浏览器会直传。</li>
            <li><b>看不到某个菜单</b>：检查角色是否勾选了该菜单权限，改完后重新登录。</li>
          </ul>
        </section>
      </div>
    </div>
  </ElDialog>
</template>

<script setup lang="ts">
  defineOptions({ name: 'ArtUserGuideDialog' })

  const visible = defineModel<boolean>({ default: false })

  const sections = [
    { id: 'start', title: '1. 快速开始' },
    { id: 'workbench', title: '2. 工作台与工作中心' },
    { id: 'org', title: '3. 组织与权限' },
    { id: 'setting', title: '4. 系统设置' },
    { id: 'password', title: '5. 密码规则' },
    { id: 'tips', title: '6. 界面与快捷键' },
    { id: 'faq', title: '7. 常见问题' }
  ]
</script>

<style scoped>
  .doc-wrap {
    display: flex;
    gap: 16px;
    max-height: 68vh;
  }

  .doc-toc {
    flex: 0 0 170px;
    overflow-y: auto;
  }

  .doc-body {
    flex: 1;
    overflow-y: auto;
    padding-right: 6px;
    font-size: 14px;
    line-height: 1.9;
    color: var(--art-gray-800);
  }

  .doc-body section + section {
    margin-top: 20px;
  }

  .doc-body h3 {
    margin: 0 0 8px;
    font-size: 15px;
    font-weight: 600;
    color: var(--art-text-gray-900);
  }

  .doc-body ul,
  .doc-body ol {
    padding-left: 20px;
  }

  .doc-body kbd {
    padding: 1px 5px;
    font-size: 12px;
    border: 1px solid var(--art-card-border);
    border-radius: 4px;
  }
</style>

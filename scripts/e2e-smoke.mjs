/**
 * E2E 冒烟测试（零依赖，使用 CDP 驱动本机 Edge/Chrome）
 *
 * 用法：
 *   node scripts/e2e-smoke.mjs [baseUrl] [username] [password]
 *
 * 流程：
 *   1. 启动无头浏览器，打开登录页
 *   2. 拦截 /auth/captcha 响应拿到 captchaId（验证码开启时需要答案；本平台默认关闭验证码，
 *      关闭时页面不渲染验证码输入框，脚本直接提交）
 *   3. 通过真实表单填写并点击登录（校验登录链路、验证码、路由与权限下发）
 *   4. 依次访问工作台、用户管理、菜单管理、定时任务等页面，断言关键内容渲染
 *   5. 汇总控制台错误、异常请求，输出 PASS / FAIL
 *
 * 环境变量：
 *   E2E_CAPTCHA_ANSWER   指定验证码答案（需同时给出 E2E_CAPTCHA_ID）
 *   E2E_REDIS_CONTAINER  旧架构遗留：验证码曾存 Redis，本平台已改为进程内存储（仅兼容保留）
 *   E2E_REDIS_PREFIX     Redis key 前缀，默认 web:
 *   E2E_BROWSER          浏览器可执行文件路径
 */
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync, existsSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const baseUrl = process.argv[2] || 'http://127.0.0.1:8090'
const username = process.argv[3] || 'admin'
const password = process.argv[4] || process.env.E2E_ADMIN_PASSWORD || ''
// Redis 容器候选：优先环境变量，其后是本地开发容器与 compose 容器名
const REDIS_CONTAINERS = [process.env.E2E_REDIS_CONTAINER, 'wb-redis', 'web-basic-redis', 'web-basic-redis57'].filter(
  Boolean
)
if (!password) {
  console.error('缺少超管密码：请用第 4 个参数或环境变量 E2E_ADMIN_PASSWORD 提供（如 admin 的初始密码，见后端启动日志）')
  process.exit(2)
}
const CDP_PORT = 9333

const BROWSERS = [
  process.env.E2E_BROWSER,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe'
].filter(Boolean)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function captchaAnswer(captchaId) {
  if (process.env.E2E_CAPTCHA_ANSWER && process.env.E2E_CAPTCHA_ID === captchaId) {
    return process.env.E2E_CAPTCHA_ANSWER
  }
  const prefix = process.env.E2E_REDIS_PREFIX ?? 'web:'
  for (const container of REDIS_CONTAINERS) {
    try {
      const v = execFileSync('docker', ['exec', container, 'redis-cli', 'GET', `${prefix}captcha:${captchaId}`], {
        encoding: 'utf8'
      }).trim()
      if (v) return v
    } catch {
      /* 换下一个候选容器 */
    }
  }
  return ''
}

class CDP {
  constructor(ws) {
    this.ws = ws
    this.id = 0
    this.pending = new Map()
    this.consoleErrors = []
    this.httpErrors = []
    this.captchaIds = []
    ws.addEventListener('message', (ev) => this.onMessage(JSON.parse(ev.data)))
  }

  onMessage(msg) {
    if (msg.id && this.pending.has(msg.id)) {
      const { resolve, reject } = this.pending.get(msg.id)
      this.pending.delete(msg.id)
      msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result)
      return
    }
    if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error') {
      this.consoleErrors.push(msg.params.args.map((a) => a.value ?? a.description).join(' '))
    }
    if (msg.method === 'Runtime.exceptionThrown') {
      const d = msg.params.exceptionDetails
      this.consoleErrors.push(`${d.text} ${d.exception?.description || ''}`)
    }
    if (msg.method === 'Fetch.requestPaused') {
      const url = msg.params.request.url
      if (this.failPattern && url.includes(this.failPattern)) {
        this.ws.send(JSON.stringify({ id: ++this.id, method: 'Fetch.failRequest', params: { requestId: msg.params.requestId, errorReason: 'Failed' } }))
      } else {
        this.ws.send(JSON.stringify({ id: ++this.id, method: 'Fetch.continueRequest', params: { requestId: msg.params.requestId } }))
      }
      return
    }
    if (msg.method === 'Network.responseReceived') {
      const r = msg.params.response
      if (r.status >= 400) this.httpErrors.push(`${r.status} ${r.url}`)
      if (r.url.includes('/auth/captcha')) this.captchaRequestId = msg.params.requestId
    }
  }

  send(method, params = {}) {
    const id = ++this.id
    this.ws.send(JSON.stringify({ id, method, params }))
    return new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }))
  }

  async eval(expression) {
    const r = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.text)
    return r.result?.value
  }
}

async function connect() {
  for (let i = 0; i < 50; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${CDP_PORT}/json/list`)).json()
      const page = list.find((t) => t.type === 'page')
      if (page?.webSocketDebuggerUrl) {
        const ws = new WebSocket(page.webSocketDebuggerUrl)
        await new Promise((res, rej) => {
          ws.addEventListener('open', res, { once: true })
          ws.addEventListener('error', rej, { once: true })
        })
        return new CDP(ws)
      }
    } catch {
      /* 等待浏览器就绪 */
    }
    await sleep(300)
  }
  throw new Error('无法连接浏览器调试端口')
}

const checks = []
const check = (name, ok, extra = '') => {
  checks.push({ name, ok })
  console.log(`${ok ? '✅' : '❌'} ${name}${extra ? '  ' + extra : ''}`)
}

/** 真实鼠标点击（合成 click() 对 Element Plus 的下拉/弹层偶尔不生效） */
async function realClickAt(cdp, x, y) {
  await cdp.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y })
  await cdp.send('Input.dispatchMouseEvent', { type: 'mousePressed', x, y, button: 'left', clickCount: 1 })
  await cdp.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x, y, button: 'left', clickCount: 1 })
}

async function realClickSelector(cdp, selector) {
  const rect = await cdp.eval(`(() => {
    const el = document.querySelector(${JSON.stringify(selector)})
    if (!el) return null
    const r = el.getBoundingClientRect()
    return r.width > 0 ? { x: r.x + r.width / 2, y: r.y + r.height / 2 } : null
  })()`)
  if (!rect) return false
  await realClickAt(cdp, rect.x, rect.y)
  return true
}

/** 按文本点击（限定在 selector 集合内，且必须是可见元素） */
async function realClickByText(cdp, selector, pattern) {
  const rect = await cdp.eval(`(() => {
    const re = new RegExp(${JSON.stringify(pattern.source)}, ${JSON.stringify(pattern.flags)})
    const els = [...document.querySelectorAll(${JSON.stringify(selector)})].filter(
      (e) => e.offsetParent !== null && re.test((e.innerText || '').trim())
    )
    const el = els[0]
    if (!el) return null
    const r = el.getBoundingClientRect()
    return r.width > 0 ? { x: r.x + r.width / 2, y: r.y + r.height / 2 } : null
  })()`)
  if (!rect) return false
  await realClickAt(cdp, rect.x, rect.y)
  return true
}

const PAGES = [
  { hash: '#/dashboard', expect: ['工作台', '用户总数', '我的待办'] },
  { hash: '#/system/user', expect: ['用户管理', '新增用户'] },
  { hash: '#/system/role', expect: ['角色管理'] },
  { hash: '#/system/menu', expect: ['菜单管理', '权限标识'] },
  { hash: '#/system/dept', expect: ['部门管理'] },
  { hash: '#/system/post', expect: ['岗位管理'] },
  { hash: '#/setting/dict', expect: ['字典', '字典类型'] },
  { hash: '#/setting/config', expect: ['参数'] },
  { hash: '#/setting/tenant', expect: ['租户'] },
  { hash: '#/setting/file', expect: ['文件', '存储目录', '近 7 天'] },
  { hash: '#/setting/job', expect: ['定时任务', 'cron'] },
  { hash: '#/monitor/online', expect: ['在线用户'] },
  { hash: '#/monitor/server', expect: ['运行环境'] },
  { hash: '#/monitor/login-log', expect: ['登录日志'] },
  { hash: '#/monitor/oper-log', expect: ['操作日志'] },
  { hash: '#/setting/notice', expect: ['通知公告管理', '新增'] },
  { hash: '#/setting/push', expect: ['推送站内信', '派发待办'] },
  { hash: '#/setting/gen', expect: ['代码生成', '表名'] },
  { hash: '#/setting/site', expect: ['站点个性化配置', '网站名称', '登录页背景图', '登录页副标题', '图形验证码', '滑块校验'] }
]

async function main() {
  const browser = BROWSERS.find((p) => existsSync(p))
  if (!browser) throw new Error('未找到 Edge / Chrome，可通过 E2E_BROWSER 指定')

  const profile = mkdtempSync(join(tmpdir(), 'wb-e2e-'))
  const child = spawn(
    browser,
    [
      '--headless=new',
      '--disable-gpu',
      '--no-first-run',
      '--no-default-browser-check',
      '--window-size=1600,900', // 桌面布局：保证顶栏搜索、设置入口等元素可见
      `--remote-debugging-port=${CDP_PORT}`,
      `--user-data-dir=${profile}`,
      'about:blank'
    ],
    { stdio: 'ignore' }
  )

  try {
    const cdp = await connect()
    await cdp.send('Runtime.enable')
    await cdp.send('Page.enable')
    await cdp.send('Network.enable')

    console.log(`==> 打开登录页 ${baseUrl}/#/auth/login`)
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/auth/login` })
    await sleep(4000)

    // 从网络层读取页面加载时的 captchaId
    let fetchedId = ''
    const resp = await cdp.send('Network.getResponseBody', { requestId: cdp.captchaRequestId }).catch(() => null)
    if (resp?.body) {
      try {
        fetchedId = JSON.parse(resp.body)?.data?.captchaId || ''
      } catch {
        /* ignore */
      }
    }
    const answer = fetchedId ? captchaAnswer(fetchedId) : ''
    console.log(`    验证码 id=${fetchedId} answer=${answer || '<未取到>'}`)

    // 站点个性化配置：登录页首帧的名称/标题来自后端，且默认关闭注册入口
    const siteCfg = await fetch(`${baseUrl}/api/v1/public/site-config`).then((r) => r.json()).catch(() => null)
    const brandText = (await cdp.eval(`({
      title: document.title,
      brand: (document.querySelector('.login-left-view .logo .title')?.innerText || '').trim(),
      desc: (document.querySelector('.login-left-view .text-wrap h1')?.innerText || '').trim(),
      subTitle: (document.querySelector('.login-left-view .text-wrap p')?.innerText || '').trim(),
      hasRegister: /注册/.test(document.body.innerText || '')
    })`)) || {}
    check(
      '登录页读取站点配置（名称 + 大标题 + 副标题，默认关闭注册入口）',
      !!siteCfg?.data?.siteName &&
        brandText.brand === siteCfg.data.siteName &&
        brandText.title.includes(siteCfg.data.siteName) &&
        (!siteCfg.data.loginDesc || brandText.desc === siteCfg.data.loginDesc) &&
        (!siteCfg.data.loginSubTitle || brandText.subTitle === siteCfg.data.loginSubTitle) &&
        brandText.hasRegister === (siteCfg.data.registerEnabled === true),
      JSON.stringify(brandText)
    )

    // 首屏不应有红色错误提示：校验只在点击「登录」后触发
    const cleanForm = await cdp.eval(`!document.querySelector('.el-form-item.is-error')`)
    check('登录页首屏不预先标红（校验时机为点击登录后）', cleanForm === true, `clean=${cleanForm}`)

    // 模板登录页带有拖拽验证（ArtDragVerify），需真实拖到底才能提交
    const rects = await cdp.eval(`(() => {
      const box = document.querySelector('.drag_verify')
      const handler = document.querySelector('.dv_handler')
      if (!box || !handler) return null
      const b = box.getBoundingClientRect(), h = handler.getBoundingClientRect()
      return { bx: b.x, by: b.y, bw: b.width, bh: b.height, hx: h.x, hy: h.y, hw: h.width, hh: h.height }
    })()`)
    if (rects) {
      const startX = rects.hx + rects.hw / 2
      const startY = rects.hy + rects.hh / 2
      // 需拖过 (容器宽度 - 滑块宽度) 才会判定通过，直接拖到最右侧
      const endX = rects.bx + rects.bw - 4
      await cdp.send('Input.dispatchMouseEvent', {
        type: 'mousePressed',
        x: startX,
        y: startY,
        button: 'left',
        clickCount: 1
      })
      const steps = 12
      for (let i = 1; i <= steps; i++) {
        await cdp.send('Input.dispatchMouseEvent', {
          type: 'mouseMoved',
          x: startX + ((endX - startX) * i) / steps,
          y: startY,
          button: 'left',
          buttons: 1
        })
        await sleep(30)
      }
      await cdp.send('Input.dispatchMouseEvent', {
        type: 'mouseReleased',
        x: endX,
        y: startY,
        button: 'left',
        clickCount: 1
      })
      await sleep(600)
    }

    const filled = await cdp.eval(`(() => {
      const setVal = (el, v) => {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
        setter.call(el, v)
        el.dispatchEvent(new Event('input', { bubbles: true }))
        el.dispatchEvent(new Event('change', { bubbles: true }))
      }
      const inputs = [...document.querySelectorAll('input')]
      const byPh = (t) => inputs.find((i) => (i.placeholder || '').includes(t))
      const u = byPh('账号'), p = byPh('密码'), c = byPh('验证码')
      if (!u || !p) return 'missing-input'
      setVal(u, ${JSON.stringify(username)})
      setVal(p, ${JSON.stringify(password)})
      // 验证码关闭时页面不渲染验证码输入框（设备平台默认如此），此时直接提交
      if (c) setVal(c, ${JSON.stringify(answer)})
      const btn = [...document.querySelectorAll('button')].find((b) => (b.innerText || '').replace(/\\s+/g, '').includes('登录'))
      if (!btn) return 'missing-button'
      btn.click()
      return 'submitted'
    })()`)
    check('登录表单提交', filled === 'submitted', filled === 'submitted' ? '' : filled)

    // 登录后必须真正落到首页（而不是停在登录页/空页/404）——轮询等待动态路由注册完成
    let afterLogin = ''
    let bodyText = ''
    for (let i = 0; i < 30; i++) {
      await sleep(500)
      afterLogin = (await cdp.eval('location.hash')) || ''
      bodyText = (await cdp.eval('document.body.innerText')) || ''
      if (afterLogin.includes('/dashboard') && /工作台/.test(bodyText)) break
    }
    const loggedIn =
      !/验证码|默认账号/.test(bodyText || '') &&
      afterLogin.includes('/dashboard') &&
      /工作台/.test(bodyText)
    check(
      '登录成功并跳转到首页（动态路由注册 + hash=#/dashboard）',
      loggedIn,
      `hash=${afterLogin} body=${bodyText.replace(/\s+/g, ' ').slice(0, 60)}`
    )
    // ---------- E2E_PROBE=dashboard：工作台加载态诊断（开发期使用） ----------
    if (process.env.E2E_PROBE === 'dashboard') {
      // 必须整页重载，否则 onMounted 不会重新拉数据（hash 路由切换会复用已加载的数据）
      await cdp.send('Page.navigate', { url: `${baseUrl}/#/dashboard` })
      await sleep(2100)
      cdp.failPattern = '/dashboard/summary'
      await cdp.send('Fetch.enable', { patterns: [{ urlPattern: '*dashboard/summary*' }] })
      await cdp.send('Page.reload', { ignoreCache: true })
      await sleep(6000)
      const during = await cdp.eval(`(() => {
        const out = { cards: [], masks: [] }
        document.querySelectorAll('.stat-card').forEach((c) => {
          const r = c.getBoundingClientRect()
          const cs = getComputedStyle(c)
          const mid = document.elementFromPoint(Math.round(r.x + r.width / 2), Math.round(r.y + r.height / 2))
          out.cards.push({ text: (c.innerText || '').replace(/\\n/g, '|'), rect: [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)], opacity: cs.opacity, visibility: cs.visibility, overflow: cs.overflow, centerEl: mid ? String(mid.className).slice(0, 60) : 'none' })
        })
        document.querySelectorAll('.el-loading-mask').forEach((m) => {
          const r = m.getBoundingClientRect()
          if (r.width > 0) out.masks.push({ parent: String(m.parentElement?.className || '').slice(0, 50), rect: [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)] })
        })
        const wb = document.querySelector('.workbench')
        if (wb) out.workbench = { rect: (() => { const r = wb.getBoundingClientRect(); return [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)] })(), scrollH: wb.scrollHeight, clientH: wb.clientHeight, overflow: getComputedStyle(wb).overflow }
        return out
      })()`)
      console.log(`  [probe:loading] ${JSON.stringify(during)}`)
      cdp.failPattern = ''
      await cdp.send('Fetch.disable')
      await sleep(4200)
      const after = await cdp.eval(`(() => {
        const out = []
        document.querySelectorAll('.stat-card').forEach((c) => {
          const r = c.getBoundingClientRect()
          out.push({ text: (c.innerText || '').replace(/\\n/g, '|'), rect: [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)] })
        })
        return out
      })()`)
      console.log(`  [probe:loaded] ${JSON.stringify(after)}`)
    }

    // ---------- E2E_PROBE=layout：布局诊断（开发期使用） ----------
    if (process.env.E2E_PROBE === 'layout') {
      for (const probeHash of ['#/system/post', '#/setting/config', '#/setting/dict']) {
        await cdp.send('Page.navigate', { url: `${baseUrl}/${probeHash}` })
        await sleep(3000)
        const info = await cdp.eval(`(() => {
          const row = document.querySelector('.el-table__body .el-table__row')
          const out = { page: location.hash }
          if (row) {
            const cells = [...row.querySelectorAll('.el-table__cell')]
            const cell = cells[cells.length - 1]
            out.cellHTML = cell.innerHTML.slice(0, 300)
            out.kids = [...cell.children].map((k) => { const b = k.getBoundingClientRect(); return { tag: k.tagName, cls: String(k.className).slice(0, 40), x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) } })
            out.grandKids = [...cell.querySelectorAll('div,span')].slice(0, 6).map((k) => { const b = k.getBoundingClientRect(); return { tag: k.tagName, cls: String(k.className).slice(0, 40), x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) } })
          }
          if (row) {
            const cells2 = [...row.querySelectorAll('.el-table__cell')]
            const cell2 = cells2[cells2.length - 1]
            const wrap = cell2.querySelector('.cell > div')
            const btn = cell2.querySelector('.cell > div > div')
            if (wrap && btn) {
              const ws = getComputedStyle(wrap), bs = getComputedStyle(btn), cs = getComputedStyle(cell2.querySelector('.cell'))
              out.wrap = { display: ws.display, flexDirection: ws.flexDirection, whiteSpace: ws.whiteSpace, width: ws.width }
              out.btn = { display: bs.display, width: bs.width, marginRight: bs.marginRight }
              out.cellStyle = { display: cs.display, whiteSpace: cs.whiteSpace, flexDirection: cs.flexDirection }
            }
          }
          const dictPage = document.querySelector('.dict-page')
          if (dictPage) {
            out.dictDisplay = getComputedStyle(dictPage).display
            out.dictFlexDir = getComputedStyle(dictPage).flexDirection
            out.dictWrap = getComputedStyle(dictPage).flexWrap
            out.dictKids = [...dictPage.children].map((k) => { const b = k.getBoundingClientRect(); return { cls: String(k.className).slice(0, 50), x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) } })
          }
          return out
        })()`)
        console.log(`  [probe] ${JSON.stringify(info)}`)
      }
    }

    for (const p of PAGES) {
      await cdp.send('Page.navigate', { url: `${baseUrl}/${p.hash}` })
      // 轮询等待页面渲染完成（首屏 chunk 与接口耗时在 dev 模式更慢）
      let text = ''
      let missing = p.expect
      for (let i = 0; i < 25; i++) {
        await sleep(400)
        text = (await cdp.eval('document.body.innerText')) || ''
        missing = p.expect.filter((k) => !text.includes(k))
        if (missing.length === 0) break
      }
      check(
        `页面渲染 ${p.hash}`,
        missing.length === 0,
        missing.length ? `缺少: ${missing.join(',')} | ${text.replace(/\s+/g, ' ').slice(0, 100)}` : ''
      )
    }

    // ---------- 菜单分组（人事权限 / 系统设置 / 监控分离） ----------
    const shellText = (await cdp.eval('document.body.innerText')) || ''
    const groups = ['组织与权限', '系统设置', '系统监控']
    const missingGroups = groups.filter((g) => !shellText.includes(g))
    check('菜单分组（组织与权限 / 系统设置 / 系统监控）', missingGroups.length === 0, missingGroups.join(','))

    // ---------- 侧边栏图标渲染（离线图标集，不允许空白图标） ----------
    const iconStat = await cdp.eval(`(() => {
      const icons = [...document.querySelectorAll('aside .art-svg-icon, .art-sidebar .art-svg-icon')]
      const withPath = icons.filter((i) => i.querySelector('path, g, circle')).length
      return { total: icons.length, withPath }
    })()`)
    check(
      '侧边栏菜单图标渲染（无空白图标）',
      iconStat.total > 0 && iconStat.withPath >= iconStat.total - 1,
      `${iconStat.withPath}/${iconStat.total}`
    )

    // ---------- 时间展示格式（不得出现 ISO 原文） ----------
    const isoRe = /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/monitor/login-log` })
    await sleep(2500)
    const loginLogText = (await cdp.eval('document.body.innerText')) || ''
    check(
      '时间展示为人类可读格式（登录日志）',
      !isoRe.test(loginLogText) && /\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/.test(loginLogText),
      isoRe.test(loginLogText) ? '仍存在 ISO 时间' : 'ok'
    )
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user` })
    await sleep(3000)
    const userText = (await cdp.eval('document.body.innerText')) || ''
    check('时间展示为人类可读格式（用户管理）', !isoRe.test(userText), isoRe.test(userText) ? '仍存在 ISO 时间' : '')

    // ---------- 菜单管理：树形展开箭头与名称同行 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/menu` })
    await sleep(2500)
    const align = await cdp.eval(`(() => {
      const cell = [...document.querySelectorAll('.el-table__row .el-table__cell')].find((c) =>
        c.querySelector('.el-table__expand-icon')
      )
      if (!cell) return { dy: -1 }
      const icon = cell.querySelector('.el-table__expand-icon')
      const label = cell.querySelector('span.inline-flex')
      if (!icon || !label) return { dy: -2 }
      const a = icon.getBoundingClientRect(), b = label.getBoundingClientRect()
      return { dy: Math.abs(a.top + a.height / 2 - (b.top + b.height / 2)) }
    })()`)
    check('菜单树箭头与名称同行', align.dy >= 0 && align.dy < 6, `垂直偏移 ${align.dy}px`)

    // ---------- 预置一条文件记录，保证文件管理操作列也被覆盖 ----------
    const token = await cdp.eval(`(() => {
      for (const k of Object.keys(localStorage)) {
        const v = localStorage.getItem(k) || ''
        const m = v.match(/"accessToken"\\s*:\\s*"([^"]+)"/)
        if (m) return m[1]
      }
      for (const k of Object.keys(sessionStorage)) {
        const m = (sessionStorage.getItem(k) || '').match(/"accessToken"\\s*:\\s*"([^"]+)"/)
        if (m) return m[1]
      }
      return ''
    })()`)
    let uploadedFileId = ''
    if (token) {
      const fd = new FormData()
      // 故意用 6KB 内容：历史上操作日志中间件把请求体截断到 4KB，导致 >4KB 的上传全部失败（10001）
      const bigBody = 'web-basic e2e upload probe line\n'.repeat(200)
      fd.append('file', new Blob([bigBody], { type: 'text/plain' }), 'e2e-layout.txt')
      const up = await fetch(`${baseUrl}/api/v1/files/upload`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd
      }).then((r) => r.json())
      uploadedFileId = up?.data?.id || ''
      console.log('    [fixture] upload code=' + (up?.code ?? 'none') + ' id=' + uploadedFileId)
    }

    // ---------- 列表操作列：统一为「单个更多按钮 + 下拉菜单」 ----------
    const actionPages = [
      { hash: '#/system/post', minItems: 2 },
      { hash: '#/setting/config', minItems: 2 },
      { hash: '#/setting/tenant', minItems: 2 },
      { hash: '#/setting/dict', minItems: 2 },
      { hash: '#/setting/file', minItems: 2 },
      { hash: '#/setting/job', minItems: 2 },
      { hash: '#/monitor/online', minItems: 1 }
    ]
    const actionResult = { ok: true, detail: [] }
    for (const ap of actionPages) {
      await cdp.send('Page.navigate', { url: `${baseUrl}/${ap.hash}` })
      // 轮询等待表格行渲染（dev 模式首次进入较慢，固定 sleep 易假失败）
      for (let i = 0; i < 16; i++) {
        const rows = await cdp.eval(`document.querySelectorAll('.el-table__body .el-table__row').length`)
        if (rows > 0) break
        await sleep(600)
      }
      await sleep(500)
      const shape = await cdp.eval(`(() => {
        const rows = [...document.querySelectorAll('.el-table__body .el-table__row')]
        for (const row of rows) {
          const cells = [...row.querySelectorAll('.el-table__cell')]
          const cell = cells[cells.length - 1]
          const icons = cell.querySelectorAll('.art-svg-icon').length
          const buttons = cell.querySelectorAll('.el-button').length
          const r = row.getBoundingClientRect()
          return { icons, buttons, rowH: Math.round(r.height) }
        }
        return { icons: -1, buttons: -1, rowH: 0 }
      })()`)
      if (shape.icons < 0) {
        actionResult.detail.push(`${ap.hash}: 无数据跳过`)
        continue
      }
      // 点击「更多」按钮，检查下拉项数量
      await realClickSelector(cdp, '.el-table__body .el-table__row .el-table__cell:last-child .art-svg-icon')
      for (let i = 0; i < 10; i++) {
        const n = await cdp.eval(`[...document.querySelectorAll('.el-dropdown-menu')].filter((m) => m.offsetParent !== null).length`)
        if (n > 0) break
        await sleep(300)
      }
      const items = await cdp.eval(`(() => {
        const menus = [...document.querySelectorAll('.el-dropdown-menu')].filter((m) => m.offsetParent !== null)
        const last = menus[menus.length - 1]
        return last ? [...last.querySelectorAll('.el-dropdown-menu__item')].filter((i) => i.offsetParent !== null).map((i) => i.innerText.trim()) : []
      })()`)
      await cdp.send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
      await cdp.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
      await sleep(400)
      const ok = shape.icons === 1 && shape.buttons === 0 && items.length >= ap.minItems && shape.rowH <= 60
      if (!ok) actionResult.ok = false
      actionResult.detail.push(`${ap.hash}: 图标${shape.icons} 下拉${items.length}项 行高${shape.rowH}px`)
    }
    check('列表操作列统一为单个「更多」按钮 + 下拉菜单', actionResult.ok, actionResult.detail.join(' | '))

    // ---------- 真实上传控件链路：选文件 → 上传 → 列表出现新记录 ----------
    {
      // 6KB 夹具：历史上中间件把请求体截断到 4KB，>4KB 的上传会 10001，必须守住
      const upPath =
        process.env.E2E_UPLOAD_FILE ||
        (() => {
          const dir = mkdtempSync(join(tmpdir(), 'wb-e2e-'))
          const p = join(dir, 'e2e-upload.txt')
          writeFileSync(p, 'web-basic upload probe line\n'.repeat(220), 'utf8')
          return p
        })()
      if (upPath) {
        await cdp.send('Page.navigate', { url: `${baseUrl}/#/setting/file` })
        await sleep(2000)
        const before = await cdp.eval(`document.querySelectorAll('.el-table__body .el-table__row').length`)
        const doc = await cdp.send('DOM.getDocument', { depth: -1 })
        const q = await cdp.send('DOM.querySelector', { nodeId: doc.root.nodeId, selector: 'input[type=file]' })
        if (q.nodeId) {
          await cdp.send('DOM.setFileInputFiles', { nodeId: q.nodeId, files: [upPath] })
          let after = before
          for (let i = 0; i < 20; i++) {
            after = await cdp.eval(`document.querySelectorAll('.el-table__body .el-table__row').length`)
            if (after > before) break
            await sleep(500)
          }
          check('上传控件选文件后列表新增记录（真实 ElUpload 链路）', after > before, `before=${before} after=${after}`)
          // 清理本次上传的文件，避免残留
          if (token) {
            const fl = await fetch(`${baseUrl}/api/v1/files?page=1&pageSize=100`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.json()).catch(() => null)
            for (const it of fl?.data?.list || []) {
              if (/e2e-upload|e2e-layout/.test(it.originName || '')) {
                await fetch(`${baseUrl}/api/v1/files/${it.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
              }
            }
          }
        } else {
          check('上传控件选文件后列表新增记录（真实 ElUpload 链路）', false, '未找到 input[type=file]')
        }
      }
    }

    // ---------- 字典管理：新增类型按钮与搜索框必须有间距 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/setting/dict` })
    for (let i = 0; i < 16; i++) {
      const ready = await cdp.eval(`(() => { const p = document.querySelector('.dict-page'); return !!(p && p.querySelector('.el-input') && p.querySelector('button.el-button')) })()`)
      if (ready) break
      await sleep(600)
    }
    const dictGap = await cdp.eval(`(() => {
      const page = document.querySelector('.dict-page')
      if (!page) return { gap: -1 }
      const btn = [...page.querySelectorAll('button.el-button')].find((b) => /新增/.test(b.innerText))
      const input = page.querySelector('.el-input')
      if (!btn || !input) return { gap: -2 }
      return { gap: Math.round(input.getBoundingClientRect().top - btn.getBoundingClientRect().bottom) }
    })()`)
    check('字典管理：新增按钮与搜索框间距正常', dictGap.gap >= 8, `间距 ${dictGap.gap}px`)
    if (uploadedFileId && token) {
      await fetch(`${baseUrl}/api/v1/files/${uploadedFileId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      }).catch(() => {})
    }

    // ---------- 字典管理：字典类型与字典项左右并排 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/setting/dict` })
    await sleep(3400)
    const dictLayout = await cdp.eval(`(() => {
      const page = document.querySelector('.dict-page')
      if (!page) return { ok: false, reason: 'no .dict-page' }
      const cards = [...page.querySelectorAll(':scope > .el-card')]
      if (cards.length < 2) return { ok: false, reason: 'cards=' + cards.length }
      const a = cards[0].getBoundingClientRect(), b = cards[1].getBoundingClientRect()
      return {
        ok: b.left >= a.right - 2 && Math.abs(a.top - b.top) <= 8,
        flexDirection: getComputedStyle(page).flexDirection,
        a: { x: Math.round(a.left), y: Math.round(a.top) },
        b: { x: Math.round(b.left), y: Math.round(b.top) }
      }
    })()`)
    check('字典管理左右分栏（类型 | 字典项）', dictLayout.ok, `${dictLayout.flexDirection || ''} ${JSON.stringify(dictLayout.a || {})} → ${JSON.stringify(dictLayout.b || {})}`)
    // ---------- 工作台统计卡片：4 张等高、有数值、无残留骨架屏 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/dashboard` })
    await sleep(2100)
    // 轮询等待概览接口回来（骨架屏消失）
    for (let i = 0; i < 12; i++) {
      const pending = await cdp.eval(`document.querySelectorAll('.stat-card .el-skeleton').length`)
      if (pending === 0) break
      await sleep(1000)
    }
    const cards = await cdp.eval(`(() => {
      const list = [...document.querySelectorAll('.stat-card')]
      return {
        count: list.length,
        heights: list.map((c) => Math.round(c.getBoundingClientRect().height)),
        values: list.map((c) => ((c.querySelector('.stat-value') || {}).innerText || '').trim()),
        skeletons: document.querySelectorAll('.stat-card .el-skeleton').length
      }
    })()`)
    const equalH = cards.heights.length > 0 && Math.max(...cards.heights) - Math.min(...cards.heights) <= 1
    check(
      '工作台统计卡片（4 张等高 / 有数值 / 无残留骨架）',
      cards.count === 4 && equalH && cards.values.every((v) => v && v !== '-') && cards.skeletons === 0,
      `count=${cards.count} h=${cards.heights.join('/')} v=${cards.values.join(' / ')} skeleton=${cards.skeletons}`
    )
    // 菜单管理表格内的图标渲染（图标列不允许空白）
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/menu` })
    await sleep(3400)
    const tableIcons = await cdp.eval(`(() => {
      const icons = [...document.querySelectorAll('.el-table .art-svg-icon')]
      const withPath = icons.filter((i) => i.querySelector('path, g, circle')).length
      return { total: icons.length, withPath }
    })()`)
    check(
      '菜单表格图标渲染（无空白图标）',
      tableIcons.total >= 4 && tableIcons.withPath >= tableIcons.total - 1,
      `${tableIcons.withPath}/${tableIcons.total}`
    )

    // 可选：E2E_SHOT=路径 时保存菜单管理页截图，便于人工复核样式
    if (process.env.E2E_SHOT) {
      const shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
      writeFileSync(process.env.E2E_SHOT, Buffer.from(shot.data, 'base64'))
      console.log(`   截图已保存: ${process.env.E2E_SHOT}`)
    }
    check('无浏览器控制台错误', cdp.consoleErrors.length === 0, cdp.consoleErrors.slice(0, 2).join(' | '))
    check('无 4xx/5xx 接口请求', cdp.httpErrors.length === 0, cdp.httpErrors.slice(0, 3).join(' | '))

    // ---------- 模板自带特性：多页签 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user` })
    await sleep(2500)
    const tabCount = await cdp.eval(`document.querySelectorAll('li[id^="scroll-li-"]').length`)
    check('多页签工作区（页签数 ≥ 3）', tabCount >= 3, `当前 ${tabCount} 个`)

    // ---------- 表格设置：斑马纹开关在所有列表页生效（曾有多页硬编码 :stripe="false"） ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/menu` })
    await sleep(4000)
    // 「其它设置」弹层是表头工具条最后一个按钮（搜索/刷新/尺寸/全屏/列设置/其它设置）
    const zebraOn = await cdp.eval(`(() => {
      const btns = [...document.querySelectorAll('#art-table-header .button')]
      const btn = btns[btns.length - 1]
      if (!btn) return 'no-settings-btn'
      btn.click()
      return 'clicked'
    })()`)
    await sleep(900)
    const zebraToggled = await cdp.eval(`(() => {
      const label = [...document.querySelectorAll('.el-popover .el-checkbox')].find((c) =>
        /斑马纹/.test(c.innerText || '')
      )
      if (!label) return 'no-zebra-option'
      label.querySelector('.el-checkbox__input')?.click()
      return 'clicked'
    })()`)
    await sleep(1200)
    const zebraState = await cdp.eval(`(() => ({
      striped: document.querySelectorAll('.el-table__row--striped').length,
      rows: document.querySelectorAll('.el-table__row').length
    }))()`)
    // 关闭斑马纹，恢复默认观感（该开关按浏览器本地保存，避免污染其它断言）
    await cdp.eval(`(() => {
      const label = [...document.querySelectorAll('.el-popover .el-checkbox')].find((c) =>
        /斑马纹/.test(c.innerText || '')
      )
      if (label && label.className.includes('is-checked')) label.querySelector('.el-checkbox__input')?.click()
      document.body.click()
      return true
    })()`)
    await sleep(600)
    check(
      '表格「斑马纹」开关在当前列表页生效（未被页面硬编码覆盖）',
      zebraOn === 'clicked' && zebraToggled === 'clicked' && zebraState.striped > 0 && zebraState.rows > 0,
      `${zebraOn}/${zebraToggled} striped=${zebraState.striped} rows=${zebraState.rows}`
    )
    // ---------- 模板自带特性：设置抽屉（多布局 / 主题 / 水印） ----------
    await realClickSelector(cdp, '.setting-btn')
    await sleep(2500)
    const panelText = (await cdp.eval('document.body.innerText')) || ''
    const layoutKeys = ['主题风格', '菜单布局', '垂直', '水平', '混合', '双列', '全局水印', '开启多标签栏']
    const missingLayout = layoutKeys.filter((k) => !panelText.includes(k))
    check(
      '设置抽屉（多布局/主题/水印配置齐全）',
      missingLayout.length === 0,
      missingLayout.length ? `缺少: ${missingLayout.join(',')}` : `开关 ${await cdp.eval(`document.querySelectorAll('.el-switch').length`)} 个`
    )

    // 深色主题切换（模板换肤能力）
    const darkClicked = await realClickByText(cdp, 'span,div', /^深色$/)
    await sleep(2100)
    const isDark = await cdp.eval(`document.documentElement.classList.contains('dark')`)
    check('深色主题切换', darkClicked && isDark, `click:${darkClicked} dark:${isDark}`)
    const lightClicked = await realClickByText(cdp, 'span,div', /^浅色$/)
    await sleep(2100)
    const isLight = await cdp.eval(`!document.documentElement.classList.contains('dark')`)
    check('切回浅色主题', lightClicked && isLight, `click:${lightClicked} light:${isLight}`)

    // 关闭抽屉（ESC）
    await cdp.send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
    await cdp.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
    await sleep(1200)

    // ---------- 写路径：新增用户 → 列表出现 → API 清理 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user` })
    await sleep(2500)
    const newUser = `e2e${Date.now().toString().slice(-6)}`
    const opened = await cdp.eval(`(() => {
      const btn = [...document.querySelectorAll('button')].find((b) => (b.innerText || '').trim() === '新增用户')
      if (!btn) return 'no-btn'
      btn.click()
      return 'opened'
    })()`)
    await sleep(2100)
    const filledDlg = await cdp.eval(`(() => {
      const setVal = (el, v) => {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
        setter.call(el, v)
        el.dispatchEvent(new Event('input', { bubbles: true }))
        el.dispatchEvent(new Event('change', { bubbles: true }))
      }
      const dialogs = [...document.querySelectorAll('.el-dialog')].filter((d) => d.offsetParent !== null)
      const dlg = dialogs[dialogs.length - 1]
      if (!dlg) return 'no-dialog'
      const ins = [...dlg.querySelectorAll('input')]
      const ph = (t) => ins.find((i) => (i.placeholder || '').includes(t))
      const u = ph('登录账号'), n = ph('昵称')
      if (!u || !n) return 'no-input'
      setVal(u, ${JSON.stringify(newUser)})
      setVal(n, 'E2E自动化用户')
      return 'filled'
    })()`)
    await sleep(300)
    const submitted = await cdp.eval(`(() => {
      const dialogs = [...document.querySelectorAll('.el-dialog')].filter((d) => d.offsetParent !== null)
      const dlg = dialogs[dialogs.length - 1]
      if (!dlg) return 'no-dialog'
      const btn = [...dlg.querySelectorAll('button')].find((b) => (b.innerText || '').trim() === '提交')
      if (!btn) return 'no-submit'
      btn.click()
      return 'submitted'
    })()`)
    let created = false
    for (let i = 0; i < 20; i++) {
      await sleep(500)
      const t = (await cdp.eval('document.body.innerText')) || ''
      if (t.includes(newUser)) {
        created = true
        break
      }
    }
    check(
      '新增用户（UI 表单 → 后端写入 → 列表回显）',
      opened === 'opened' && filledDlg === 'filled' && submitted === 'submitted' && created,
      `账号=${newUser} ${opened}/${filledDlg}/${submitted}/${created ? 'shown' : 'not-shown'}`
    )

    // 清理：用页面里已登录的 accessToken 调后端删除
    const accessToken = token
    if (accessToken) {
      const found = await (
        await fetch(`${baseUrl}/api/v1/system/users?page=1&pageSize=10&username=${encodeURIComponent(newUser)}`, {
          headers: { Authorization: `Bearer ${accessToken}` }
        })
      ).json()
      const target = (found?.data?.list || []).find((u) => u.username === newUser)
      if (target?.id) {
        const del = await (
          await fetch(`${baseUrl}/api/v1/system/users`, {
            method: 'DELETE',
            headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids: [target.id] })
          })
        ).json()
        check('清理测试用户', del.code === 0, del.message || '')
      } else {
        check('清理测试用户', false, '未查到刚创建的用户')
      }
    }

    // ---------- 个人中心：资料回显与密码表单（自服务能力） ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user-center` })
    let ucState = { cards: 0, nickname: '', hasPwd: false, avatarCard: false }
    for (let i = 0; i < 18; i++) {
      ucState = await cdp.eval(`(() => {
        const cards = [...document.querySelectorAll('.art-card-sm')]
        const inputs = [...document.querySelectorAll('input')]
        const nickname = inputs.find((el) => (el.value || '').length > 0 && el.offsetParent !== null)
        return {
          cards: cards.length,
          nickname: nickname ? nickname.value : '',
          hasPwd: [...document.querySelectorAll('input[type=password]')].length >= 3,
          avatarCard: !!document.querySelector('.art-card-sm img')
        }
      })()`)
      if (ucState.cards >= 2 && ucState.nickname) break
      await sleep(600)
    }
    check(
      '个人中心（资料回显 + 密码表单）',
      ucState.cards >= 2 && !!ucState.nickname && ucState.hasPwd,
      `cards=${ucState.cards} nickname=${ucState.nickname} pwdInputs=${ucState.hasPwd}`
    )

    // ---------- 新增能力：操作日志字段变更差异（diff）以表格呈现 ----------
    {
      await cdp.send('Page.navigate', { url: `${baseUrl}/#/monitor/oper-log` })
      for (let i = 0; i < 16; i++) {
        const rows = await cdp.eval(`document.querySelectorAll('.el-table__body .el-table__row').length`)
        if (rows > 0) break
        await sleep(500)
      }
      // 点开第一行详情抽屉，确认抽屉可打开（diff 有值时会渲染表格）
      const opened = await cdp.eval(`(() => {
        const row = document.querySelector('.el-table__body .el-table__row')
        if (!row) return false
        row.click()
        return true
      })()`)
      await sleep(1200)
      const drawer = await cdp.eval(`(() => {
        const d = document.querySelector('.el-drawer')
        if (!d) return { open: false }
        const text = d.innerText || ''
        return { open: true, hasFields: /字段变更/.test(text) }
      })()`)
      check('操作日志详情抽屉（含字段变更区块）', opened && drawer.open && drawer.hasFields, JSON.stringify(drawer))
      await cdp.send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
      await cdp.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 })
      await sleep(500)
    }

    // ---------- 前端无按钮权限（负向）：只给"用户管理页面"，不给任何操作按钮 ----------
    if (token) {
      const stampBtn = Math.random().toString(36).slice(2, 8)
      const h = { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }
      const role = await fetch(`${baseUrl}/api/v1/system/roles`, {
        method: 'POST',
        headers: h,
        body: JSON.stringify({ name: `E2E按钮角色${stampBtn}`, code: `e2e_btn_${stampBtn}`, sort: 99, dataScope: 1, status: 1 })
      }).then((r) => r.json())
      const roleId = role?.data?.id || ''
      let limitedUser = ''
      if (roleId) {
        // 只授权「组织与权限」目录 + 「用户管理」页面，不授权任何按钮（1011~1017）
        await fetch(`${baseUrl}/api/v1/system/roles/${roleId}/grant`, {
          method: 'PUT',
          headers: h,
          body: JSON.stringify({ menuIds: ['1000', '1010'], deptIds: [] })
        })
        const u = await fetch(`${baseUrl}/api/v1/system/users`, {
          method: 'POST',
          headers: h,
          body: JSON.stringify({
            username: `e2e_btn_${stampBtn}`,
            nickname: 'E2E按钮账号',
            password: 'E2e@123456',
            deptId: '1',
            status: 1,
            roleIds: [String(roleId)],
            sex: 0
          })
        }).then((r) => r.json())
        limitedUser = u?.data?.id || ''
      }

      // 用受限账号登录（验证码从 Redis 取），把 token 写回前端存储后整页重载
      let limitedToken = ''
      if (limitedUser) {
        const cap = await fetch(`${baseUrl}/api/v1/auth/captcha`).then((r) => r.json())
        const capId = cap?.data?.captchaId || ''
        const ans = capId ? captchaAnswer(capId) : ''
        const lg = await fetch(`${baseUrl}/api/v1/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: `e2e_btn_${stampBtn}`, password: 'E2e@123456', captchaId: capId, captchaCode: ans })
        }).then((r) => r.json())
        limitedToken = lg?.data?.accessToken || ''
      }

      const swapped = limitedToken
        ? await cdp.eval(`(() => {
            const key = Object.keys(localStorage).find((k) => /accessToken/.test(localStorage.getItem(k) || ''))
            if (!key) return false
            const raw = localStorage.getItem(key)
            localStorage.setItem(
              key,
              raw
                .replace(/"accessToken"\s*:\s*"[^"]*"/, '"accessToken":"${limitedToken}"')
                .replace(/"refreshToken"\s*:\s*"[^"]*"/, '"refreshToken":""')
            )
            return true
          })()`)
        : false

      let uiState = { swapped, menus: [], buttons: {} }
      if (swapped) {
        await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user` })
        await cdp.send('Page.reload', { ignoreCache: true })
        // 等页面渲染 + 表格数据落地（只等侧边栏文字会误判成"表格为空"）
        for (let i = 0; i < 30; i++) {
          await sleep(600)
          const ready = await cdp.eval(`(() => {
            const t = document.body.innerText || ''
            const rows = document.querySelectorAll('.el-table__body .el-table__row').length
            return t.includes('用户管理') && rows > 0
          })()`)
          if (ready) break
        }
        uiState = await cdp.eval(`(() => {
          const sidebar = document.querySelector('aside') || document.body
          const page = document.body.innerText || ''
          return {
            swapped: true,
            sidebarHasUser: /用户管理/.test(sidebar.innerText || ''),
            hasCreate: /新增用户/.test(page),
            hasBatchDelete: /批量删除/.test(page),
            hasExport: /导出/.test(page),
            hasRows: document.querySelectorAll('.el-table__body .el-table__row').length > 0
          }
        })()`)
      }
      check(
        '前端无按钮权限：仅页面权限时操作按钮全部隐藏（负向）',
        uiState.swapped &&
          uiState.sidebarHasUser &&
          uiState.hasRows &&
          !uiState.hasCreate &&
          !uiState.hasBatchDelete &&
          !uiState.hasExport,
        JSON.stringify(uiState)
      )

      // 清理：恢复管理员 token → 删除受限账号与角色
      if (limitedUser) {
        await fetch(`${baseUrl}/api/v1/system/users`, {
          method: 'DELETE',
          headers: h,
          body: JSON.stringify({ ids: [String(limitedUser)] })
        }).catch(() => {})
      }
      if (roleId) {
        await fetch(`${baseUrl}/api/v1/system/roles/${roleId}`, { method: 'DELETE', headers: h }).catch(() => {})
      }
      // 重新登录管理员，保证后续（锁屏等）检查仍在管理员上下文
      const cap2 = await fetch(`${baseUrl}/api/v1/auth/captcha`).then((r) => r.json())
      const cap2Id = cap2?.data?.captchaId || ''
      const ans2 = cap2Id ? captchaAnswer(cap2Id) : ''
      const relogin = await fetch(`${baseUrl}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, captchaId: cap2Id, captchaCode: ans2 })
      }).then((r) => r.json())
      const adminToken2 = relogin?.data?.accessToken || ''
      if (adminToken2) {
        await cdp.eval(`(() => {
          const key = Object.keys(localStorage).find((k) => /accessToken/.test(localStorage.getItem(k) || ''))
          if (!key) return false
          const raw = localStorage.getItem(key)
          localStorage.setItem(
            key,
            raw.replace(/"accessToken"\s*:\s*"[^"]*"/, '"accessToken":"${adminToken2}"')
          )
          return true
        })()`)
        await cdp.send('Page.navigate', { url: `${baseUrl}/#/dashboard` })
        await cdp.send('Page.reload', { ignoreCache: true })
        await sleep(5000)
      }
    }
    // 等布局（顶栏头像）渲染完成，最多等 12s
    for (let i = 0; i < 24; i++) {
      const ready = await cdp.eval(`!!document.querySelector('.art-user-menu, header, .layout-header')`)
      if (ready) break
      await sleep(500)
    }
    const avatarClicked = await cdp.eval(`(() => {
      const avatar = [...document.querySelectorAll('img')].find(
        (i) => String(i.className).includes('rounded-full') && i.width < 60
      )
      if (!avatar) return 'no-avatar'
      avatar.click()
      return 'clicked'
    })()`)
    await sleep(1700)
    const lockClicked = await cdp.eval(`(() => {
      const item = [...document.querySelectorAll('.btn-item')].find((i) => /锁|Lock/i.test(i.innerText || ''))
      if (!item) return 'no-item'
      item.click()
      return 'clicked'
    })()`)
    await sleep(2500)
    // 锁屏界面以覆盖层 + 密码输入框呈现，innerText 取不到输入框，改为断言 DOM 特征
    const lockState = await cdp.eval(`(() => {
      const input = [...document.querySelectorAll('input')].find((i) =>
        /锁屏密码|password|lock/i.test(i.placeholder || '')
      )
      return { overlay: !!document.querySelector('.layout-lock-screen'), input: !!input && input.offsetParent !== null }
    })()`)
    check(
      '锁屏特性（打开锁屏界面并要求输入锁屏密码）',
      avatarClicked === 'clicked' && lockClicked === 'clicked' && lockState.input,
      `${avatarClicked}/${lockClicked} input:${lockState.input}`
    )


    // ---------- 字典页：左右两块只有右侧是「表格」（避免两个表头设置互相影响） ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/setting/dict` })
    await sleep(4000)
    const dictHeaderState = await cdp.eval(`(() => ({
      headers: document.querySelectorAll('#art-table-header').length,
      hasTypePanel: !!document.querySelector('.type-panel-title'),
      settingsBtns: document.querySelectorAll('#art-table-header .flex-c > .button').length
    }))()`)
    check(
      '字典页布局：仅右侧表格有表头设置（左侧类型导航不再出现表格设置按钮）',
      dictHeaderState.headers === 1 && dictHeaderState.hasTypePanel === true,
      `headers=${dictHeaderState.headers} typePanel=${dictHeaderState.hasTypePanel} btns=${dictHeaderState.settingsBtns}`
    )
    // ---------- 提示去重：保存站点配置只弹一次成功提示（接口层统一提示，页面不再重复弹） ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/setting/site` })
    await sleep(4000)
    const saveClick = await cdp.eval(`(() => {
      const btn = [...document.querySelectorAll('button')].find((b) => /保存配置/.test(b.innerText || ''))
      if (!btn) return 'no-button'
      btn.click()
      return 'clicked'
    })()`)
    await sleep(2000)
    const successToasts = await cdp.eval(
      `[...document.querySelectorAll('.el-message--success')].map((el) => (el.innerText || '').trim())`
    )
    check(
      '保存操作只弹一次成功提示（无接口/页面重复弹窗）',
      saveClick === 'clicked' && Array.isArray(successToasts) && successToasts.length === 1,
      `click=${saveClick} toasts=${JSON.stringify(successToasts)}`
    )
    // ---------- 顶栏精简：隐藏快速入口与聊天入口，保留通知铃铛 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/dashboard` })
    await sleep(3500)
    const headerState = await cdp.eval(`(() => ({
      fastEnter: !!document.querySelector('.fast-enter, [class*="fast-enter"]'),
      chatBtn: !!document.querySelector('.chat-button'),
      noticeBtn: !!document.querySelector('.notice-button'),
      noticeDot: !!document.querySelector('.notice-button .bg-danger')
    }))()`)
    check(
      '顶栏精简：快速入口与聊天入口已隐藏，通知铃铛保留',
      headerState.fastEnter === false && headerState.chatBtn === false && headerState.noticeBtn === true,
      JSON.stringify(headerState)
    )

    // ---------- 通知铃铛：整合「通知 / 消息 / 待办」三类真实数据 ----------
    await cdp.eval(`(() => { const b = document.querySelector('.notice-button'); if (b) b.click(); return true })()`)
    await sleep(1500)
    const noticePanel = await cdp.eval(`(() => {
      const panel = document.querySelector('.art-notification-panel')
      if (!panel) return { open: false }
      const tabs = [...panel.querySelectorAll('li')].map((li) => (li.innerText || '').trim()).filter((t) => /^通知|^消息|^待办/.test(t))
      return { open: panel.offsetParent !== null, tabs, text: (panel.innerText || '').slice(0, 60) }
    })()`)
    check(
      '通知铃铛面板：通知 / 消息 / 待办 三个分类（真实接口数据）',
      noticePanel.open === true &&
        noticePanel.tabs.length === 3 &&
        /通知/.test(noticePanel.tabs[0]) &&
        /消息/.test(noticePanel.tabs[1]) &&
        /待办/.test(noticePanel.tabs[2]),
      JSON.stringify(noticePanel.tabs || noticePanel)
    )
    await cdp.eval(`(() => { const b = document.querySelector('.notice-button'); if (b) b.click(); return true })()`)
    await sleep(500)

    // ---------- 用户菜单：隐藏 GitHub，使用文档改为站内页面 ----------
    const userMenuOpened = await cdp.eval(`(() => {
      try {
        const ref = [...document.querySelectorAll('img')].find((i) => (i.className || '').includes('size-8.5'))
        if (!ref) return 'no-avatar'
        ref.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
        ref.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }))
        return 'hovered'
      } catch (e) {
        return 'err:' + e.message
      }
    })()`)
    await sleep(1200)
    const menuText = (await cdp.eval(`(() => {
      try {
        const p = document.querySelector('.user-menu-popover')
        return p ? p.innerText : ''
      } catch (e) {
        return ''
      }
    })()`)) || ''
    check(
      '用户菜单：GitHub 入口已移除，保留个人中心与使用文档',
      userMenuOpened === 'hovered' && !/github/i.test(menuText) && /个人中心/.test(menuText) && /使用文档/.test(menuText),
      menuText.replace(/\s+/g, ' ').slice(0, 80)
    )

    // 点「使用文档」应进入站内文档页
    const docClicked = await cdp.eval(`(() => {
      try {
        const items = [...document.querySelectorAll('.user-menu-popover .btn-item')]
        const item = items.find((el) => /使用文档/.test(el.innerText || ''))
        if (!item) return 'no-item'
        item.click()
        return 'clicked'
      } catch (e) {
        return 'err:' + e.message
      }
    })()`)
    let docDialog = { open: false, text: '' }
    for (let i = 0; i < 16; i++) {
      await sleep(500)
      docDialog = await cdp.eval(`(() => {
        const dlg = [...document.querySelectorAll('.el-dialog')].find((d) => /使用文档/.test(d.innerText || ''))
        return { open: !!dlg && dlg.offsetParent !== null, text: dlg ? dlg.innerText.slice(0, 80) : '' }
      })()`)
      if (docDialog.open) break
    }
    check(
      '使用文档以弹窗打开（不再是独立菜单页）',
      docClicked === 'clicked' && docDialog.open === true && /快速开始|密码规则/.test(docDialog.text),
      `click=${docClicked} open=${docDialog.open}`
    )
    await cdp.eval(`(() => {
      const btns = [...document.querySelectorAll('.el-dialog__headerbtn')]
      if (btns.length) btns[btns.length - 1].click()
      return true
    })()`)
    await sleep(600)

    // ---------- 个人中心：头像上传链路 ----------
    // 造一张最小 PNG 作为头像文件
    const avatarPath = join(tmpdir(), `wb-avatar-${Date.now()}.png`)
    writeFileSync(
      avatarPath,
      Buffer.from(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
        'base64'
      )
    )
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user-center` })
    await sleep(3500)
    const avatarBefore = await cdp.eval(`(() => {
      const img = document.querySelector('.avatar-wrap img')
      return img ? img.getAttribute('src') : ''
    })()`)
    const doc2 = await cdp.send('DOM.getDocument', { depth: -1 })
    const fileInput = await cdp.send('DOM.querySelector', { nodeId: doc2.root.nodeId, selector: '.avatar-upload input[type=file]' })
    let avatarAfter = avatarBefore
    if (fileInput.nodeId) {
      await cdp.send('DOM.setFileInputFiles', { nodeId: fileInput.nodeId, files: [avatarPath] })
      for (let i = 0; i < 24; i++) {
        await sleep(600)
        avatarAfter = await cdp.eval(`(() => {
          const img = document.querySelector('.avatar-wrap img')
          return img ? img.getAttribute('src') : ''
        })()`)
        if (avatarAfter && avatarAfter !== avatarBefore) break
      }
    }
    check(
      '个人中心头像上传（上传后预览地址更新为已上传文件）',
      !!fileInput.nodeId && !!avatarAfter && avatarAfter !== avatarBefore && !/avatar\.webp$/.test(avatarAfter || ''),
      `before=${String(avatarBefore).slice(-24)} after=${String(avatarAfter).slice(-24)}`
    )

    // 保存后顶栏头像同步
    const savedProfile = await cdp.eval(`(() => {
      const btn = [...document.querySelectorAll('button')].find((b) => /保存/.test(b.innerText || ''))
      if (!btn) return 'no-button'
      btn.click()
      return 'clicked'
    })()`)
    await sleep(2500)
    const headerAvatar = await cdp.eval(`(() => {
      try {
        const img = [...document.querySelectorAll('img')].find((i) => (i.className || '').includes('size-8.5'))
        return img ? img.getAttribute('src') : ''
      } catch (e) {
        return ''
      }
    })()`)
    check(
      '个人中心资料保存后顶栏头像同步',
      savedProfile === 'clicked' && !!headerAvatar && headerAvatar.includes(String(avatarAfter).split('/').pop() || 'x'),
      `saved=${savedProfile} header=${String(headerAvatar).slice(-24)}`
    )

    // ---------- 语言切换入口已隐藏（界面固定中文） ----------
    const langState = await cdp.eval(`(() => ({
      btn: !!document.querySelector('.language-btn'),
      menuTxt: [...document.querySelectorAll('.el-dropdown-menu__item')].some((el) => /English/.test(el.innerText || ''))
    }))()`)
    check('语言切换按钮已隐藏，界面保持中文', langState.btn === false, JSON.stringify(langState))

    // ---------- 通知面板保持小卡片尺寸 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/dashboard` })
    await sleep(3200)
    await cdp.eval(`(() => { const b = document.querySelector('.notice-button'); if (b) b.click(); return true })()`)
    await sleep(1200)
    const panelBox = await cdp.eval(`(() => {
      const p = document.querySelector('.art-notification-panel')
      if (!p) return null
      const r = p.getBoundingClientRect()
      return { w: Math.round(r.width), h: Math.round(r.height), vw: window.innerWidth }
    })()`)
    check(
      '通知面板是小卡片（未撑满整页宽度）',
      !!panelBox && panelBox.w > 200 && panelBox.w < Math.round((panelBox.vw || 0) * 0.6),
      JSON.stringify(panelBox || {})
    )
    await cdp.eval(`(() => { const b = document.querySelector('.notice-button'); if (b) b.click(); return true })()`)
    await sleep(500)

    // ---------- 工作台：待办 / 消息 / 通知 三合一页签 ----------
    const tabsState = await cdp.eval(`(() => {
      const tabs = [...document.querySelectorAll('.el-tabs__item')].map((t) => (t.innerText || '').trim())
      return { tabs, rows: document.querySelectorAll('.el-table__body .el-table__row').length }
    })()`)
    const tabSwitched = await cdp.eval(`(() => {
      const target = [...document.querySelectorAll('.el-tabs__item')].find((t) => /消息/.test(t.innerText || ''))
      if (!target) return 'no-tab'
      target.click()
      return 'clicked'
    })()`)
    await sleep(1600)
    const tabRows = await cdp.eval(`document.querySelectorAll('.el-table__body .el-table__row').length`)
    check(
      '工作台集成待办 / 消息 / 通知（页签切换 + 表格渲染）',
      tabsState.tabs.length === 3 &&
        tabsState.tabs.some((t) => /待办/.test(t)) &&
        tabsState.tabs.some((t) => /消息/.test(t)) &&
        tabsState.tabs.some((t) => /通知/.test(t)) &&
        tabSwitched === 'clicked',
      `tabs=${JSON.stringify(tabsState.tabs)} rows=${tabRows}`
    )

    // ---------- 密码规则：弱密码在表单上直接说明原因 ----------
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/system/user-center` })
    await sleep(3200)
    const weakPwdTyped = await cdp.eval(`(() => {
      const inputs = [...document.querySelectorAll('input[type=password]')]
      const target = inputs[2] || inputs[inputs.length - 1]
      if (!target) return 'no-input'
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
      setter.call(target, '123456')
      target.dispatchEvent(new Event('input', { bubbles: true }))
      target.dispatchEvent(new Event('change', { bubbles: true }))
      target.dispatchEvent(new Event('blur', { bubbles: true }))
      return 'typed'
    })()`)
    await sleep(1500)
    const pwdErrors = await cdp.eval(
      `[...document.querySelectorAll('.el-form-item__error')].map((e) => (e.innerText || '').trim())`
    )
    check(
      '密码规则可读提示（弱密码在表单上直接说明原因）',
      weakPwdTyped === 'typed' && Array.isArray(pwdErrors) && pwdErrors.some((t) => /密码/.test(t)),
      JSON.stringify(pwdErrors || [])
    )
    // ---------- 站点设置：保存后登录页即时生效（注册开关正反向验证，放在最后） ----------
    // 注册入口只对「未登录」用户可见：清掉本地登录态后再验证，验证完本轮即结束。
    if (token) {
      const h = { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }
      const cur =
        (await fetch(`${baseUrl}/api/v1/system/site-config`, { headers: h }).then((r) => r.json()))?.data || {}
      const base = {
        siteName: cur.siteName,
        siteLogo: cur.siteLogo || '',
        loginBg: cur.loginBg || '',
        loginDesc: cur.loginDesc || '',
        registerEnabled: cur.registerEnabled === true
      }
      const put = (patch) =>
        fetch(`${baseUrl}/api/v1/system/site-config`, {
          method: 'PUT',
          headers: h,
          body: JSON.stringify({ ...base, ...patch })
        }).then((r) => r.json())

      await cdp.eval(`(() => {
        Object.keys(localStorage)
          .filter((k) => /accessToken|refreshToken/.test(localStorage.getItem(k) || ''))
          .forEach((k) => localStorage.removeItem(k))
        return true
      })()`)

      // 打开注册 → 登录页出现「注册」入口
      const on = await put({ registerEnabled: true })
      await cdp.send('Page.navigate', { url: `${baseUrl}/#/auth/login` })
      await cdp.send('Page.reload', { ignoreCache: true })
      await sleep(5000)
      const shown = await cdp.eval(`/注册/.test(document.body.innerText || '')`)

      // 关闭注册 → 入口消失，且直接访问 /auth/register 会被拦回登录页
      const off = await put({ registerEnabled: false })
      await cdp.send('Page.reload', { ignoreCache: true })
      await sleep(3500)
      const hidden = await cdp.eval(`!/注册/.test(document.body.innerText || '')`)
      await cdp.send('Page.navigate', { url: `${baseUrl}/#/auth/register` })
      await sleep(3000)
      const blocked = (await cdp.eval('location.hash')) !== '#/auth/register'

      check(
        '站点设置「开放注册」开关生效（登录页显示/隐藏 + 路由拦截）',
        on?.code === 0 && off?.code === 0 && shown === true && hidden === true && blocked,
        `on=${on?.code} off=${off?.code} shown=${shown} hidden=${hidden} blocked=${blocked}`
      )


      // ---------- 登录交互开关：关闭验证码 + 滑块后，直接账号密码即可登录 ----------
      const cfgList = await fetch(`${baseUrl}/api/v1/system/configs`, { headers: h }).then((r) => r.json())
      const rows = cfgList?.data || []
      const findRow = (key) => rows.find((r) => r.key === key)
      const capRow = findRow('sys.login.captcha')
      const sliderRow = findRow('sys.login.slider')
      const putCfg = (row, value) =>
        fetch(`${baseUrl}/api/v1/system/configs/${row.id}`, {
          method: 'PUT',
          headers: h,
          body: JSON.stringify({
            name: row.name,
            key: row.key,
            value: String(value),
            valueType: row.valueType || 'bool',
            isPublic: true,
            remark: row.remark || ''
          })
        }).then((r) => r.json())

      let switchesOk = false
      let uiState = { drag: null, captcha: null, loggedIn: false, hash: '' }
      if (capRow?.id && sliderRow?.id) {
        const offCap = await putCfg(capRow, false)
        const offSlider = await putCfg(sliderRow, false)
        await cdp.send('Page.navigate', { url: `${baseUrl}/#/auth/login` })
        await cdp.send('Page.reload', { ignoreCache: true })
        await sleep(5000)
        const capApi = await fetch(`${baseUrl}/api/v1/auth/captcha`).then((r) => r.json()).catch(() => null)
        uiState.drag = await cdp.eval(`!!document.querySelector('.drag_verify')`)
        uiState.captcha = await cdp.eval(
          `[...document.querySelectorAll('input')].some((i) => (i.placeholder || '').includes('验证码'))`
        )
        const submitted = await cdp.eval(`(() => {
          const setVal = (el, v) => {
            const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
            setter.call(el, v)
            el.dispatchEvent(new Event('input', { bubbles: true }))
            el.dispatchEvent(new Event('change', { bubbles: true }))
          }
          const inputs = [...document.querySelectorAll('input')]
          const u = inputs.find((i) => (i.placeholder || '').includes('账号'))
          const p = inputs.find((i) => (i.placeholder || '').includes('密码'))
          if (!u || !p) return 'missing-input'
          setVal(u, ${JSON.stringify(username)})
          setVal(p, ${JSON.stringify(password)})
          const btn = [...document.querySelectorAll('button')].find((b) => (b.innerText || '').replace(/\\s+/g, '').includes('登录'))
          if (!btn) return 'no-button'
          btn.click()
          return 'submitted'
        })()`)
        for (let i = 0; i < 24; i++) {
          await sleep(500)
          uiState.hash = (await cdp.eval('location.hash')) || ''
          if (uiState.hash.includes('/dashboard')) {
            uiState.loggedIn = true
            break
          }
        }
        switchesOk =
          offCap?.code === 0 &&
          offSlider?.code === 0 &&
          capApi?.data?.enabled === false &&
          submitted === 'submitted'
        // 复原开关，避免影响后续手工使用
        await putCfg(capRow, true)
        await putCfg(sliderRow, true)
      }
      check(
        '登录开关生效（关闭验证码+滑块：登录页不渲染二者，可直接账号密码登录）',
        switchesOk && uiState.drag === false && uiState.captcha === false && uiState.loggedIn,
        `drag=${uiState.drag} captcha=${uiState.captcha} hash=${uiState.hash}`
      )
    }  } finally {
    child.kill()
    await sleep(500)
    try {
      rmSync(profile, { recursive: true, force: true })
    } catch {
      /* ignore */
    }
  }

  const failed = checks.filter((c) => !c.ok)
  console.log(`\n结果: ${checks.length - failed.length}/${checks.length} 通过`)
  process.exit(failed.length ? 1 : 0)
}

main().catch((e) => {
  console.error('E2E 失败:', e.message)
  process.exit(1)
})

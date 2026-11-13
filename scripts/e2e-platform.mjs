/**
 * 设备平台页面冒烟（零依赖，CDP 驱动本机 Edge/Chrome）
 *
 * 覆盖本次新增的 17 个页面：登录 → 逐个路由打开 → 断言关键内容渲染、
 * 且没有控制台报错 / 4xx-5xx 接口（插件与日志页允许空数据）。
 *
 * 用法：
 *   node scripts/e2e-platform.mjs [baseUrl] [username] [password]
 *
 * 环境变量：
 *   E2E_ADMIN_PASSWORD  超管密码（也可用第 4 个参数）
 *   E2E_BROWSER         浏览器路径
 */
import { spawn } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const baseUrl = process.argv[2] || 'http://127.0.0.1:8090'
const username = process.argv[3] || 'admin'
const password = process.argv[4] || process.env.E2E_ADMIN_PASSWORD || ''
if (!password) {
  console.error('缺少超管密码：请用第 4 个参数或环境变量 E2E_ADMIN_PASSWORD 提供')
  process.exit(2)
}
const CDP_PORT = 9444
const BROWSERS = [
  process.env.E2E_BROWSER,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
  '/usr/bin/chromium-browser'
].filter(Boolean)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

/** 待验证的页面：hash 路由 + 期望出现的关键文案 */
const PAGES = [
  { hash: '#/host/overview', expect: ['CPU 使用率', '内存使用率', '磁盘使用率', '设备信息'] },
  { hash: '#/host/hardware', expect: ['硬件概览', '每核 CPU 使用率', '内存与磁盘'] },
  { hash: '#/host/monitor', expect: ['指标趋势', 'CPU 使用率'] },
  { hash: '#/host/traffic', expect: ['流量采样记录', '实时入向'] },
  { hash: '#/network/iface', expect: ['网络接口'] },
  { hash: '#/network/route', expect: ['静态路由', '下发全部'] },
  { hash: '#/network/dns', expect: ['DNS 服务器', '当前解析配置'] },
  { hash: '#/network/iface-config', expect: ['网卡地址配置', 'netplan 片段预览'] },
  { hash: '#/network/bridge', expect: ['网桥与端口聚合', '新增网桥'] },
  { hash: '#/network/policy-route', expect: ['策略路由', '内核当前规则'] },
  { hash: '#/network/dhcp', expect: ['DHCP 服务', '静态绑定'] },
  { hash: '#/network/dns-server', expect: ['DNS 服务', '本地解析'] },
  { hash: '#/network/routing', expect: ['动态路由', 'BGP', 'OSPF'] },
  { hash: '#/device/setting', expect: ['设备本地设置', '系统时间', 'NTP 同步'] },
  { hash: '#/device/alert', expect: ['告警规则', '告警记录', '阈值规则'] },
  { hash: '#/device/tools', expect: ['工具参数', '执行结果', 'ping'] },
  { hash: '#/device/netstatus', expect: ['网卡', '路由与 DNS'] },
  { hash: '#/device/about', expect: ['平台版本', '数据根目录', '硬件摘要'] },
  { hash: '#/device/snmp', expect: ['SNMP 服务', '协议版本'] },
  { hash: '#/security/firewall', expect: ['防火墙状态', '生效规则'] },
  { hash: '#/security/policy', expect: ['安全策略', '新增策略'] },
  { hash: '#/security/nat', expect: ['NAT 策略', '新增规则'] },
  { hash: '#/security/qos', expect: ['带宽限速', '新增规则'] },
  { hash: '#/plugin/market', expect: ['插件市场', '已安装', '在线市场'] },
  { hash: '#/plugin/source', expect: ['插件源', '索引地址'] },
  { hash: '#/log/access', expect: ['访问日志'] },
  { hash: '#/log/system', expect: ['系统日志'] },
  { hash: '#/log/traffic', expect: ['流量日志'] }
]

class CDP {
  constructor(ws) {
    this.ws = ws
    this.id = 0
    this.pending = new Map()
    this.consoleErrors = []
    this.httpErrors = []
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
    if (msg.method === 'Network.responseReceived') {
      const r = msg.params.response
      if (r.status >= 400 && r.url.includes('/api/')) this.httpErrors.push(`${r.status} ${r.url}`)
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

function launchBrowser(profileDir) {
  for (const bin of BROWSERS) {
    try {
      const child = spawn(
        bin,
        [
          `--remote-debugging-port=${CDP_PORT}`,
          `--user-data-dir=${profileDir}`,
          '--headless=new',
          '--no-first-run',
          '--no-default-browser-check',
          '--disable-gpu',
          '--window-size=1560,980',
          'about:blank'
        ],
        { stdio: 'ignore', detached: false }
      )
      child.on('error', () => {})
      return child
    } catch {
      /* 试下一个 */
    }
  }
  throw new Error('未找到可用浏览器（可用 E2E_BROWSER 指定路径）')
}

async function main() {
  console.log(`==> 设备平台页面冒烟 ${baseUrl}`)
  const profileDir = mkdtempSync(join(tmpdir(), 'dp-e2e-'))
  const browser = launchBrowser(profileDir)
  let cdp
  try {
    cdp = await connect()
    await cdp.send('Runtime.enable')
    await cdp.send('Page.enable')
    await cdp.send('Network.enable')

    // 1) 登录
    await cdp.send('Page.navigate', { url: `${baseUrl}/#/auth/login` })
    await sleep(4000)

    // 拖拽滑块（若开启）
    const rects = await cdp.eval(`(() => {
      const box = document.querySelector('.drag_verify'), handler = document.querySelector('.dv_handler')
      if (!box || !handler) return null
      const b = box.getBoundingClientRect(), h = handler.getBoundingClientRect()
      return { bx: b.x, by: b.y, bw: b.width, hx: h.x, hy: h.y, hw: h.width, hh: h.height }
    })()`)
    if (rects) {
      const startX = rects.hx + rects.hw / 2
      const startY = rects.hy + rects.hh / 2
      const endX = rects.bx + rects.bw - 4
      await cdp.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: startX, y: startY, button: 'left', clickCount: 1 })
      for (let i = 1; i <= 12; i++) {
        await cdp.send('Input.dispatchMouseEvent', {
          type: 'mouseMoved',
          x: startX + ((endX - startX) * i) / 12,
          y: startY,
          button: 'left',
          buttons: 1
        })
        await sleep(25)
      }
      await cdp.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: endX, y: startY, button: 'left', clickCount: 1 })
      await sleep(500)
    }

    const submitted = await cdp.eval(`(() => {
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
      if (c) setVal(c, '')
      const btn = [...document.querySelectorAll('button')].find((b) => (b.innerText || '').replace(/\\s+/g, '').includes('登录'))
      if (!btn) return 'missing-button'
      btn.click()
      return 'submitted'
    })()`)
    check('登录表单提交', submitted === 'submitted', submitted)

    let hash = ''
    for (let i = 0; i < 30; i++) {
      await sleep(500)
      hash = (await cdp.eval('location.hash')) || ''
      if (hash.includes('/dashboard')) break
    }
    check('登录后进入工作台', hash.includes('/dashboard'), `hash=${hash}`)

    // 2) 逐个打开新页面
    for (const page of PAGES) {
      cdp.consoleErrors.length = 0
      cdp.httpErrors.length = 0
      await cdp.eval(`location.hash = ${JSON.stringify(page.hash)}`)
      await sleep(1800)
      const text = (await cdp.eval('document.body.innerText')) || ''
      const missing = page.expect.filter((k) => !text.includes(k))
      const apiErrs = cdp.httpErrors.filter((u) => !u.includes('/auth/'))
      check(
        `页面渲染 ${page.hash}`,
        missing.length === 0 && apiErrs.length === 0,
        [missing.length ? `缺少: ${missing.join(',')}` : '', apiErrs.length ? `接口异常: ${apiErrs[0]}` : '']
          .filter(Boolean)
          .join(' | ')
      )
    }

    // 3) 页面无控制台错误
    check('无浏览器控制台错误', cdp.consoleErrors.length === 0, cdp.consoleErrors[0] || '')
  } finally {
    try {
      browser.kill()
    } catch {
      /* ignore */
    }
    await sleep(300)
    try {
      rmSync(profileDir, { recursive: true, force: true })
    } catch {
      /* ignore */
    }
  }

  const passed = checks.filter((c) => c.ok).length
  console.log(`\n结果: ${passed}/${checks.length} 通过`)
  process.exit(passed === checks.length ? 0 : 1)
}

main().catch((e) => {
  console.error('冒烟失败:', e.message)
  process.exit(1)
})

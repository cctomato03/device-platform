/**
 * API 冒烟测试（零依赖）：接口连通性 + RBAC 负向校验 + 多租户/数据范围基础校验
 *
 * 用法：
 *   node scripts/e2e-api.mjs [baseUrl] [adminUser] [adminPassword]
 *
 * 覆盖内容：
 *   1. 健康检查：/healthz、/readyz
 *   2. 登录链路：验证码（Redis 取值）→ 登录 → userinfo → routes
 *   3. 全部只读接口逐个断言 code=0（列表/详情/字典/监控/概览）
 *   4. RBAC 负向：创建「普通用户」角色的账号 → 登录 → 访问 /system/users 必须 403/30001
 *   5. 越权防护：该账号访问 /auth/routes 只应拿到极少菜单（无用户管理）
 *   6. 清理：删除测试账号
 *
 * 环境变量：
 *   E2E_REDIS_CONTAINER  Redis 容器名；缺省时依次尝试 wb-redis / web-basic-redis / web-basic-redis57
 *   E2E_REDIS_PREFIX     Redis key 前缀，默认 web:
 */
const baseUrl = process.argv[2] || 'http://127.0.0.1:8090'
const adminUser = process.argv[3] || 'admin'
const adminPass = process.argv[4] || process.env.E2E_ADMIN_PASSWORD || ''
const REDIS_CONTAINERS = [process.env.E2E_REDIS_CONTAINER, 'wb-redis', 'web-basic-redis', 'web-basic-redis57'].filter(
  Boolean
)
if (!adminPass) {
  console.error('缺少超管密码：请用第 4 个参数或环境变量 E2E_ADMIN_PASSWORD 提供（见后端启动日志）')
  process.exit(2)
}
const REDIS_PREFIX = process.env.E2E_REDIS_PREFIX ?? 'web:'

const results = []
const check = (name, ok, detail = '') => {
  results.push({ name, ok, detail })
  console.log(`${ok ? '✅' : '❌'} ${name}${detail ? `  ${detail}` : ''}`)
}

async function redisGet(key) {
  const { execFileSync } = await import('node:child_process')
  try {
    for (const container of REDIS_CONTAINERS) {
    try {
      const v = execFileSync('docker', ['exec', container, 'redis-cli', 'GET', key], { encoding: 'utf8' }).trim()
      if (v) return v
    } catch {
      /* 换下一个候选容器 */
    }
  }
  throw new Error(`无法从 Redis 读取验证码，已尝试容器：${REDIS_CONTAINERS.join(', ')}`)
  } catch {
    return ''
  }
}

async function api(path, { method = 'GET', token, body } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(`${baseUrl}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  let json = null
  try {
    json = await res.json()
  } catch {
    /* 非 JSON 响应（如 404 页面） */
  }
  return { status: res.status, json }
}

async function login(username, password) {
  const cap = await api('/api/v1/auth/captcha')
  const captchaId = cap.json?.data?.captchaId || ''
  const captchaCode = captchaId ? await redisGet(`${REDIS_PREFIX}captcha:${captchaId}`) : ''
  if (!captchaCode) throw new Error('无法从 Redis 读取验证码，请确认容器与 key 前缀')
  const res = await api('/api/v1/auth/login', {
    method: 'POST',
    body: { username, password, captchaId, captchaCode }
  })
  return res
}

const READONLY_ENDPOINTS = [
  '/api/v1/auth/userinfo',
  '/api/v1/auth/routes',
  '/api/v1/profile',
  '/api/v1/dashboard/summary',
  '/api/v1/system/users?page=1&size=10',
  '/api/v1/system/roles?page=1&size=10',
  '/api/v1/system/roles/all',
  '/api/v1/system/menus/tree',
  '/api/v1/system/depts/tree',
  '/api/v1/system/posts',
  '/api/v1/system/dict/types',
  '/api/v1/system/configs',
  '/api/v1/system/tenants',
  '/api/v1/system/jobs',
  '/api/v1/files?page=1&size=10',
  '/api/v1/monitor/online',
  '/api/v1/monitor/server',
  '/api/v1/monitor/login-logs?page=1&size=10',
  '/api/v1/monitor/oper-logs?page=1&size=10',
  '/api/v1/dicts/sys_user_sex'
]

async function main() {
  console.log(`==> API 冒烟 ${baseUrl}`)

  // 1. 健康检查
  for (const p of ['/healthz', '/readyz']) {
    const r = await api(p)
    check(`健康检查 ${p}`, r.status === 200, `status=${r.status}`)
  }
  const ready = await api('/readyz')
  const readyBody = ready.json?.data || ready.json
  check('依赖就绪（mysql + redis）', readyBody?.mysql === true && readyBody?.redis === true, JSON.stringify(readyBody))

  // 2. 登录
  const admin = await login(adminUser, adminPass)
  const adminToken = admin.json?.data?.accessToken || ''
  check('管理员登录', admin.json?.code === 0 && !!adminToken, `code=${admin.json?.code}`)
  const refreshToken = admin.json?.data?.refreshToken || ''
  check('登录返回 refreshToken', !!refreshToken, refreshToken ? 'ok' : 'missing')

  // refresh 流程
  const refreshed = await api('/api/v1/auth/refresh', { method: 'POST', body: { refreshToken } })
  check('刷新令牌（refresh）', refreshed.json?.code === 0 && !!refreshed.json?.data?.accessToken, `code=${refreshed.json?.code}`)
  const token = refreshed.json?.data?.accessToken || adminToken

  // 3. 只读接口全量 sweep
  const failed = []
  for (const ep of READONLY_ENDPOINTS) {
    const r = await api(ep, { token })
    const ok = r.status === 200 && r.json?.code === 0
    if (!ok) failed.push(`${ep} → ${r.status}/${r.json?.code}`)
  }
  check(`只读接口连通（${READONLY_ENDPOINTS.length} 个）`, failed.length === 0, failed.join(' | ') || 'all ok')

  // 3.5 各模块 CRUD 闭环（新增 → 列表可见 → 修改 → 删除 → 列表消失）
  const crud = async (name, opts) => {
    const errs = []
    const create = await api(opts.create.path, { method: 'POST', token, body: opts.create.body })
    const id = create.json?.data?.id || create.json?.data
    if (create.json?.code !== 0 || !id) return check(`CRUD ${name}`, false, `新增失败 code=${create.json?.code} msg=${create.json?.message ?? ''}`)

    const updated = await api(opts.update(id), { method: 'PUT', token, body: opts.updateBody })
    if (updated.json?.code !== 0) errs.push(`修改 code=${updated.json?.code}`)

    if (opts.after) {
      const extra = await opts.after(id)
      if (extra !== true) errs.push(String(extra))
    }

    const del = await api(opts.del(id), { method: 'DELETE', token })
    if (del.json?.code !== 0) errs.push(`删除 code=${del.json?.code}`)

    // 删除后的存在性校验：按 id 查详情应 404（列表接口可能按租户/数据范围过滤，不能作为唯一证据）
    if (opts.verify) {
      const after = await api(opts.verify(id), { token })
      if (after.json?.code !== 10004) errs.push(`删除后详情仍可查到 code=${after.json?.code}`)
    }

    const list = await api(opts.list, { token })
    const raw = list.json?.data
    const text = JSON.stringify(Array.isArray(raw) ? raw : (raw?.list ?? {}))
    if (text.includes(String(id)) && id !== undefined) errs.push('删除后列表中仍存在')
    check(`CRUD 闭环 ${name}`, errs.length === 0, errs.join(' | ') || 'ok')
  }

  const stamp = Math.random().toString(36).slice(2, 8)

  // 岗位
  await crud('岗位', {
    create: { path: '/api/v1/system/posts', body: { code: `e2epost_${stamp}`, name: `E2E岗位${stamp}`, sort: 99, status: 1 } },
    update: (id) => `/api/v1/system/posts/${id}`,
    updateBody: { code: `e2epost_${stamp}`, name: `E2E岗位改${stamp}`, sort: 98, status: 1 },
    del: (id) => `/api/v1/system/posts/${id}`,
    list: '/api/v1/system/posts'
  })

  // 部门
  await crud('部门', {
    create: { path: '/api/v1/system/depts', body: { parentId: '0', name: `E2E部门${stamp}`, code: `e2edept_${stamp}`, sort: 99, status: 1 } },
    update: (id) => `/api/v1/system/depts/${id}`,
    updateBody: { parentId: '0', name: `E2E部门改${stamp}`, code: `e2edept_${stamp}`, sort: 98, status: 1 },
    del: (id) => `/api/v1/system/depts/${id}`,
    verify: (id) => `/api/v1/system/depts/detail/${id}`,
    list: '/api/v1/system/depts/tree'
  })

  // 参数配置
  await crud('参数配置', {
    create: { path: '/api/v1/system/configs', body: { name: `E2E参数${stamp}`, key: `e2e.config.${stamp}`, value: 'v1', valueType: 'string', isPublic: false } },
    update: (id) => `/api/v1/system/configs/${id}`,
    updateBody: { name: `E2E参数改${stamp}`, key: `e2e.config.${stamp}`, value: 'v2', valueType: 'string', isPublic: false },
    del: (id) => `/api/v1/system/configs/${id}`,
    list: '/api/v1/system/configs'
  })

  // 租户
  await crud('租户', {
    create: { path: '/api/v1/system/tenants', body: { name: `E2E租户${stamp}`, code: `e2etn_${stamp}`, contactName: 'E2E', maxUsers: 10, status: 1 } },
    update: (id) => `/api/v1/system/tenants/${id}`,
    updateBody: { name: `E2E租户改${stamp}`, code: `e2etn_${stamp}`, contactName: 'E2E', maxUsers: 20, status: 1 },
    del: (id) => `/api/v1/system/tenants/${id}`,
    list: '/api/v1/system/tenants'
  })

  // 字典类型 + 字典项
  {
    const dt = await api('/api/v1/system/dict/types', {
      method: 'POST',
      token,
      body: { name: `E2E字典${stamp}`, code: `e2e_dict_${stamp}`, status: 1 }
    })
    const dtId = dt.json?.data?.id || dt.json?.data
    const errs = []
    if (dt.json?.code !== 0) errs.push(`字典类型新增 code=${dt.json?.code}`)
    if (dtId) {
      const dd = await api('/api/v1/system/dict/data', {
        method: 'POST',
        token,
        body: { dictCode: `e2e_dict_${stamp}`, label: '选项A', value: 'a', sort: 1, status: 1 }
      })
      const ddId = dd.json?.data?.id || dd.json?.data
      if (dd.json?.code !== 0) errs.push(`字典项新增 code=${dd.json?.code}`)
      const read = await api(`/api/v1/dicts/e2e_dict_${stamp}`, { token })
      if (read.json?.code !== 0) errs.push(`字典读取失败 code=${read.json?.code}`)
      if (ddId) {
        const upd = await api(`/api/v1/system/dict/data/${ddId}`, {
          method: 'PUT',
          token,
          body: { dictCode: `e2e_dict_${stamp}`, label: '选项A改', value: 'a', sort: 2, status: 1 }
        })
        if (upd.json?.code !== 0) errs.push(`字典项修改 code=${upd.json?.code}`)
        const delD = await api(`/api/v1/system/dict/data/${ddId}`, { method: 'DELETE', token })
        if (delD.json?.code !== 0) errs.push(`字典项删除 code=${delD.json?.code}`)
      }
      const updT = await api(`/api/v1/system/dict/types/${dtId}`, {
        method: 'PUT',
        token,
        body: { name: `E2E字典改${stamp}`, code: `e2e_dict_${stamp}`, status: 1 }
      })
      if (updT.json?.code !== 0) errs.push(`字典类型修改 code=${updT.json?.code}`)
      const delT = await api(`/api/v1/system/dict/types/${dtId}`, { method: 'DELETE', token })
      if (delT.json?.code !== 0) errs.push(`字典类型删除 code=${delT.json?.code}`)
    }
    check('CRUD 闭环 字典类型 + 字典项', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // 角色（新增 → 授权菜单 → 读取已授权 → 修改 → 删除）
  {
    const errs = []
    const role = await api('/api/v1/system/roles', {
      method: 'POST',
      token,
      body: { name: `E2E角色${stamp}`, code: `e2e_role_${stamp}`, sort: 99, dataScope: 1, status: 1 }
    })
    const roleId = role.json?.data?.id || role.json?.data
    if (role.json?.code !== 0) errs.push(`角色新增 code=${role.json?.code}`)
    if (roleId) {
      const menus = await api('/api/v1/system/menus/tree', { token })
      const firstMenu = JSON.stringify(menus.json?.data || []).match(/"id":"(\d+)"/)
      const grant = await api(`/api/v1/system/roles/${roleId}/grant`, {
        method: 'PUT',
        token,
        body: { menuIds: firstMenu ? [firstMenu[1]] : [], deptIds: [] }
      })
      if (grant.json?.code !== 0) errs.push(`角色授权 code=${grant.json?.code}`)
      const granted = await api(`/api/v1/system/roles/${roleId}/granted`, { token })
      if (granted.json?.code !== 0) errs.push(`读取授权 code=${granted.json?.code}`)
      const upd = await api(`/api/v1/system/roles/${roleId}`, {
        method: 'PUT',
        token,
        body: { name: `E2E角色改${stamp}`, code: `e2e_role_${stamp}`, sort: 98, dataScope: 2, status: 1 }
      })
      if (upd.json?.code !== 0) errs.push(`角色修改 code=${upd.json?.code}`)
      const del = await api(`/api/v1/system/roles/${roleId}`, { method: 'DELETE', token })
      if (del.json?.code !== 0) errs.push(`角色删除 code=${del.json?.code}`)
    }
    check('CRUD 闭环 角色 + 菜单授权', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // 定时任务（新增 → 立即执行 → 启用/停用 → 日志 → 删除）
  {
    const errs = []
    const job = await api('/api/v1/system/jobs', {
      method: 'POST',
      token,
      body: {
        name: `E2E任务${stamp}`,
        handler: 'cleanLoginLog',
        jobType: 1,
        cron: '0 0 3 * * *',
        params: '{"days":3650}',
        concurrent: 2,
        timeoutSec: 60
      }
    })
    const jobId = job.json?.data?.id || job.json?.data
    if (job.json?.code !== 0) errs.push(`任务新增 code=${job.json?.code} msg=${job.json?.message ?? ''}`)
    if (jobId) {
      const run = await api(`/api/v1/system/jobs/${jobId}/run`, { method: 'POST', token })
      if (run.json?.code !== 0) errs.push(`任务执行 code=${run.json?.code} msg=${run.json?.message ?? ''}`)
      const st = await api(`/api/v1/system/jobs/${jobId}/status`, { method: 'PUT', token, body: { status: 2 } })
      if (st.json?.code !== 0) errs.push(`任务停用(仅允许1/2) code=${st.json?.code}`)
      const logs = await api('/api/v1/system/jobs/logs?page=1&pageSize=10', { token })
      if (logs.json?.code !== 0) errs.push(`任务日志 code=${logs.json?.code}`)
      const del = await api(`/api/v1/system/jobs/${jobId}`, { method: 'DELETE', token })
      if (del.json?.code !== 0) errs.push(`任务删除 code=${del.json?.code}`)
    }
    check('CRUD 闭环 定时任务（含立即执行/日志）', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // 文件上传 → 访问 URL → 删除
  {
    const errs = []
    const fd = new FormData()
    fd.append('file', new Blob(['web-basic api smoke'], { type: 'text/plain' }), `e2e-api-${stamp}.txt`)
    const up = await fetch(`${baseUrl}/api/v1/files/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: fd
    }).then((r) => r.json())
    const fileId = up?.data?.id || ''
    if (up?.code !== 0 || !fileId) errs.push(`上传 code=${up?.code}`)
    if (fileId) {
      const url = up?.data?.url || ''
      const head = await fetch(url.startsWith('http') ? url : `${baseUrl}${url}`).then((r) => r.status)
      if (head !== 200) errs.push(`文件访问 status=${head}`)
      const del = await api(`/api/v1/files/${fileId}`, { method: 'DELETE', token })
      if (del.json?.code !== 0) errs.push(`文件删除 code=${del.json?.code}`)
    }
    check('文件上传 / 下载 / 删除', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // ID 精度与"不存在记录"语义
  {
    const post = await api('/api/v1/system/posts', {
      method: 'POST',
      token,
      body: { code: `e2e_precision_${stamp}`, name: `E2E精度${stamp}`, sort: 99, status: 1 }
    })
    const newId = post.json?.data?.id
    const list = await api('/api/v1/system/posts', { token })
    const rawData = list.json?.data
    const postArr = Array.isArray(rawData) ? rawData : (rawData?.list ?? [])
    const found = postArr.some((p) => p.id === newId)
    check('新增返回的雪花 ID 为字符串且与列表一致（无精度丢失）', typeof newId === 'string' && found, `id=${newId} type=${typeof newId} inList=${found}`)

    if (newId) {
      const del = await api(`/api/v1/system/posts/${newId}`, { method: 'DELETE', token })
      if (del.json?.code !== 0) check('清理精度测试岗位', false, `code=${del.json?.code}`)
    }

    const ghost = await api('/api/v1/system/posts/999999999999999', {
      method: 'PUT',
      token,
      body: { code: 'ghost', name: 'ghost', sort: 1, status: 1 }
    })
    check('修改不存在的记录返回 404（不再是静默成功）', ghost.status === 404 || ghost.json?.code === 10004, `status=${ghost.status} code=${ghost.json?.code}`)
    const ghostDel = await api('/api/v1/system/posts/999999999999999', { method: 'DELETE', token })
    check('删除不存在的记录返回 404', ghostDel.status === 404 || ghostDel.json?.code === 10004, `status=${ghostDel.status} code=${ghostDel.json?.code}`)
  }

  // 参数校验与重复键：必须返回业务错误码而不是 500
  {
    const badPost = await api('/api/v1/system/posts', { method: 'POST', token, body: { code: '', name: '' } })
    check('参数校验（缺必填字段返回 400 而非 500）', badPost.status === 400 || badPost.json?.code === 10002, `status=${badPost.status} code=${badPost.json?.code}`)
    // 先建一条，再重复建同名编码 → 期望唯一键冲突被归一化为 10005/409
    const dup1 = await api('/api/v1/system/posts', {
      method: 'POST',
      token,
      body: { code: 'e2e_dup_key', name: 'E2E重复', sort: 1, status: 1 }
    })
    if (dup1.json?.code !== 0) {
      // 上次运行残留（软删后唯一索引仍占位）时先清空再建
      const l = await api('/api/v1/system/posts', { token })
      const rawL = l.json?.data
      const arr = Array.isArray(rawL) ? rawL : (rawL?.list ?? [])
      for (const p of arr.filter((x) => x.code === 'e2e_dup_key')) {
        await api(`/api/v1/system/posts/${p.id}`, { method: 'DELETE', token })
      }
    }
    const dup1b = await api('/api/v1/system/posts', {
      method: 'POST',
      token,
      body: { code: 'e2e_dup_key', name: 'E2E重复', sort: 1, status: 1 }
    })
    const dup2 = await api('/api/v1/system/posts', {
      method: 'POST',
      token,
      body: { code: 'e2e_dup_key', name: 'E2E重复', sort: 1, status: 1 }
    })
    check('唯一键冲突返回业务错误（非 500）', dup2.status !== 500 && dup2.json?.code !== 0, `status=${dup2.status} code=${dup2.json?.code}`)
    for (const d of [dup1, dup1b]) {
      const id = d.json?.data?.id
      if (id) await api(`/api/v1/system/posts/${id}`, { method: 'DELETE', token })
    }
  }
  // 3.6 菜单 CRUD（目录 → 菜单 → 修改 → 删除）
  {
    const errs = []
    const dir = await api('/api/v1/system/menus', {
      method: 'POST',
      token,
      body: { parentId: '0', name: `E2E目录${stamp}`, type: 1, path: `/e2e-${stamp}`, icon: 'ep:grid', sort: 99, visible: true }
    })
    const dirId = dir.json?.data?.id
    if (dir.json?.code !== 0 || !dirId) errs.push(`目录新增 code=${dir.json?.code}`)
    if (dirId) {
      const menu = await api('/api/v1/system/menus', {
        method: 'POST',
        token,
        body: {
          parentId: dirId,
          name: `E2E菜单${stamp}`,
          type: 2,
          path: 'list',
          component: '/system/user/index',
          icon: 'ep:user',
          sort: 1,
          visible: true
        }
      })
      const menuId = menu.json?.data?.id
      if (menu.json?.code !== 0 || !menuId) errs.push(`菜单新增 code=${menu.json?.code}`)
      if (menuId) {
        const upd = await api(`/api/v1/system/menus/${menuId}`, {
          method: 'PUT',
          token,
          body: { parentId: dirId, name: `E2E菜单改${stamp}`, type: 2, path: 'list', component: '/system/user/index', sort: 2, visible: true }
        })
        if (upd.json?.code !== 0) errs.push(`菜单修改 code=${upd.json?.code}`)
        const delM = await api(`/api/v1/system/menus/${menuId}`, { method: 'DELETE', token })
        if (delM.json?.code !== 0) errs.push(`菜单删除 code=${delM.json?.code}`)
      }
      const delD = await api(`/api/v1/system/menus/${dirId}`, { method: 'DELETE', token })
      if (delD.json?.code !== 0) errs.push(`目录删除 code=${delD.json?.code}`)
    }
    check('CRUD 闭环 菜单（目录 + 子菜单）', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // 3.7 个人中心与账号治理（改资料 / 改密码 / 重置密码 / 启停 / 分配角色）
  {
    const errs = []
    const uname = `e2e_prof_${stamp}`
    const created = await api('/api/v1/system/users', {
      method: 'POST',
      token,
      body: { username: uname, nickname: 'E2E资料', password: 'E2e@123456', deptId: '1', status: 1, roleIds: ['2'], sex: 0 }
    })
    const uid = created.json?.data?.id
    if (created.json?.code !== 0 || !uid) errs.push(`账号新增 code=${created.json?.code}`)

    if (uid) {
      const login1 = await login(uname, 'E2e@123456')
      const tk = login1.json?.data?.accessToken
      if (!tk) errs.push(`账号登录 code=${login1.json?.code}`)
      if (tk) {
        const before = await api('/api/v1/profile', { token: tk })
        const upd = await api('/api/v1/profile', {
          method: 'PUT',
          token: tk,
          body: { nickname: 'E2E资料改', mobile: '13800000000', email: 'e2e@example.com', sex: 1 }
        })
        const after = await api('/api/v1/profile', { token: tk })
        if (upd.json?.code !== 0) errs.push(`改资料 code=${upd.json?.code}`)
        else if (after.json?.data?.nickname !== 'E2E资料改') errs.push(`改资料未生效 nickname=${after.json?.data?.nickname}`)
        if (before.json?.code !== 0) errs.push(`读资料 code=${before.json?.code}`)

        const wrongPwd = await api('/api/v1/profile/password', {
          method: 'PUT',
          token: tk,
          body: { oldPassword: 'wrong-password', newPassword: 'NewPwd@12345' }
        })
        if (wrongPwd.json?.code === 0) errs.push('旧密码错误却修改成功')

        const chg = await api('/api/v1/profile/password', {
          method: 'PUT',
          token: tk,
          body: { oldPassword: 'E2e@123456', newPassword: 'NewPwd@12345' }
        })
        if (chg.json?.code !== 0) errs.push(`改密码 code=${chg.json?.code}`)
        const login2 = await login(uname, 'NewPwd@12345')
        if (login2.json?.code !== 0) errs.push(`新密码登录失败 code=${login2.json?.code}`)
      }

      const st = await api(`/api/v1/system/users/${uid}/status`, { method: 'PUT', token, body: { status: 0 } })
      if (st.json?.code !== 0) errs.push(`停用账号 code=${st.json?.code}`)
      const disabledLogin = await login(uname, 'NewPwd@12345')
      if (disabledLogin.json?.code === 0) errs.push('停用后仍可登录')
      await api(`/api/v1/system/users/${uid}/status`, { method: 'PUT', token, body: { status: 1 } })

      const reset = await api(`/api/v1/system/users/${uid}/reset-password`, { method: 'PUT', token })
      const newPwd = reset.json?.data?.password
      if (reset.json?.code !== 0 || !newPwd) errs.push(`重置密码 code=${reset.json?.code}`)

      const assign = await api(`/api/v1/system/users/${uid}/roles`, { method: 'PUT', token, body: { roleIds: ['2'] } })
      if (assign.json?.code !== 0) errs.push(`分配角色 code=${assign.json?.code}`)

      const del = await api('/api/v1/system/users', { method: 'DELETE', token, body: { ids: [String(uid)] } })
      if (del.json?.code !== 0) errs.push(`删除测试账号 code=${del.json?.code}`)
    }
    check('个人中心与账号治理（改资料/改密/重置/启停/分配角色）', errs.length === 0, errs.join(' | ') || 'ok')
  }

  // 3.8 平台能力：指标 / 文档 / 通知公告 / 站内信 / Excel / 代码生成 / 审计差异
  const raw = async (path, opts = {}) => {
    const headers = { ...(opts.headers || {}) }
    if (opts.token) headers.Authorization = `Bearer ${opts.token}`
    if (opts.body !== undefined && !(opts.body instanceof FormData)) headers['Content-Type'] = 'application/json'
    const res = await fetch(`${baseUrl}${path}`, {
      method: opts.method || 'GET',
      headers,
      body: opts.body === undefined || opts.body instanceof FormData ? opts.body : JSON.stringify(opts.body)
    })
    const buffer = Buffer.from(await res.arrayBuffer())
    return { status: res.status, headers: res.headers, buffer }
  }

  // Prometheus 指标
  {
    const m = await raw('/metrics')
    const text = m.buffer.toString('utf8')
    check(
      'Prometheus 指标 /metrics（请求计数 + 构建信息 + 运行时）',
      m.status === 200 &&
        text.includes('web_basic_http_requests_total') &&
        text.includes('web_basic_build_info') &&
        text.includes('web_basic_goroutines'),
      `status=${m.status} bytes=${m.buffer.length}`
    )
  }

  // OpenAPI 文档
  {
    const specRes = await raw('/swagger/openapi.json')
    let spec = null
    try {
      spec = JSON.parse(specRes.buffer.toString('utf8'))
    } catch {
      /* ignore */
    }
    const paths = spec ? Object.keys(spec.paths || {}).length : 0
    check(
      'OpenAPI 规范 /swagger/openapi.json（3.0.3，覆盖全部业务路径）',
      specRes.status === 200 && spec?.openapi === '3.0.3' && paths >= 60 && !!spec?.components?.securitySchemes?.bearerAuth,
      `paths=${paths}`
    )
    const page = await raw('/swagger/index.html')
    const html = page.buffer.toString('utf8')
    check(
      'Swagger 文档页离线可用（无 CDN 依赖）',
      page.status === 200 && html.includes('openapi.json') && !/https?:\/\/[^"']*(cdn|unpkg|jsdelivr)/i.test(html),
      `status=${page.status} bytes=${page.buffer.length}`
    )
  }

  // 通知公告 + 站内信
  {
    const notice = await api('/api/v1/system/notices', {
      method: 'POST',
      token,
      body: { title: `E2E公告${stamp}`, type: 2, level: 2, content: 'E2E 冒烟测试正文' }
    })
    const noticeId = notice.json?.data?.id || notice.json?.data || ''
    check('通知公告新增', notice.json?.code === 0 && !!noticeId, `code=${notice.json?.code}`)
    if (noticeId) {
      const upd = await api(`/api/v1/system/notices/${noticeId}`, {
        method: 'PUT',
        token,
        body: { title: `E2E公告改${stamp}`, type: 2, level: 3, content: 'E2E 冒烟测试正文（已修改）' }
      })
      check('通知公告修改', upd.json?.code === 0, `code=${upd.json?.code}`)

      // 审计差异：上一步修改应写进操作日志的 diff 列
      // 操作日志由中间件在响应后写入，这里轮询等待，避免与断言竞态
      let logs = null
      let withDiff = null
      for (let i = 0; i < 10; i++) {
        logs = await api('/api/v1/monitor/oper-logs?page=1&pageSize=50', { token })
        withDiff = (logs.json?.data?.list || []).find((l) => (l.diff || '').includes('"field":"title"'))
        if (withDiff) break
        await new Promise((r) => setTimeout(r, 300))
      }
      check(
        '操作日志记录字段级前后差异（diff）',
        logs.json?.code === 0 && !!withDiff,
        withDiff ? withDiff.diff.slice(0, 90) : '未找到含 title 差异的日志'
      )

      const pub = await api(`/api/v1/system/notices/${noticeId}/publish`, { method: 'POST', token })
      const pushed = pub.json?.data?.pushed ?? 0
      check('公告发布并扇出站内信', pub.json?.code === 0 && pushed >= 1, `pushed=${pushed}`)

      const msgs = await api('/api/v1/messages?page=1&pageSize=10', { token })
      const firstMsg = msgs.json?.data?.list?.[0]
      const unreadBefore = (await api('/api/v1/messages/unread-count', { token })).json?.data?.count ?? 0
      check(
        '站内信列表 + 未读计数',
        msgs.json?.code === 0 && (msgs.json?.data?.total ?? 0) >= 1 && unreadBefore >= 1,
        `total=${msgs.json?.data?.total} unread=${unreadBefore}`
      )

      let marked = false
      if (firstMsg?.id) {
        const read = await api(`/api/v1/messages/${firstMsg.id}/read`, { method: 'PUT', token })
        const unreadAfter = (await api('/api/v1/messages/unread-count', { token })).json?.data?.count ?? 0
        const all = await api('/api/v1/messages/read-all', { method: 'PUT', token })
        marked = read.json?.code === 0 && all.json?.code === 0 && unreadAfter < unreadBefore
      }
      check('站内信已读 / 全部已读', marked, marked ? 'ok' : '流转失败')

      const del = await api(`/api/v1/system/notices/${noticeId}`, { method: 'DELETE', token })
      const after = await api(`/api/v1/system/notices/${noticeId}`, { token })
      check('通知公告删除（连带清理站内信）', del.json?.code === 0 && after.json?.code === 10004, `del=${del.json?.code} after=${after.json?.code}`)
    }
  }

  // Excel 导出 / 导入
  {
    const exp = await raw('/api/v1/system/users/export', { token })
    check(
      '用户导出 Excel（xlsx + 下载文件名）',
      exp.status === 200 && exp.buffer.slice(0, 2).toString() === 'PK' && (exp.headers.get('content-disposition') || '').includes('filename'),
      `bytes=${exp.buffer.length}`
    )

    const operExp = await raw('/api/v1/monitor/oper-logs/export', { token })
    check('操作日志导出 Excel', operExp.status === 200 && operExp.buffer.slice(0, 2).toString() === 'PK', `bytes=${operExp.buffer.length}`)

    const tpl = await raw('/api/v1/system/users/import-template', { token })
    check('用户导入模板下载', tpl.status === 200 && tpl.buffer.slice(0, 2).toString() === 'PK', `bytes=${tpl.buffer.length}`)

    const form = new FormData()
    form.append('file', new Blob([tpl.buffer], { type: 'application/vnd.openxmlformats-officedocument.spreaappeetml.sheet' }), 'tpl.xlsx')
    const imp = await raw('/api/v1/system/users/import', { method: 'POST', token, body: form })
    let impJson = null
    try {
      impJson = JSON.parse(imp.buffer.toString('utf8'))
    } catch {
      /* ignore */
    }
    const imported = impJson?.data?.success ?? 0
    const failedRows = impJson?.data?.failed ?? 0
    check('用户 Excel 导入（模板示例行）', impJson?.code === 0 && imported >= 1, `success=${imported} failed=${failedRows} ${(impJson?.data?.errors || []).join(';')}`)

    // 清理导入出来的示例账号
    if (imported > 0) {
      const list = await api('/api/v1/system/users?page=1&pageSize=20&username=zhangsan', { token })
      const zs = (list.json?.data?.list || []).find((u) => u.username === 'zhangsan')
      if (zs) await api('/api/v1/system/users', { method: 'DELETE', token, body: { ids: [String(zs.id)] } })
    }
  }

  // 站点个性化配置
  {
    const pub = await api('/api/v1/public/site-config')
    check(
      '站点配置公开接口（免登录可读）',
      pub.json?.code === 0 && typeof pub.json?.data?.siteName === 'string' && pub.json?.data?.siteName.length > 0,
      `siteName=${pub.json?.data?.siteName}`
    )

    const before = pub.json?.data || {}
    const badName = await api('/api/v1/system/site-config', {
      method: 'PUT',
      token,
      body: { siteName: '  ', loginDesc: '' }
    })
    check(
      '站点配置校验（网站名称为空 → 10001 且返回可读原因）',
      badName.json?.code === 10001 && String(badName.json?.message || '').includes('网站名称'),
      `code=${badName.json?.code} msg=${badName.json?.message}`
    )

    const badUrl = await api('/api/v1/system/site-config', {
      method: 'PUT',
      token,
      body: { siteName: 'x', siteLogo: 'javascript:alert(1)' }
    })
    check('站点配置校验（非法图片地址被拒）', badUrl.json?.code === 10001, `code=${badUrl.json?.code}`)

    const saved = await api('/api/v1/system/site-config', {
      method: 'PUT',
      token,
      body: {
        siteName: `E2E站点${stamp}`,
        siteLogo: '',
        loginBg: '',
        loginDesc: `E2E 站点描述 ${stamp}`,
        loginSubTitle: `E2E 站点副标题 ${stamp}`,
        registerEnabled: false
      }
    })
    const after = await api('/api/v1/public/site-config')
    check(
      '站点配置保存并即时生效（公开接口同步新值，含登录页副标题）',
      saved.json?.code === 0 &&
        after.json?.data?.siteName === `E2E站点${stamp}` &&
        after.json?.data?.loginSubTitle === `E2E 站点副标题 ${stamp}` &&
        after.json?.data?.registerEnabled === false,
      `siteName=${after.json?.data?.siteName} subTitle=${after.json?.data?.loginSubTitle}`
    )

    // 恢复默认，避免污染后续用例与人工验收
    await api('/api/v1/system/site-config', {
      method: 'PUT',
      token,
      body: {
        siteName: before.siteName || 'Web Basic Admin',
        siteLogo: before.siteLogo || '',
        loginBg: before.loginBg || '',
        loginDesc: before.loginDesc || '',
        loginSubTitle: before.loginSubTitle || '',
        registerEnabled: before.registerEnabled === true
      }
    })

    // 关闭注册时，注册页路由不可达（前端守卫），后端也无需注册接口即可保证安全
    const reg = await api('/api/v1/auth/register', { method: 'POST', body: { username: 'x' } })
    check('注册接口未开放（关闭注册时不可用）', reg.json?.code !== 0 || reg.status === 404, `status=${reg.status} code=${reg.json?.code}`)
  }

  // 文件管理：按日期目录归类 + 日期区间筛选
  {
    const form = new FormData()
    form.append('file', new Blob([`e2e file ${stamp}`], { type: 'text/plain' }), `e2e-${stamp}.txt`)
    form.append('bizType', 'document')
    const up = await fetch(`${baseUrl}/api/v1/files/upload`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: form
    }).then((r) => r.json())
    const key = up?.data?.objectKey || ''
    const today = new Date()
    const ymd = `${today.getFullYear()}/${String(today.getMonth() + 1).padStart(2, '0')}/${String(today.getDate()).padStart(2, '0')}`
    check(
      '文件上传按「年/月/日/业务」目录归类（文件名带日期时间）',
      up?.code === 0 && key.startsWith(`${ymd}/document/`) && /\/\d{8}-\d{6}-/.test(key),
      `key=${key}`
    )

    const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
    const tomorrow = new Date(today.getTime() + 86400000)
    const tomStr = `${tomorrow.getFullYear()}-${String(tomorrow.getMonth() + 1).padStart(2, '0')}-${String(tomorrow.getDate()).padStart(2, '0')}`

    const inRange = await api(`/api/v1/files?page=1&pageSize=50&startDate=${todayStr}&endDate=${todayStr}`, { token })
    const outRange = await api(`/api/v1/files?page=1&pageSize=50&startDate=${tomStr}`, { token })
    check(
      '文件列表支持上传日期区间筛选（今天命中 / 明天不命中）',
      inRange.json?.code === 0 &&
        (inRange.json?.data?.list || []).some((f) => f.objectKey === key) &&
        (outRange.json?.data?.list || []).every((f) => f.objectKey !== key),
      `命中=${(inRange.json?.data?.list || []).length} 越界=${(outRange.json?.data?.list || []).length}`
    )

    if (up?.data?.id) await api(`/api/v1/files/${up.data.id}`, { method: 'DELETE', token })
  }
  // 定时任务：类型元信息 + Shell 任务安全闸（默认关闭）
  {
    const meta = await api('/api/v1/system/jobs/meta', { token })
    const types = meta.json?.data?.types || []
    check(
      '定时任务元信息接口（3 种类型 + 内置处理器清单 + 解释器白名单）',
      meta.json?.code === 0 &&
        types.length === 3 &&
        (meta.json?.data?.handlers || []).length >= 4 &&
        (meta.json?.data?.interpreters || []).includes('sh'),
      `types=${types.map((t) => t.value).join('/')} shell=${meta.json?.data?.shellEnabled}`
    )

    // Shell 任务是否启用由部署配置决定：关闭时必须拒绝创建，开启时必须能创建并执行
    const shellEnabled = meta.json?.data?.shellEnabled === true
    const shellJob = await api('/api/v1/system/jobs', {
      method: 'POST',
      token,
      body: {
        name: `e2e-shell-${stamp}`,
        jobType: 3,
        cron: '0 */10 * * * *',
        params: JSON.stringify({ interpreter: 'sh', script: 'echo hi' }),
        timeoutSec: 15,
        status: 2
      }
    })
    if (!shellEnabled) {
      check(
        'Shell 脚本任务受总开关保护（未开 scheduler.shell_enabled 时拒绝创建并说明原因）',
        shellJob.json?.code === 10001 && /Shell/.test(String(shellJob.json?.message || '')),
        `code=${shellJob.json?.code} msg=${shellJob.json?.message}`
      )
    } else {
      const shellId = shellJob.json?.data?.id
      await api(`/api/v1/system/jobs/${shellId}/run`, { method: 'POST', token })
      let shellLog = null
      for (let i = 0; i < 16; i++) {
        await new Promise((r) => setTimeout(r, 500))
        const logs = await api('/api/v1/system/jobs/logs?page=1&pageSize=10', { token })
        shellLog = (logs.json?.data?.list || []).find((l) => l.jobId === shellId)
        if (shellLog) break
      }
      check(
        'Shell 脚本任务可执行（脚本内容存库 → 立即执行 → 输出写日志）',
        shellJob.json?.code === 0 && !!shellLog && /hi/.test(String(shellLog?.result || '')) && shellLog?.status === 1,
        `code=${shellJob.json?.code} result=${String(shellLog?.result || '').slice(0, 40)}`
      )
      if (shellId) await api(`/api/v1/system/jobs/${shellId}`, { method: 'DELETE', token })
    }
  }

  // 登录交互开关：公开参数暴露有效值，关闭验证码后登录接口不再校验
  {
    const pub = await api('/api/v1/system/configs/public')
    const cfgRows = await api('/api/v1/system/configs', { token })
    const list = cfgRows.json?.data || []
    const capRow = list.find((r) => r.key === 'sys.login.captcha')
    const sliderRow = list.find((r) => r.key === 'sys.login.slider')
    check(
      '登录开关已进入公开参数（验证码 / 滑块）',
      pub.json?.code === 0 &&
        pub.json.data['sys.login.captcha'] === 'true' &&
        pub.json.data['sys.login.slider'] === 'true' &&
        !!capRow?.id &&
        !!sliderRow?.id,
      `captcha=${pub.json?.data?.['sys.login.captcha']} slider=${pub.json?.data?.['sys.login.slider']}`
    )

    const putCfg = (row, value) =>
      api(`/api/v1/system/configs/${row.id}`, {
        method: 'PUT',
        token,
        body: {
          name: row.name,
          key: row.key,
          value: String(value),
          valueType: row.valueType || 'bool',
          isPublic: true,
          remark: row.remark || ''
        }
      })

    if (capRow?.id) {
      await putCfg(capRow, false)
      const capOff = await api('/api/v1/auth/captcha')
      // 关闭验证码后：接口返回 enabled=false，且登录不再需要 captchaId
      const loginNoCaptcha = await fetch(`${baseUrl}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: adminUser, password: adminPass })
      }).then((r) => r.json())
      check(
        '关闭验证码后：/auth/captcha 返回 enabled=false，且不带验证码可登录',
        capOff.json?.data?.enabled === false && loginNoCaptcha?.code === 0,
        `enabled=${capOff.json?.data?.enabled} login=${loginNoCaptcha?.code}`
      )
      await putCfg(capRow, true)
      const capOn = await api('/api/v1/auth/captcha')
      check('重新开启验证码后立即恢复（开关热更新）', capOn.json?.data?.enabled === true, `enabled=${capOn.json?.data?.enabled}`)
    }
  }

  // 文件直传：本地存储时签名接口明确返回「不支持」，前端据此回退服务端上传
  {
    const ticket = await api('/api/v1/files/oss-ticket', {
      method: 'POST',
      token,
      body: { originName: 'demo.txt', size: 12, bizType: 'document' }
    })
    check(
      'OSS 直传签名接口（本地存储返回不支持，前端自动回退中转上传）',
      ticket.json?.code !== 0 && /不支持/.test(String(ticket.json?.message || '')),
      `code=${ticket.json?.code} msg=${ticket.json?.message}`
    )
  }
  // 工作中心：待办 / 通知 / 消息 / 推送（每个用户都有，接口强制只操作自己的数据）
  {
    const me = await api('/api/v1/auth/userinfo', { token })
    const myId = me.json?.data?.user?.userId
    const summary = await api('/api/v1/my/summary', { token })
    check(
      '工作中心汇总接口（未读消息 / 待办 / 逾期 / 未读通知）',
      summary.json?.code === 0 &&
        typeof summary.json.data.unreadMessages === 'number' &&
        typeof summary.json.data.pendingTodos === 'number' &&
        typeof summary.json.data.unreadNotices === 'number',
      JSON.stringify(summary.json?.data || {})
    )

    const todoCreate = await api('/api/v1/my/todos', {
      method: 'POST',
      token,
      body: { title: `e2e-todo-${stamp}`, content: '接口自测', priority: 3, dueTime: '2030-01-01 09:00:00' }
    })
    const todoId = todoCreate.json?.data?.id
    const todoList = await api('/api/v1/my/todos?page=1&pageSize=20&status=1', { token })
    const created = (todoList.json?.data?.list || []).find((t) => t.id === todoId)
    check(
      '我的待办：新建后可查询（只返回自己的待办）',
      todoCreate.json?.code === 0 && !!created && created.title === `e2e-todo-${stamp}`,
      `id=${todoId}`
    )

    if (todoId) {
      const done = await api(`/api/v1/my/todos/${todoId}/status`, { method: 'PUT', token, body: { status: 2 } })
      const pending = await api('/api/v1/my/todos?page=1&pageSize=20&status=1', { token })
      const stillPending = (pending.json?.data?.list || []).some((t) => t.id === todoId)
      check(
        '待办状态流转（完成后再查待处理列表已消失）',
        done.json?.code === 0 && stillPending === false,
        `code=${done.json?.code}`
      )
      const del = await api(`/api/v1/my/todos/${todoId}`, { method: 'DELETE', token })
      check('删除待办', del.json?.code === 0, `code=${del.json?.code}`)
    }

    // 通知：发布一条公告 → 出现在「我的通知」→ 标记已读
    const notice = await api('/api/v1/system/notices', {
      method: 'POST',
      token,
      body: { title: `e2e-notice-${stamp}`, type: 1, level: 2, content: '接口自测公告' }
    })
    const noticeId = notice.json?.data?.id
    const published = await api(`/api/v1/system/notices/${noticeId}/publish`, { method: 'POST', token })
    const myNotices = await api('/api/v1/my/notices?page=1&pageSize=20', { token })
    const mine = (myNotices.json?.data?.list || []).find((n) => n.id === noticeId)
    check(
      '通知中心：发布的公告出现在「我的通知」且初始未读',
      notice.json?.code === 0 && published.json?.code === 0 && !!mine && mine.isRead === false,
      `id=${noticeId} read=${mine?.isRead}`
    )
    if (noticeId) {
      const read = await api(`/api/v1/my/notices/${noticeId}/read`, { method: 'POST', token })
      const after = await api('/api/v1/my/notices?page=1&pageSize=20', { token })
      const mineAfter = (after.json?.data?.list || []).find((n) => n.id === noticeId)
      check('通知标记已读', read.json?.code === 0 && mineAfter?.isRead === true, `read=${mineAfter?.isRead}`)
      await api(`/api/v1/system/notices/${noticeId}`, { method: 'DELETE', token })
    }

    // 推送：给当前用户推一条站内信，未读数应增加
    if (myId) {
      const before = await api('/api/v1/my/summary', { token })
      const push = await api('/api/v1/system/push/messages', {
        method: 'POST',
        token,
        body: { userIds: [myId], title: `e2e-push-${stamp}`, content: '推送自测', type: 1 }
      })
      const after = await api('/api/v1/my/summary', { token })
      const repeat = await api('/api/v1/system/push/messages', {
        method: 'POST',
        token,
        body: { userIds: [myId], title: `e2e-push-${stamp}`, content: '推送自测', type: 1, bizKey: `push-${stamp}` }
      })
      const repeatAgain = await api('/api/v1/system/push/messages', {
        method: 'POST',
        token,
        body: { userIds: [myId], title: `e2e-push-${stamp}`, content: '推送自测', type: 1, bizKey: `push-${stamp}` }
      })
      check(
        '消息推送（未读数增加 + bizKey 幂等：重复推送不会重复打扰）',
        push.json?.code === 0 &&
          push.json?.data?.created === 1 &&
          (after.json?.data?.unreadMessages || 0) > (before.json?.data?.unreadMessages || 0) &&
          repeat.json?.data?.created === 1 &&
          repeatAgain.json?.data?.created === 0,
        `push=${push.json?.data?.created} repeat=${repeat.json?.data?.created} again=${repeatAgain.json?.data?.created} before=${before.json?.data?.unreadMessages} after=${after.json?.data?.unreadMessages} code=${push.json?.code} msg=${push.json?.message}`
      )
    }

    // 开放接口：未配置 api_key 时应整体不可用（默认关闭，避免误暴露）
    const openMsg = await api('/api/v1/open/messages', {
      method: 'POST',
      body: { userIds: [1], title: 'x' }
    })
    check(
      '开放接口默认关闭（未配置 integration.api_key 时不可用）',
      openMsg.status === 404 || openMsg.json?.code !== 0,
      `status=${openMsg.status} code=${openMsg.json?.code}`
    )
  }
  // 密码规则：公开接口 + 弱密码返回可读原因（而不是笼统的「参数校验失败」）
  {
    const policy = await api('/api/v1/public/password-policy')
    check(
      '密码规则公开接口（前端据此展示规则）',
      policy.json?.code === 0 &&
        policy.json.data.minLength >= 8 &&
        policy.json.data.requireUpper === true &&
        /大写|数字/.test(String(policy.json.data.description || '')),
      policy.json?.data?.description || ''
    )

    const weak = await api('/api/v1/profile/password', {
      method: 'PUT',
      token,
      body: { oldPassword: adminPass, newPassword: '123456' }
    })
    check(
      '弱密码返回可读原因（含长度/组成说明，不是笼统「参数校验失败」）',
      weak.json?.code === 10001 && /密码长度|大写|数字/.test(String(weak.json?.message || '')),
      `code=${weak.json?.code} msg=${weak.json?.message}`
    )

    const weak2 = await api('/api/v1/profile/password', {
      method: 'PUT',
      token,
      body: { oldPassword: adminPass, newPassword: 'abcdefgh' }
    })
    check(
      '弱密码提示具体缺什么（缺大写/数字时逐项说明）',
      weak2.json?.code === 10001 && /还需要包含/.test(String(weak2.json?.message || '')),
      `msg=${weak2.json?.message}`
    )
  }
  // 在线会话与工作台统计一致性（回归防护：曾因逐条查用户表导致大幅放大查询）
  {
    const online = await api('/api/v1/monitor/online', { token })
    const summary = await api('/api/v1/dashboard/summary', { token })
    const listLen = (online.json?.data || []).length
    check(
      '在线会话与工作台 onlineCount 一致（且在线用户资料已批量补齐）',
      online.json?.code === 0 &&
        summary.json?.code === 0 &&
        listLen > 0 &&
        summary.json?.data?.onlineCount === listLen &&
        (online.json.data || []).every((u) => typeof u.userId === 'string'),
      `list=${listLen} onlineCount=${summary.json?.data?.onlineCount}`
    )
  }
  // 代码生成器
  {
    const tables = await api('/api/v1/tool/gen/tables', { token })
    const names = (tables.json?.data || []).map((t) => t.name)
    check(
      '代码生成器：库表清单',
      tables.json?.code === 0 && names.length >= 10 && names.includes('sys_user'),
      `tables=${names.length}`
    )
    const preview = await api('/api/v1/tool/gen/tables/sys_notice/preview', { token })
    const files = preview.json?.data || []
    check(
      '代码生成器：预览生成代码（entity/service/api/router/页面/菜单 SQL）',
      preview.json?.code === 0 && files.length === 6 && files.some((f) => f.path.endsWith('.vue')),
      `files=${files.length}`
    )
    const zip = await raw('/api/v1/tool/gen/download', { method: 'POST', token, body: { tables: ['sys_notice'] } })
    check('代码生成器：打包下载 zip', zip.status === 200 && zip.buffer.slice(0, 2).toString() === 'PK', `bytes=${zip.buffer.length}`)
  }
  // 4. RBAC 负向：受限账号
  const username = `e2e_rbac_${Math.random().toString(36).slice(2, 8)}`
  const password = 'E2e@123456'
  const created = await api('/api/v1/system/users', {
    method: 'POST',
    token,
    body: { username, nickname: 'E2E受限账号', password, deptId: '1', status: 1, roleIds: ['2'], sex: 0 }
  })
  const newUserId = created.json?.data?.id || created.json?.data || ''
  check('创建受限账号（普通用户角色）', created.json?.code === 0, `code=${created.json?.code} id=${newUserId}`)

  let limitedToken = ''
  if (newUserId) {
    const limited = await login(username, password)
    limitedToken = limited.json?.data?.accessToken || ''
    check('受限账号登录', limited.json?.code === 0 && !!limitedToken, `code=${limited.json?.code}`)

    const denied = await api('/api/v1/system/users?page=1&size=10', { token: limitedToken })
    check('RBAC 负向：无权限访问用户管理被拒（403/30001）', denied.status === 403 || denied.json?.code === 30001, `status=${denied.status} code=${denied.json?.code}`)

    const deniedCreate = await api('/api/v1/system/users', {
      method: 'POST',
      token: limitedToken,
      body: { username: `x_${username}`, nickname: 'x', password: 'E2e@123456', deptId: '1', status: 1, roleIds: ['2'], sex: 0 }
    })
    check('RBAC 负向：无权限新增用户被拒', deniedCreate.status === 403 || deniedCreate.json?.code === 30001, `status=${deniedCreate.status} code=${deniedCreate.json?.code}`)

    const routes = await api('/api/v1/auth/routes', { token: limitedToken })
    const flat = JSON.stringify(routes.json?.data || [])
    check('越权防护：受限账号菜单不含用户/角色管理', routes.json?.code === 0 && !flat.includes('system:user:list') && !flat.includes('system:role:list'), `code=${routes.json?.code}`)

    const self = await api('/api/v1/profile', { token: limitedToken })
    check('受限账号可访问自己的资料（/profile）', self.json?.code === 0, `code=${self.json?.code}`)

    // 新模块的负向校验：公告管理无权限
    const deniedNotice = await api('/api/v1/system/notices?page=1&pageSize=10', { token: limitedToken })
    check(
      'RBAC 负向：无权限访问通知公告被拒（403/30001）',
      deniedNotice.status === 403 || deniedNotice.json?.code === 30001,
      `status=${deniedNotice.status} code=${deniedNotice.json?.code}`
    )

    // 前端无按钮权限：按钮权限列表不得包含公告管理相关按钮码
    const info = await api('/api/v1/auth/userinfo', { token: limitedToken })
    const buttons = JSON.stringify(info.json?.data?.buttons || [])
    check(
      '前端无按钮权限：受限账号按钮列表不含 system:notice:*',
      info.json?.code === 0 && !buttons.includes('system:notice:'),
      `buttons=${buttons.slice(0, 120)}`
    )

    // 站内信是"登录即可用"的能力：受限账号也能读自己的消息，但看不到别人的
    const myMsgs = await api('/api/v1/messages?page=1&pageSize=10', { token: limitedToken })
    const othersMsgs = (myMsgs.json?.data?.list || []).every((m) => String(m.userId) !== '1')
    check('站内信仅返回当前登录用户的数据', myMsgs.json?.code === 0 && othersMsgs, `code=${myMsgs.json?.code}`)
  }

  // 5. 清理
  if (newUserId) {
    const del = await api('/api/v1/system/users', { method: 'DELETE', token, body: { ids: [String(newUserId)] } })
    check('清理受限测试账号', del.json?.code === 0, `code=${del.json?.code}`)
  }

  // 6. 登出（注销 token）
  const logout = await api('/api/v1/auth/logout', { method: 'POST', token: adminToken })
  check('登出', logout.json?.code === 0, `code=${logout.json?.code}`)
  const afterLogout = await api('/api/v1/auth/userinfo', { token: adminToken })
  check('登出后旧 token 失效', afterLogout.status === 401 || afterLogout.json?.code === 20001, `status=${afterLogout.status} code=${afterLogout.json?.code}`)

  const failedChecks = results.filter((r) => !r.ok)
  console.log(`\n结果: ${results.length - failedChecks.length}/${results.length} 通过`)
  process.exit(failedChecks.length ? 1 : 0)
}

main().catch((e) => {
  console.error('API 冒烟失败:', e.message)
  process.exit(1)
})

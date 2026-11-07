/**
 * web-basic 后端领域模型类型声明
 *
 * 这里描述 Go 后端（/api/v1/**）的真实出参结构，
 * 与 Web Admin Pro 模板自身的 `Api.*` 命名空间（模板演示数据契约）相互独立：
 *
 * - `WebBasic.*`  → 后端真实结构
 * - `Api.*`       → 模板页面使用的展示结构（由 src/api/* 适配层从 WebBasic 映射而来）
 *
 * @module types/api/web-basic
 */

declare namespace WebBasic {
  /** 通用分页出参 */
  interface Page<T> {
    list: T[]
    total: number
    page: number
    pageSize: number
  }

  /** 通用分页入参 */
  interface PageQuery {
    page?: number
    pageSize?: number
  }

  /** 首页概览 */  interface DashboardSummary {
    users: number
    roles: number
    depts: number
    posts: number
    files: number
    fileSize: number
    tenants: number
    todayLogins: number
    todayOpers: number
    onlineCount: number
    loginTrend: Array<{ date: string; count: number }>
    recentLogins: Array<{ username: string; ip: string; status: number; loginTime: string }>
    recentOpers: Array<{
      title: string
      userName: string
      requestUri: string
      httpMethod: string
      status: number
      costMs: number
      operTime: string
    }>
    generatedAt: string
  }

  /** 用户 */
  interface User {
    id: string
    tenantId?: string
    username: string
    nickname: string
    deptId: string
    deptName?: string
    mobile: string
    email: string
    sex: number
    avatar: string
    status: number
    isSuper?: boolean
    remark?: string
    roleIds?: string[]
    roleNames?: string[]
    postIds?: string[]
    lastLoginIp?: string
    lastLoginTime?: string | null
    createTime?: string
    updateTime?: string | null
  }

  /** 角色 */
  interface Role {
    id: string
    name: string
    code: string
    sort: number
    dataScope: number
    status: number
    remark: string
    createTime?: string
  }

  /** 菜单 */
  interface Menu {
    id: string
    parentId: string
    name: string
    type: number
    permission: string
    path: string
    component: string
    redirect: string
    icon: string
    sort: number
    visible: boolean
    keepAlive: boolean
    affix: boolean
    isFrame: boolean
    isPublic: boolean
    /** 接口权限点（type=4）使用的请求方法与路径 */
    apiMethod?: string
    apiPath?: string
    status: number
    children?: Menu[]
  }

  /** 部门 */
  interface Dept {
    id: string
    parentId: string
    name: string
    code: string
    ancestors: string
    leader: string
    phone: string
    email: string
    sort: number
    status: number
    remark?: string
    children?: Dept[]
  }

  /** 岗位 */
  interface Post {
    id: string
    name: string
    code: string
    sort: number
    status: number
    remark: string
    createTime?: string
  }

  /** 字典类型 */
  interface DictType {
    id: string
    name: string
    code: string
    status: number
    remark: string
  }

  /** 字典项 */
  interface DictData {
    id: string
    dictCode: string
    label: string
    value: string
    sort: number
    tagType: string
    isDefault: number
    status: number
  }

  /** 参数配置 */
  interface Config {
    id: string
    name: string
    key: string
    value: string
    /** 值类型：string / bool / int / json（后端 value_type） */
    valueType?: string
    /** 兼容字段 */
    type?: number
    /** 内置参数不可删除 */
    isBuiltin?: boolean
    isPublic: boolean
    remark: string
  }

  /** 租户 */
  interface Tenant {
    id: string
    name: string
    code: string
    contactName: string
    contactPhone: string
    maxUsers: number
    expireAt: string
    status: number
    remark: string
  }

  /** 文件记录 */
  /** OSS 直传签名 */
  interface OssTicket {
    objectKey: string
    uploadUrl: string
    method: string
    headers: Record<string, string>
    url: string
    expiresIn: number
    storage: string
  }

  interface FileItem {
    id: string
    originName: string
    fileName: string
    /** 完整对象键，如 2026/11/06/site/20261106-140912-123.jpg */
    path: string
    /** 所在目录，如 2026/11/06/site（按上传日期分层） */
    dir: string
    url: string
    size: number
    ext: string
    mimeType: string
    storage: string
    bizType: string
    createTime?: string
  }

  /** 登录日志 */
  interface LoginLog {
    id: string
    username: string
    loginType: number
    status: number
    msg: string
    ip: string
    location: string
    userAgent: string
    loginTime: string
  }

  /** 操作日志 */
  interface OperLog {
    /** 字段级变更差异（JSON 字符串：[{field,before,after}]） */
    diff?: string
    id: string
    traceId: string
    title: string
    method: string
    requestUri: string
    httpMethod: string
    userId: string
    userName: string
    deptName: string
    ip: string
    location: string
    params: string
    result: string
    status: number
    errorMsg: string
    costMs: number
    operTime: string
  }

  /** 在线用户 */
  interface OnlineUser {
    userId: string
    username: string
    nickname: string
    jti: string
    loginTime: number
  }

  /** 服务器信息 */
  interface ServerInfo {
    goVersion: string
    os: string
    arch: string
    numCpu: number
    goroutines: number
    heapAllocMb: number
    sysMb: number
    uptimeSec: number
    mysqlOk: boolean
    redisOk: boolean
  }

  /** 个人资料 */
  interface Profile {
    id: string
    username: string
    nickname: string
    avatar: string
    mobile: string
    email: string
    sex: number
    deptName: string
    roles: string[]
  }

  /** 定时任务 */
  /** 定时任务类型选项 */
  interface JobTypeOption {
    value: number
    label: string
    description: string
    enabled: boolean
  }

  /** 内置任务处理器选项 */
  interface JobHandlerOption {
    value: string
    label: string
    description: string
  }

  /** 定时任务元信息 */
  interface JobMeta {
    types: JobTypeOption[]
    handlers: JobHandlerOption[]
    shellEnabled: boolean
    interpreters: string[]
  }

  interface Job {
    id: string
    name: string
    handler: string
    jobType: number
    target: string
    httpMethod: string
    cron: string
    params: string
    concurrent: number
    timeoutSec: number
    retryTimes: number
    status: number
    nextRunAt: string
    remark: string
    createTime: string
  }

  /** 任务执行日志 */
  interface JobLog {
    id: string
    jobId: string
    jobName: string
    triggerType: number
    status: number
    startTime: string
    endTime: string
    costMs: number
    result: string
    createTime: string
  }

  /** 通知公告 */
  interface Notice {
    id: string
    title: string
    /** 1 通知 2 公告 3 维护 */
    type: number
    /** 1 普通 2 重要 3 紧急 */
    level: number
    content: string
    /** 0 草稿 1 已发布 2 已关闭 */
    status: number
    publisherId: string
    publisher: string
    publishTime: string | null
    tenantId: string
    createBy: string
    createTime: string
    updateTime: string | null
    remark: string
  }

  /** 站内信 */
  interface Message {
    id: string
    userId: string
    noticeId: string
    title: string
    content: string
    /** 1 系统 2 公告 3 任务 */
    type: number
    isRead: number
    readTime: string | null
    createTime: string
    remark: string
  }

  /** 代码生成器：库表元信息 */
  interface GenTable {
    name: string
    comment: string
    rowCount: number
    createAt: string
    module: string
    entity: string
    var: string
    perm: string
    routePath: string
    viewPath: string
    columns?: GenColumn[]
  }
  /** 代码生成器：字段元信息 */
  interface GenColumn {
    name: string
    dataType: string
    columnType: string
    comment: string
    nullable: boolean
    isPrimary: boolean
    autoIncr: boolean
    maxLen: number
  }

  /** 代码生成器：生成的文件 */
  interface GenFile {
    path: string
    content: string
    lang: string
  }

  /** 站点个性化配置 */
  interface SiteConfig {
    siteName: string
    siteLogo: string
    loginBg: string
    loginDesc: string
    loginSubTitle: string
    registerEnabled: boolean
    updateTime?: string
  }

/** 待办 */
interface Todo {
  id: string
  userId: string
  title: string
  content: string
  /** 优先级：1 低 2 普通 3 高 4 紧急 */
  priority: number
  /** 状态：1 待处理 2 已完成 3 已取消 */
  status: number
  dueTime?: string | null
  completeTime?: string | null
  linkUrl?: string
  bizType?: string
  bizId?: string
  /** 来源：1 手工 2 系统推送 3 开放接口 */
  source?: number
  createTime?: string
}

/** 待办统计 */
interface TodoSummary {
  pending: number
  overdue: number
  completed: number
}

/** 工作中心汇总 */
interface WorkbenchSummary {
  unreadMessages: number
  pendingTodos: number
  overdueTodos: number
  unreadNotices: number
}

/** 我的通知（公告 + 已读状态） */
interface MyNotice {
  id: string
  title: string
  type: number
  level: number
  content: string
  publisher?: string
  publishTime?: string | null
  createTime?: string
  isRead: boolean
}
}

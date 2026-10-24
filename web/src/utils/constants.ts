export type NTagType = 'default' | 'error' | 'primary' | 'info' | 'success' | 'warning'

export interface JobTypeOption {
  value: string
  label: string
  description: string
  dangerous?: boolean
}

export const jobTypeOptions: JobTypeOption[] = [
  { value: 'commit', label: '上传', description: '将代码模板上传到小程序并生成体验版' },
  { value: 'submit_audit', label: '提审', description: '按审核配置提交代码审核' },
  { value: 'release', label: '发布', description: '发布已通过审核的版本', dangerous: true },
  { value: 'undo_audit', label: '撤回', description: '撤回正在审核的版本', dangerous: true },
  { value: 'rollback', label: '回退', description: '回退到上一版本或指定历史版本', dangerous: true },
  { value: 'gray_release', label: '灰度', description: '按比例灰度发布新版本', dangerous: true },
  { value: 'cancel_gray_release', label: '取消灰度', description: '取消当前分阶段发布', dangerous: true },
  { value: 'modify_domain', label: '域名', description: '添加、覆盖或删除小程序服务器域名', dangerous: true },
  { value: 'bind_tester', label: '绑定', description: '为多个小程序绑定同一个微信号' },
  { value: 'unbind_tester', label: '解绑', description: '按 userstr 解绑体验者', dangerous: true },
  { value: 'set_visit_status', label: '状态', description: '开启或暂停小程序服务', dangerous: true },
  { value: 'set_support_version', label: '基础库', description: '设置小程序最低基础库版本' },
  { value: 'preflight', label: '预检', description: '批量检查审核状态、隐私接口、类目和域名' },
]

export const jobTypeLabels: Record<string, string> = Object.fromEntries(jobTypeOptions.map((item) => [item.value, item.label]))

export const jobStatusMeta: Record<string, { label: string; type: NTagType }> = {
  queued: { label: '排队中', type: 'default' },
  running: { label: '执行中', type: 'info' },
  success: { label: '成功', type: 'primary' },
  partial: { label: '部分成功', type: 'warning' },
  failed: { label: '失败', type: 'error' },
  canceled: { label: '已取消', type: 'default' },
}

export const itemStatusMeta: Record<string, { label: string; type: NTagType }> = {
  pending: { label: '等待中', type: 'default' },
  running: { label: '执行中', type: 'info' },
  success: { label: '成功', type: 'primary' },
  failed: { label: '失败', type: 'error' },
  skipped: { label: '已跳过', type: 'warning' },
  canceled: { label: '已取消', type: 'default' },
}

export const auditStatusMeta: Record<number, { label: string; type: NTagType }> = {
  0: { label: '审核通过', type: 'primary' },
  1: { label: '审核驳回', type: 'error' },
  2: { label: '审核中', type: 'info' },
  3: { label: '已撤回', type: 'default' },
  4: { label: '审核延后', type: 'warning' },
}

export const accountStatusMeta: Record<number, { label: string; type: NTagType }> = {
  1: { label: '正常', type: 'primary' },
  14: { label: '已注销', type: 'default' },
  16: { label: '已封禁', type: 'error' },
  18: { label: '已告警', type: 'warning' },
  19: { label: '已冻结', type: 'error' },
}

export const authorizerStatusMeta: Record<string, { label: string; type: NTagType }> = {
  active: { label: '已授权', type: 'primary' },
  unauthorized: { label: '已取消授权', type: 'default' },
}

export const permissionNames: Record<number, string> = {
  17: '获取小程序码',
  18: '小程序开发与数据分析',
  25: '开放平台账号管理',
  30: '小程序基本信息管理',
  31: '小程序认证名称检测',
}

export function permissionLabel(id: number): string {
  return permissionNames[id] || `权限集 ${id}`
}

export const operationActionLabels: Record<string, string> = {
  login_success: '登录成功',
  login_failed: '登录失败',
  logout: '退出登录',
  authorizer_sync: '同步授权账号',
  authorizer_exchange: '授权回调',
  authorizer_refresh: '刷新小程序信息',
  preauth_create: '生成授权链接',
  job_create: '创建任务',
  job_finished: '任务结束',
  job_retry: '重试任务',
  job_cancel: '取消任务',
  job_auto_release: '自动发布',
  template_sync: '同步代码模板',
  template_add: '草稿加入模板库',
  template_delete: '删除代码模板',
  settings_reveal: '显示敏感配置',
  component_token_refresh: '刷新 component token',
  component_token_refresh_failed: '刷新 component token 失败',
  ticket_push: '启动票据推送',
  ticket_push_failed: '启动票据推送失败',
  tester_bind: '绑定体验者',
  tester_unbind: '解绑体验者',
  qrcode_zip: '批量生成二维码',
  audit_media_upload: '上传提审素材',
}

export const domainActions = [
  { label: '添加（保留已有）', value: 'add', hint: '在现有域名基础上追加' },
  { label: '覆盖（危险）', value: 'set', hint: '用本次列表覆盖现有域名' },
  { label: '删除', value: 'delete', hint: '从现有域名中删除本次列表' },
]

export const domainFields = [
  { key: 'requestdomain', label: 'request 合法域名', placeholder: 'https://api.example.com' },
  { key: 'wsrequestdomain', label: 'socket 合法域名', placeholder: 'wss://ws.example.com' },
  { key: 'uploaddomain', label: 'uploadFile 合法域名', placeholder: 'https://upload.example.com' },
  { key: 'downloaddomain', label: 'downloadFile 合法域名', placeholder: 'https://download.example.com' },
  { key: 'udpdomain', label: 'UDP 合法域名', placeholder: 'udp.example.com:9000' },
  { key: 'tcpdomain', label: 'TCP 合法域名', placeholder: 'tcp.example.com:9000' },
] as const

export const grayPlanStatusLabels: Record<number, string> = {
  0: '初始',
  1: '执行中',
  2: '暂停中',
  3: '执行完毕',
  4: '已删除',
}

export function jobTypeLabel(type: string): string {
  return jobTypeLabels[type] || type
}

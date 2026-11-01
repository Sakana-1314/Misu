export type NTagType = 'default' | 'error' | 'primary' | 'info' | 'success' | 'warning'

export interface JobTypeOption {
  value: string
  label: string
  description: string
  group: 'release' | 'manage' | 'check'
  dangerous?: boolean
  // Hidden from batch menus; still used to label existing jobs.
  hidden?: boolean
}

export const jobTypeOptions: JobTypeOption[] = [
  { value: 'commit', label: '上传代码', description: '把模板库中的代码上传到小程序，生成新的体验版', group: 'release' },
  { value: 'submit_audit', label: '提交审核', description: '把体验版提交给微信审核，通过后才能发布', group: 'release' },
  { value: 'release', label: '发布上线', description: '把已通过审核的版本发布给所有用户', group: 'release', dangerous: true },
  { value: 'gray_release', label: '分阶段发布', description: '先让一部分用户用上新版本，确认无误后再全量', group: 'release', dangerous: true },
  { value: 'cancel_gray_release', label: '停止分阶段发布', description: '停止正在进行的分阶段发布', group: 'release', dangerous: true },
  { value: 'undo_audit', label: '撤回审核', description: '撤回正在审核中的版本，每天有次数限制', group: 'release', dangerous: true },
  { value: 'rollback', label: '版本回退', description: '把线上版本退回到之前发布过的版本', group: 'release', dangerous: true },
  { value: 'modify_domain', label: '服务器域名', description: '添加、替换或删除小程序可访问的服务器域名', group: 'manage', dangerous: true },
  { value: 'bind_tester', label: '添加体验成员', description: '让指定微信号可以打开体验版', group: 'manage' },
  { value: 'unbind_tester', label: '移除体验成员', description: '移除体验成员', group: 'manage', dangerous: true, hidden: true },
  { value: 'set_visit_status', label: '暂停 / 恢复服务', description: '暂停后用户将无法打开小程序', group: 'manage', dangerous: true },
  { value: 'set_support_version', label: '最低基础库版本', description: '低于该版本的微信将提示用户升级', group: 'manage' },
  { value: 'preflight', label: '提审前检查', description: '检查类目、域名、隐私接口和审核状态，提前发现问题', group: 'check' },
]

export const jobGroupLabels: Record<JobTypeOption['group'], string> = {
  release: '版本发布',
  manage: '小程序设置',
  check: '检查',
}

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
  1: { label: '未通过', type: 'error' },
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
  active: { label: '已接入', type: 'primary' },
  unauthorized: { label: '已取消授权', type: 'default' },
}

export const permissionNames: Record<number, string> = {
  17: '获取小程序码',
  18: '代码管理与数据分析',
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
  mpweb_login_start: '发起公众平台扫码登录',
  mpweb_login_success: '公众平台扫码登录成功',
  mpweb_logout: '断开公众平台连接',
}

export const domainActions = [
  { label: '追加', value: 'add', hint: '保留现有域名，把下面填写的加进去' },
  { label: '替换', value: 'set', hint: '用下面填写的内容整体替换，未填写的类型会被清空' },
  { label: '删除', value: 'delete', hint: '从现有域名中删掉下面填写的这些' },
]

export const domainFields = [
  { key: 'requestdomain', label: '接口请求', api: 'request', placeholder: 'https://api.example.com' },
  { key: 'wsrequestdomain', label: '实时通信', api: 'socket', placeholder: 'wss://ws.example.com' },
  { key: 'uploaddomain', label: '文件上传', api: 'uploadFile', placeholder: 'https://upload.example.com' },
  { key: 'downloaddomain', label: '文件下载', api: 'downloadFile', placeholder: 'https://cdn.example.com' },
  { key: 'udpdomain', label: 'UDP 通信', api: 'UDP', placeholder: 'udp.example.com:9000' },
  { key: 'tcpdomain', label: 'TCP 通信', api: 'TCP', placeholder: 'tcp.example.com:9000' },
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

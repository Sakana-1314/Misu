export interface Paged<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface AdminInfo {
  username: string
  expires_at?: string | null
}

export interface FuncInfo {
  funcscope_category: { id: number }
}

export interface Authorizer {
  id: number
  appid: string
  nick_name: string
  head_img: string
  principal_name: string
  user_name: string
  alias: string
  qrcode_url: string
  signature: string
  service_type_id: number
  service_type_name: string
  verify_type_id: number
  verify_type_name: string
  account_status: number
  register_type: number
  miniprogram_info: any
  func_info: FuncInfo[] | null
  access_token_expires_at: string | null
  auth_time: string | null
  status: 'active' | 'unauthorized'
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

export interface AuditRecord {
  id: number
  authorizer_appid: string
  audit_id: number
  status: number
  reason: string
  screenshot: string
  user_version: string
  user_desc: string
  submit_audit_time: string | null
  submitted_at: string | null
  last_checked_at: string | null
  profile_id: number | null
  auto_release: boolean
  auto_release_job_id: number | null
  created_at: string
  updated_at: string
}

export interface ReleaseRecord {
  id: number
  authorizer_appid: string
  version: string
  description: string
  action: string
  job_id: number | null
  released_at: string
  created_at: string
}

export interface CodeTemplate {
  id: number
  template_id: number
  draft_id: number
  user_version: string
  user_desc: string
  create_time: number
  source_miniprogram_appid: string
  source_miniprogram: string
  template_type: number
  category_list: AuditItem[] | null
  audit_scene: string
  audit_status: number
  reason: string
  synced_at: string
  created_at: string
  updated_at: string
}

export interface CodeDraft {
  draft_id: number
  user_version: string
  user_desc: string
  create_time: number
  source_miniprogram_appid: string
  source_miniprogram: string
  developer: string
}

export interface AuditItem {
  address?: string
  tag?: string
  first_class: string
  second_class: string
  third_class?: string
  first_id: number
  second_id: number
  third_id?: number
  title?: string
}

export interface AuditProfile {
  id: number
  name: string
  item_list: AuditItem[] | null
  version_desc: string
  feedback_info: string
  feedback_stuff: string
  ugc_declare: any
  preview_info: any
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface Job {
  id: number
  type: string
  title: string
  status: string
  payload: any
  total: number
  success_count: number
  failed_count: number
  skipped_count: number
  created_by: string
  error: string
  cancel_requested: boolean
  source_audit_id: number | null
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}

export interface JobItem {
  id: number
  job_id: number
  authorizer_appid: string
  status: string
  attempts: number
  request: any
  response: any
  error_code: string
  error_message: string
  audit_id: number
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}

export interface TokenStats {
  configured: boolean
  component_appid: string
  ticket_received_at: string | null
  ticket_created_at: string | null
  ticket_age_seconds: number
  component_token_expires_at: string | null
  component_token_valid: boolean
  last_push_ok_at: string | null
  last_push_error: string
  last_token_error: string
}

export interface DashboardStats {
  authorizers: number
  auditing: number
  released_7d: number
  failed_jobs_7d: number
  pending_jobs: number
}

export interface AuditSummary extends AuditRecord {
  nick_name: string
  head_img: string
}

export interface CallbackEvent {
  id: number
  info_type: string
  authorizer_appid: string
  appid: string
  event_time: string | null
  payload: any
  created_at: string
}

export interface DashboardData {
  stats: DashboardStats
  health: TokenStats
  platform_published: boolean
  wechat_configured: boolean
  recent_jobs: Job[]
  recent_audits: AuditSummary[]
  recent_events: CallbackEvent[]
}

export interface ConfigView {
  component_appid: string
  app_secret_set: boolean
  verify_token: string
  aes_key: string
  aes_key_previous_set: boolean
  admin_username: string
  cookie_secure: boolean
}

export interface SettingsData {
  public_base_url: string
  auth_redirect_url: string
  callback_url: string
  event_url: string
  egress_ip: string
  platform_published: boolean
  wechat_configured: boolean
  health: TokenStats
  version: string
  commit: string
  build_time: string
  uptime_seconds: number
  db_ok: boolean
  job_concurrency: number
  wx_max_qps: number
  log_retention_days: number
  config: ConfigView
  warnings: string[]
}

export interface VersionInfo {
  exp_info?: { exp_time: number; exp_version: string; exp_desc: string }
  release_info?: { release_time: number; release_version: string; release_desc: string }
  errcode?: number
  errmsg?: string
}

export interface LatestAudit {
  auditid: number
  status: number
  reason: string
  screenshot: string
  user_version: string
  user_desc: string
  submit_audit_time: number
}

export interface DomainResponse {
  requestdomain: string[] | null
  wsrequestdomain: string[] | null
  uploaddomain: string[] | null
  downloaddomain: string[] | null
  udpdomain: string[] | null
  tcpdomain: string[] | null
  invalid_requestdomain?: string[] | null
  invalid_wsrequestdomain?: string[] | null
  invalid_uploaddomain?: string[] | null
  invalid_downloaddomain?: string[] | null
  invalid_udpdomain?: string[] | null
  invalid_tcpdomain?: string[] | null
  no_icp_domain?: string[] | null
}

export interface SettingCategory {
  first: number
  first_name: string
  second: number
  second_name: string
  audit_status: number
  audit_reason: string
}

export interface SettingCategoriesResponse {
  categories: SettingCategory[]
  limit: number
  quota: number
  category_limit: number
}

export interface HistoryVersion {
  app_version: string
  user_version: string
  user_desc: string
  commit_time: number
}

export interface GrayPlan {
  status: number
  create_timestamp: number
  gray_percentage: number
  support_debuger_first: boolean
  support_experiencer_first: boolean
}

export interface OperationLog {
  id: number
  actor: string
  action: string
  target: string
  detail: any
  ip: string
  created_at: string
}

export interface APICallLog {
  id: number
  endpoint: string
  appid: string
  method: string
  errcode: number
  errmsg: string
  latency_ms: number
  created_at: string
}

export interface SyncResult {
  total: number
  added: number
  updated: number
  unauthorized: number
  info_failed: number
  errors: string[] | null
}

export interface PreauthResult {
  auth_url: string
  redirect_uri: string
  expires_in: number
  test_mode: boolean
}

export interface HealthData {
  status: string
  db_ok: boolean
  version: string
  uptime_seconds: number
  server_time: string
}

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import {
  AlertCircleOutline,
  ArrowForwardOutline,
  CheckmarkCircle,
  ChevronDownOutline,
  CloudUploadOutline,
  DownloadOutline,
  GlobeOutline,
  HourglassOutline,
  PeopleOutline,
  QrCodeOutline,
  RefreshOutline,
  RocketOutline,
  TimeOutline,
} from '@vicons/ionicons5'
import { api } from '@/api/client'
import type {
  AuditRecord,
  Authorizer,
  DomainResponse,
  GrayPlan,
  HistoryVersion,
  LatestAudit,
  Paged,
  ReleaseRecord,
  SettingCategoriesResponse,
  VersionInfo,
} from '@/types/api'
import { errorMessage } from '@/utils/error'
import { downloadBlob } from '@/utils/download'
import { formatTime, fromUnix, relativeTimeText } from '@/utils/format'
import { domainFields, grayPlanStatusLabels, permissionLabel } from '@/utils/constants'
import { actionMenu, useJobCreated } from '@/utils/actions'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'
import CopyButton from '@/components/CopyButton.vue'
import EmptyState from '@/components/EmptyState.vue'
import BatchActionModal from '@/components/BatchActionModal.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const notifyJob = useJobCreated()

const appid = String(route.params.appid || '')
const authorizer = ref<Authorizer | null>(null)
const versionInfo = ref<VersionInfo | null>(null)
const latestAudit = ref<LatestAudit | null>(null)
const visitStatus = ref<number | null>(null)
const grayPlan = ref<GrayPlan | null>(null)
const categories = ref<SettingCategoriesResponse | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const appError = ref('')
const tab = ref('overview')

const actionShow = ref(false)
const actionType = ref<string | null>(null)
const rollbackPreset = ref('')

const audits = ref<AuditRecord[]>([])
const auditsTotal = ref(0)
const auditsPage = ref(1)
const auditsLoading = ref(false)
const auditsLoaded = ref(false)

const releases = ref<ReleaseRecord[]>([])
const releasesTotal = ref(0)
const releasesPage = ref(1)
const releasesLoading = ref(false)
const releasesLoaded = ref(false)

const domains = ref<DomainResponse | null>(null)
const domainsLoading = ref(false)
const domainsLoaded = ref(false)

const testers = ref<string[]>([])
const testersLoading = ref(false)
const testersLoaded = ref(false)
const testerWechatid = ref('')
const bindingTester = ref(false)

const historyVersions = ref<HistoryVersion[]>([])
const historyLoading = ref(false)
const historyLoaded = ref(false)

const screenshotShow = ref(false)
const screenshotLoading = ref(false)
const screenshotError = ref('')
const screenshotUrls = ref<string[]>([])

const qrType = ref<'trial' | 'unlimited'>('trial')
const qrPath = ref('')
const qrScene = ref('')
const qrWidth = ref(430)
const qrLoading = ref(false)
const qrUrl = ref('')
const qrBlob = ref<Blob | null>(null)

const actionApps = computed(() => (authorizer.value ? [authorizer.value] : []))
const permissionIds = computed(() => (authorizer.value?.func_info || []).map((item) => item.funcscope_category?.id).filter(Boolean))
const hasCodePermission = computed(() => permissionIds.value.includes(18))
const active = computed(() => authorizer.value?.status === 'active')

const releaseActionLabels: Record<string, string> = {
  release: '发布上线',
  rollback: '版本回退',
  gray: '分阶段发布',
  cancel_gray: '停止分阶段',
}

const auditsPageCount = computed(() => Math.max(1, Math.ceil(auditsTotal.value / 20)))
const releasesPageCount = computed(() => Math.max(1, Math.ceil(releasesTotal.value / 20)))

function settled<T>(result: PromiseSettledResult<T>): T | null {
  return result.status === 'fulfilled' ? result.value : null
}

async function loadAll() {
  loading.value = true
  appError.value = ''
  try {
    const results = await Promise.allSettled([
      api.get<Authorizer>(`/apps/${appid}`),
      api.get<VersionInfo>(`/apps/${appid}/version`, { silent: true }),
      api.get<LatestAudit>(`/apps/${appid}/latest-audit`, { silent: true }),
      api.get<{ status: number }>(`/apps/${appid}/visit-status`, { silent: true }),
      api.get<{ gray_release_plan: GrayPlan }>(`/apps/${appid}/gray-plan`, { silent: true }),
      api.get<SettingCategoriesResponse>(`/apps/${appid}/categories`, { silent: true }),
    ])
    const appResult = results[0]
    if (appResult.status === 'rejected') {
      appError.value = errorMessage(appResult.reason)
      return
    }
    authorizer.value = appResult.value
    versionInfo.value = settled(results[1])
    const audit = settled(results[2])
    latestAudit.value = audit && audit.auditid ? audit : null
    const visit = settled(results[3])
    visitStatus.value = visit ? visit.status : null
    const gray = settled(results[4])
    grayPlan.value = gray?.gray_release_plan || null
    categories.value = settled(results[5])
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadAll()
  void loadDomains()
})

onBeforeUnmount(() => {
  if (qrUrl.value) URL.revokeObjectURL(qrUrl.value)
  clearScreenshotUrls()
})

watch(tab, (value) => {
  if (value === 'audits' && !auditsLoaded.value) void loadAudits(1)
  if (value === 'releases' && !releasesLoaded.value) void loadReleases(1)
  if (value === 'domains' && !domainsLoaded.value) void loadDomains()
  if (value === 'testers' && !testersLoaded.value) void loadTesters()
})

watch(screenshotShow, (show) => {
  if (!show) clearScreenshotUrls()
})

async function refreshInfo() {
  refreshing.value = true
  try {
    await api.post(`/apps/${appid}/refresh`)
    message.success('已更新')
    await loadAll()
  } finally {
    refreshing.value = false
  }
}

async function refreshLatestAudit() {
  refreshing.value = true
  try {
    await api.get(`/apps/${appid}/latest-audit`)
    message.success('审核状态已刷新')
    await loadAll()
    if (auditsLoaded.value) await loadAudits(auditsPage.value)
  } finally {
    refreshing.value = false
  }
}

async function loadAudits(page: number) {
  auditsPage.value = page
  auditsLoading.value = true
  try {
    const data = await api.get<Paged<AuditRecord>>(`/apps/${appid}/audits`, { params: { page, page_size: 20 } })
    audits.value = data.items || []
    auditsTotal.value = data.total
    auditsLoaded.value = true
  } finally {
    auditsLoading.value = false
  }
}

async function refreshAudit(row: AuditRecord) {
  if (!row.audit_id) return
  await api.post(`/apps/${appid}/audits/${row.audit_id}/refresh`)
  message.success('审核状态已更新')
  await loadAudits(auditsPage.value)
  await loadAll()
}

async function loadReleases(page: number) {
  releasesPage.value = page
  releasesLoading.value = true
  try {
    const data = await api.get<Paged<ReleaseRecord>>(`/apps/${appid}/releases`, { params: { page, page_size: 20 } })
    releases.value = data.items || []
    releasesTotal.value = data.total
    releasesLoaded.value = true
  } finally {
    releasesLoading.value = false
  }
}

async function loadDomains() {
  domainsLoading.value = true
  try {
    domains.value = await api.get<DomainResponse>(`/apps/${appid}/domains`, { silent: true })
    domainsLoaded.value = true
  } catch {
    domains.value = null
  } finally {
    domainsLoading.value = false
  }
}

async function loadTesters() {
  testersLoading.value = true
  try {
    const data = await api.get<{ items: string[] }>(`/apps/${appid}/testers`)
    testers.value = data.items || []
    testersLoaded.value = true
  } finally {
    testersLoading.value = false
  }
}

async function bindTester() {
  if (!testerWechatid.value.trim()) {
    message.warning('请填写微信号')
    return
  }
  bindingTester.value = true
  try {
    await api.post(`/apps/${appid}/testers`, { wechatid: testerWechatid.value.trim() })
    message.success('体验成员已添加')
    testerWechatid.value = ''
    await loadTesters()
  } finally {
    bindingTester.value = false
  }
}

function unbindTester(userstr: string, index: number) {
  dialog.warning({
    title: '移除体验成员',
    content: `移除后「成员 ${index + 1}」将无法再打开体验版。`,
    positiveText: '移除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.delete(`/apps/${appid}/testers`, { data: { userstr } })
      message.success('已移除')
      await loadTesters()
    },
  })
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const data = await api.get<{ version_list: HistoryVersion[] }>(`/apps/${appid}/history-versions`)
    historyVersions.value = data.version_list || []
    historyLoaded.value = true
  } finally {
    historyLoading.value = false
  }
}

function openAction(type: string, preset = '') {
  if (!active.value) {
    message.warning('该小程序已取消授权，请重新接入后再操作')
    return
  }
  rollbackPreset.value = preset
  actionType.value = type
  actionShow.value = true
}

function onJobCreated(job: Parameters<typeof notifyJob>[0]) {
  notifyJob(job)
  window.setTimeout(() => void loadAll(), 2000)
}

// ---- release pipeline ----

interface StepAction {
  label: string
  action?: string
  run?: () => void
}

interface NextStep {
  tone: 'primary' | 'info' | 'success' | 'error' | 'default'
  title: string
  desc: string
  primary?: StepAction
  secondary?: StepAction
}

const expVersion = computed(() => versionInfo.value?.exp_info?.exp_version || '')
const releaseVersion = computed(() => versionInfo.value?.release_info?.release_version || '')

const nextStep = computed<NextStep>(() => {
  const audit = latestAudit.value
  if (!active.value) {
    return { tone: 'default', title: '小程序已取消授权', desc: '需要管理员重新授权后才能继续管理。' }
  }
  if (!hasCodePermission.value) {
    return { tone: 'error', title: '缺少代码管理权限', desc: '授权时没有勾选「开发管理与数据分析」，无法上传和提审。请让管理员重新授权。' }
  }
  if (audit?.status === 2 || audit?.status === 4) {
    return {
      tone: 'info',
      title: `${audit.user_version || '新版本'} 正在审核中`,
      desc: `提交于 ${relativeTimeText(audit.submit_audit_time * 1000)}。微信审核通常需要 1–3 天，结果出来后会自动更新。`,
      primary: { label: '刷新审核状态', run: () => void refreshLatestAudit() },
      secondary: { label: '撤回审核', action: 'undo_audit' },
    }
  }
  if (audit?.status === 0 && audit.user_version && audit.user_version !== releaseVersion.value) {
    return {
      tone: 'success',
      title: `${audit.user_version} 已通过审核`,
      desc: '发布后所有用户即可使用新版本。也可以先分阶段发布给一部分用户。',
      primary: { label: '发布上线', action: 'release' },
      secondary: { label: '分阶段发布', action: 'gray_release' },
    }
  }
  if (audit?.status === 1) {
    return {
      tone: 'error',
      title: `${audit.user_version || '上次提交的版本'} 审核未通过`,
      desc: audit.reason || '请根据驳回原因修改后重新提交。',
      primary: { label: '重新提交审核', action: 'submit_audit' },
      secondary: audit.screenshot ? { label: '查看驳回截图', run: () => void openScreenshot(audit.screenshot) } : { label: '上传新版本', action: 'commit' },
    }
  }
  if (expVersion.value && expVersion.value !== releaseVersion.value && audit?.user_version !== expVersion.value) {
    return {
      tone: 'primary',
      title: `体验版 ${expVersion.value} 可以提交审核了`,
      desc: '确认体验版运行正常后提交给微信审核，审核通过即可发布。',
      primary: { label: '提交审核', action: 'submit_audit' },
      secondary: { label: '上传新版本', action: 'commit' },
    }
  }
  return {
    tone: 'default',
    title: releaseVersion.value ? `线上版本 ${releaseVersion.value} 运行中` : '还没有发布过版本',
    desc: '从代码模板上传新代码，开始下一次发布。',
    primary: { label: '上传代码', action: 'commit' },
  }
})

function runStep(step?: StepAction) {
  if (!step) return
  if (step.run) step.run()
  else if (step.action) openAction(step.action)
}

const auditStage = computed(() => {
  const audit = latestAudit.value
  if (!audit) return { label: '未提交', tone: 'idle' }
  switch (audit.status) {
    case 0:
      return { label: '已通过', tone: 'done' }
    case 1:
      return { label: '未通过', tone: 'error' }
    case 2:
      return { label: '审核中', tone: 'active' }
    case 3:
      return { label: '已撤回', tone: 'idle' }
    case 4:
      return { label: '审核延后', tone: 'active' }
    default:
      return { label: '—', tone: 'idle' }
  }
})

const moreOptions = actionMenu()

// ---- tables ----

const auditColumns = computed<DataTableColumns<AuditRecord>>(() => [
  { title: '版本', key: 'user_version', width: 110, render: (row) => h('b', row.user_version || '—') },
  { title: '结果', key: 'status', width: 100, render: (row) => h(StatusTag, { kind: 'audit', value: row.status }) },
  {
    title: '说明',
    key: 'reason',
    minWidth: 240,
    render: (row) => h('span', { class: row.status === 1 ? 'reject-reason' : 'cell-muted' }, row.reason || row.user_desc || '—'),
  },
  { title: '提交时间', key: 'submitted_at', width: 170, render: (row) => h('span', { class: 'cell-muted' }, formatTime(row.submitted_at)) },
  {
    title: '',
    key: 'actions',
    width: 150,
    render: (row) =>
      h('div', { class: 'inline-actions' }, [
        h(NButton, { size: 'small', quaternary: true, onClick: () => refreshAudit(row) }, { default: () => '刷新' }),
        ...(row.screenshot
          ? [h(NButton, { size: 'small', quaternary: true, type: 'primary', onClick: () => openScreenshot(row.screenshot) }, { default: () => '截图' })]
          : []),
      ]),
  },
])

const releaseColumns = computed<DataTableColumns<ReleaseRecord>>(() => [
  {
    title: '操作',
    key: 'action',
    width: 120,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => releaseActionLabels[row.action] || row.action }),
  },
  { title: '版本', key: 'version', width: 120, render: (row) => h('b', row.version || '—') },
  { title: '说明', key: 'description', minWidth: 220, render: (row) => h('span', { class: 'cell-muted' }, row.description || '—') },
  { title: '时间', key: 'released_at', width: 180, render: (row) => h('span', { class: 'cell-muted' }, formatTime(row.released_at)) },
])

function clearScreenshotUrls() {
  for (const url of screenshotUrls.value) URL.revokeObjectURL(url)
  screenshotUrls.value = []
}

async function openScreenshot(screenshot: string) {
  screenshotShow.value = true
  screenshotLoading.value = true
  screenshotError.value = ''
  clearScreenshotUrls()
  const ids = (screenshot || '').split('|').map((item) => item.trim()).filter(Boolean)
  try {
    for (const id of ids) {
      const blob = await api.download(`/apps/${appid}/media`, { params: { media_id: id } })
      screenshotUrls.value.push(URL.createObjectURL(blob))
    }
  } catch (error) {
    screenshotError.value = errorMessage(error)
  } finally {
    screenshotLoading.value = false
  }
}

async function generateQR() {
  if (qrType.value === 'unlimited' && !qrScene.value.trim()) {
    message.warning('小程序码需要填写参数')
    return
  }
  qrLoading.value = true
  try {
    const blob = await api.download(`/apps/${appid}/qrcode`, {
      params: { type: qrType.value, path: qrPath.value, scene: qrScene.value, width: qrWidth.value },
    })
    if (qrUrl.value) URL.revokeObjectURL(qrUrl.value)
    qrBlob.value = blob
    qrUrl.value = URL.createObjectURL(blob)
  } catch (error) {
    message.error(errorMessage(error))
  } finally {
    qrLoading.value = false
  }
}

function downloadQR() {
  if (qrBlob.value) downloadBlob(qrBlob.value, `${authorizer.value?.nick_name || appid}-${qrType.value === 'trial' ? '体验版' : '小程序码'}.jpg`)
}

const domainGroups = computed(() => {
  const value = domains.value as Record<string, string[] | null | undefined> | null
  if (!value) return []
  return domainFields.map((field) => ({ label: field.label, api: field.api, values: value[field.key] || [] }))
})

const configuredDomainCount = computed(() => domainGroups.value.reduce((sum, group) => sum + group.values.length, 0))
</script>

<template>
  <div>
    <div v-if="loading && !authorizer" class="surface-card loading-card">
      <n-spin size="large" />
    </div>

    <div v-else-if="appError" class="surface-card">
      <EmptyState :icon="AlertCircleOutline" title="无法加载小程序" :description="appError">
        <n-button type="primary" @click="router.push('/authorizers')">返回列表</n-button>
      </EmptyState>
    </div>

    <template v-else-if="authorizer">
      <!-- Hero -->
      <div class="hero">
        <div class="hero-left">
          <AppAvatar :src="authorizer.head_img" :name="authorizer.nick_name" :appid="authorizer.appid" :size="60" />
          <div class="hero-info">
            <div class="hero-name">{{ authorizer.nick_name || '未命名小程序' }}</div>
            <div class="hero-meta">
              <span class="mono">{{ authorizer.appid }}</span>
              <CopyButton :text="authorizer.appid" label="复制" />
              <span class="dot">·</span>
              <span>{{ authorizer.principal_name || '主体未知' }}</span>
            </div>
            <div class="hero-tags">
              <StatusTag kind="authorizer" :value="authorizer.status" />
              <StatusTag kind="account" :value="authorizer.account_status" />
              <n-tag size="small" :bordered="false">{{ authorizer.verify_type_id === 0 ? '已认证' : '未认证' }}</n-tag>
              <n-tag v-if="visitStatus === 0" size="small" :bordered="false" type="error">已暂停服务</n-tag>
            </div>
          </div>
        </div>
        <div class="hero-actions">
          <n-tooltip>
            <template #trigger>
              <n-button circle class="icon-button" :loading="refreshing" @click="refreshInfo">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
              </n-button>
            </template>
            从微信重新拉取小程序信息
          </n-tooltip>
          <n-dropdown :options="moreOptions" trigger="click" placement="bottom-end" @select="(key: string | number) => openAction(String(key))">
            <n-button :disabled="!active" icon-placement="right">
              更多操作
              <template #icon><n-icon><ChevronDownOutline /></n-icon></template>
            </n-button>
          </n-dropdown>
        </div>
      </div>

      <!-- Release pipeline -->
      <div class="surface-card pipeline-card">
        <div class="pipeline">
          <div class="stage" :class="expVersion ? 'done' : 'idle'">
            <div class="stage-icon"><n-icon><CloudUploadOutline /></n-icon></div>
            <div class="stage-body">
              <div class="stage-label">体验版</div>
              <div class="stage-version">{{ expVersion || '—' }}</div>
              <div class="stage-desc">{{ versionInfo?.exp_info?.exp_desc || (expVersion ? '' : '还没有上传代码') }}</div>
              <div class="stage-time">{{ versionInfo?.exp_info?.exp_time ? fromUnix(versionInfo.exp_info.exp_time) : '' }}</div>
            </div>
          </div>
          <n-icon class="stage-arrow"><ArrowForwardOutline /></n-icon>
          <div class="stage" :class="auditStage.tone">
            <div class="stage-icon">
              <n-icon>
                <HourglassOutline v-if="auditStage.tone === 'active'" />
                <AlertCircleOutline v-else-if="auditStage.tone === 'error'" />
                <CheckmarkCircle v-else />
              </n-icon>
            </div>
            <div class="stage-body">
              <div class="stage-label">微信审核 · <span class="stage-state">{{ auditStage.label }}</span></div>
              <div class="stage-version">{{ latestAudit?.user_version || '—' }}</div>
              <div class="stage-desc">{{ latestAudit?.user_desc || (latestAudit ? '' : '还没有提交过审核') }}</div>
              <div class="stage-time">{{ latestAudit ? fromUnix(latestAudit.submit_audit_time) : '' }}</div>
            </div>
          </div>
          <n-icon class="stage-arrow"><ArrowForwardOutline /></n-icon>
          <div class="stage" :class="releaseVersion ? 'live' : 'idle'">
            <div class="stage-icon"><n-icon><RocketOutline /></n-icon></div>
            <div class="stage-body">
              <div class="stage-label">线上版本</div>
              <div class="stage-version">{{ releaseVersion || '—' }}</div>
              <div class="stage-desc">{{ versionInfo?.release_info?.release_desc || (releaseVersion ? '' : '还没有发布过') }}</div>
              <div class="stage-time">{{ versionInfo?.release_info?.release_time ? fromUnix(versionInfo.release_info.release_time) : '' }}</div>
            </div>
          </div>
        </div>

        <div class="next-step" :class="nextStep.tone">
          <div class="next-text">
            <div class="next-title">{{ nextStep.title }}</div>
            <div class="next-desc">{{ nextStep.desc }}</div>
          </div>
          <div class="next-actions">
            <n-button v-if="nextStep.secondary" @click="runStep(nextStep.secondary)">{{ nextStep.secondary.label }}</n-button>
            <n-button v-if="nextStep.primary" type="primary" :loading="refreshing && !nextStep.primary.action" @click="runStep(nextStep.primary)">
              {{ nextStep.primary.label }}
            </n-button>
          </div>
        </div>

        <div v-if="grayPlan && (grayPlan.status === 1 || grayPlan.status === 2)" class="gray-strip">
          <div>
            <b>分阶段发布{{ grayPlanStatusLabels[grayPlan.status] || '' }}</b>
            <span class="hint">已覆盖 {{ grayPlan.gray_percentage }}% 的用户</span>
          </div>
          <n-progress type="line" :percentage="grayPlan.gray_percentage" :show-indicator="false" :height="6" class="gray-bar" />
          <div class="gray-actions">
            <n-button size="small" @click="openAction('gray_release')">调整比例</n-button>
            <n-button size="small" @click="openAction('cancel_gray_release')">停止</n-button>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div class="tab-card surface-card">
        <n-tabs v-model:value="tab" type="line" animated>
          <n-tab-pane name="overview" tab="基本信息">
            <div class="info-grid">
              <div class="info-item">
                <div class="info-label">主体</div>
                <div class="info-value">{{ authorizer.principal_name || '—' }}</div>
              </div>
              <div class="info-item">
                <div class="info-label">原始 ID</div>
                <div class="info-value mono">{{ authorizer.user_name || '—' }}</div>
              </div>
              <div class="info-item">
                <div class="info-label">接入时间</div>
                <div class="info-value">{{ formatTime(authorizer.auth_time) }}</div>
              </div>
              <div class="info-item">
                <div class="info-label">服务状态</div>
                <div class="info-value">
                  <template v-if="visitStatus !== null">
                    <span :class="visitStatus === 1 ? 'ok-text' : 'bad-text'">{{ visitStatus === 1 ? '正常' : '已暂停' }}</span>
                    <n-button text size="small" type="primary" class="inline-link" :disabled="!active" @click="openAction('set_visit_status')">修改</n-button>
                  </template>
                  <span v-else>—</span>
                </div>
              </div>
            </div>

            <div class="two-col">
              <div class="block">
                <div class="block-title">服务类目</div>
                <div v-if="categories?.categories?.length" class="row-list">
                  <div v-for="(item, index) in categories.categories" :key="index" class="row-item">
                    <span>{{ item.first_name }} › {{ item.second_name }}</span>
                    <n-tag size="small" :bordered="false" :type="item.audit_status === 3 ? 'primary' : item.audit_status === 1 ? 'info' : 'error'">
                      {{ item.audit_status === 3 ? '可用' : item.audit_status === 1 ? '审核中' : '未通过' }}
                    </n-tag>
                  </div>
                </div>
                <div v-else class="hint">还没有配置服务类目。提交审核前需要先在小程序后台「设置 → 服务类目」中添加。</div>
              </div>

              <div class="block">
                <div class="block-title">授权的权限</div>
                <div class="tag-cloud">
                  <n-tag v-for="id in permissionIds" :key="id" size="small" :bordered="false" :type="id === 18 ? 'primary' : 'default'">
                    {{ permissionLabel(id) }}
                  </n-tag>
                  <span v-if="permissionIds.length === 0" class="hint">暂无权限信息，点击右上角刷新试试</span>
                </div>
              </div>
            </div>

            <div class="block">
              <div class="block-head">
                <div>
                  <div class="block-title">历史版本</div>
                  <div class="hint">线上版本出问题时，可以快速回退到之前的版本。</div>
                </div>
                <n-button v-if="!historyLoaded" size="small" :loading="historyLoading" @click="loadHistory">查看历史版本</n-button>
              </div>
              <div v-if="historyVersions.length" class="row-list">
                <div v-for="item in historyVersions" :key="item.app_version" class="row-item">
                  <div>
                    <b>{{ item.user_version || item.app_version }}</b>
                    <span class="hint"> · {{ item.user_desc || '无说明' }} · {{ fromUnix(item.commit_time) }}</span>
                  </div>
                  <n-button size="small" :disabled="!active" @click="openAction('rollback', item.app_version)">回退到此版本</n-button>
                </div>
              </div>
              <div v-else-if="historyLoaded" class="hint">没有可回退的版本。</div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="audits" tab="审核记录">
            <div class="pane-toolbar">
              <span class="hint">每次提交审核的结果。未通过时可查看驳回原因和截图。</span>
              <n-button size="small" :loading="refreshing" @click="refreshLatestAudit">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
                刷新
              </n-button>
            </div>
            <n-data-table :columns="auditColumns" :data="audits" :loading="auditsLoading" :pagination="false" :scroll-x="860">
              <template #empty><EmptyState compact :icon="TimeOutline" title="还没有审核记录" /></template>
            </n-data-table>
            <div v-if="auditsPageCount > 1" class="pager">
              <n-pagination v-model:page="auditsPage" :page-count="auditsPageCount" @update:page="loadAudits" />
            </div>
          </n-tab-pane>

          <n-tab-pane name="releases" tab="发布记录">
            <n-data-table :columns="releaseColumns" :data="releases" :loading="releasesLoading" :pagination="false" :scroll-x="760">
              <template #empty><EmptyState compact :icon="RocketOutline" title="还没有发布记录" /></template>
            </n-data-table>
            <div v-if="releasesPageCount > 1" class="pager">
              <n-pagination v-model:page="releasesPage" :page-count="releasesPageCount" @update:page="loadReleases" />
            </div>
          </n-tab-pane>

          <n-tab-pane name="domains" tab="服务器域名">
            <div class="pane-toolbar">
              <span class="hint">小程序只能访问这里配置过的域名。每月最多修改 50 次。</span>
              <n-button size="small" type="primary" :disabled="!active" @click="openAction('modify_domain')">修改域名</n-button>
            </div>
            <n-spin :show="domainsLoading">
              <div v-if="domains" class="domain-grid">
                <div v-for="group in domainGroups" :key="group.api" class="domain-group" :class="{ empty: !group.values.length }">
                  <div class="domain-label">{{ group.label }} <span class="api-name">{{ group.api }}</span></div>
                  <div v-if="group.values.length" class="domain-values">
                    <span v-for="value in group.values" :key="value" class="mono">{{ value }}</span>
                  </div>
                  <div v-else class="hint">未配置</div>
                </div>
              </div>
              <EmptyState v-else-if="!domainsLoading" compact :icon="GlobeOutline" title="无法读取域名" description="请确认已授权相关权限，或稍后重试。" />
              <n-alert v-if="domains?.no_icp_domain?.length" type="error" :bordered="false" class="icp-alert">
                以下域名未备案，可能无法访问：{{ domains.no_icp_domain.join('、') }}
              </n-alert>
              <div v-if="domains && configuredDomainCount === 0" class="hint" style="margin-top: 10px">还没有配置任何域名。</div>
            </n-spin>
          </n-tab-pane>

          <n-tab-pane name="testers" tab="体验成员">
            <div class="tester-form">
              <n-input v-model:value="testerWechatid" placeholder="输入对方的微信号" class="tester-input" @keyup.enter="bindTester" />
              <n-button type="primary" :loading="bindingTester" :disabled="!active" @click="bindTester">添加</n-button>
            </div>
            <div class="hint" style="margin-bottom: 12px">体验成员可以扫描体验版二维码，在正式发布前试用新版本。</div>
            <n-spin :show="testersLoading">
              <div v-if="testers.length" class="row-list">
                <div v-for="(tester, index) in testers" :key="tester" class="row-item">
                  <div class="tester-name">
                    <n-icon><PeopleOutline /></n-icon>
                    成员 {{ index + 1 }}
                    <span class="hint mono">{{ tester.slice(0, 10) }}…</span>
                  </div>
                  <n-button size="small" quaternary :disabled="!active" @click="unbindTester(tester, index)">移除</n-button>
                </div>
              </div>
              <EmptyState v-else-if="!testersLoading" compact :icon="PeopleOutline" title="还没有体验成员" />
            </n-spin>
          </n-tab-pane>

          <n-tab-pane name="qrcode" tab="二维码">
            <div class="qr-layout">
              <div class="qr-form">
                <div class="choice-grid">
                  <button type="button" class="choice" :class="{ selected: qrType === 'trial' }" @click="qrType = 'trial'">
                    <div class="choice-title">体验版二维码</div>
                    <div class="choice-desc">给体验成员扫码试用，有效期较短</div>
                  </button>
                  <button type="button" class="choice" :class="{ selected: qrType === 'unlimited' }" @click="qrType = 'unlimited'">
                    <div class="choice-title">小程序码</div>
                    <div class="choice-desc">打开线上版本，可长期使用，适合印刷推广</div>
                  </button>
                </div>
                <div class="field">
                  <div class="field-label">打开的页面 <span class="hint">留空则打开首页</span></div>
                  <n-input v-model:value="qrPath" placeholder="例如 pages/index/index" />
                </div>
                <template v-if="qrType === 'unlimited'">
                  <div class="field">
                    <div class="field-label">携带参数</div>
                    <n-input v-model:value="qrScene" placeholder="例如 id=123，最多 32 个字符" maxlength="32" />
                    <div class="hint" style="margin-top: 6px">小程序可在页面中读取该参数，用于区分来源或打开指定内容。</div>
                  </div>
                  <div class="field">
                    <div class="field-label">尺寸</div>
                    <n-input-number v-model:value="qrWidth" :min="280" :max="1280" :step="50" style="width: 160px">
                      <template #suffix>px</template>
                    </n-input-number>
                  </div>
                </template>
                <n-button type="primary" :loading="qrLoading" @click="generateQR">生成二维码</n-button>
              </div>
              <div class="qr-preview">
                <div v-if="qrUrl" class="qr-result">
                  <img :src="qrUrl" alt="小程序二维码" class="qr-image" />
                  <n-button @click="downloadQR">
                    <template #icon><n-icon><DownloadOutline /></n-icon></template>
                    下载
                  </n-button>
                </div>
                <div v-else class="qr-empty">
                  <n-icon :size="34"><QrCodeOutline /></n-icon>
                  <span>生成后在这里预览</span>
                </div>
              </div>
            </div>
          </n-tab-pane>
        </n-tabs>
      </div>
    </template>

    <BatchActionModal
      v-model:show="actionShow"
      :action="actionType"
      :appids="authorizer ? [authorizer.appid] : []"
      :apps="actionApps"
      :initial-domains="domains"
      :initial-rollback-version="rollbackPreset"
      @created="onJobCreated"
    />

    <n-modal v-model:show="screenshotShow" preset="card" title="驳回截图" :style="{ width: 'min(820px, 94vw)' }">
      <div v-if="screenshotLoading" style="padding: 30px; text-align: center"><n-spin /></div>
      <n-alert v-else-if="screenshotError" type="error" :bordered="false">{{ screenshotError }}</n-alert>
      <div v-else class="screenshot-grid">
        <img v-for="url in screenshotUrls" :key="url" :src="url" alt="审核截图" />
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
.loading-card {
  padding: 80px;
  text-align: center;
}

/* ---- hero ---- */
.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  margin: 4px 0 16px;
  flex-wrap: wrap;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.hero-info {
  min-width: 0;
}

.hero-name {
  font-size: 21px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.hero-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 12px;
  flex-wrap: wrap;
}

.dot {
  color: var(--text-faint);
}

.hero-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.hero-actions {
  display: flex;
  gap: 8px;
}

/* ---- pipeline ---- */
.pipeline-card {
  padding: 18px 20px;
  margin-bottom: 14px;
}

.pipeline {
  display: grid;
  grid-template-columns: 1fr auto 1fr auto 1fr;
  align-items: stretch;
  gap: 10px;
}

.stage {
  display: flex;
  gap: 12px;
  padding: 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  min-width: 0;
}

.stage-icon {
  flex: none;
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: var(--surface);
  color: var(--text-faint);
  font-size: 18px;
}

.stage.done .stage-icon,
.stage.live .stage-icon {
  background: var(--primary-soft);
  color: var(--primary);
}

.stage.active .stage-icon {
  background: var(--primary);
  color: #fff;
}

.stage.error .stage-icon {
  background: var(--danger-soft);
  color: var(--danger);
}

.stage.live {
  background: var(--primary-faint);
  box-shadow: inset 0 0 0 1px var(--primary-soft);
}

.stage-body {
  min-width: 0;
}

.stage-label {
  color: var(--text-muted);
  font-size: 12px;
}

.stage.active .stage-state {
  color: var(--primary);
  font-weight: 600;
}

.stage.error .stage-state {
  color: var(--danger);
  font-weight: 600;
}

.stage-version {
  margin-top: 4px;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.stage-desc {
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stage-time {
  margin-top: 2px;
  color: var(--text-faint);
  font-size: 11px;
  min-height: 15px;
}

.stage-arrow {
  align-self: center;
  color: var(--text-faint);
  font-size: 16px;
}

.next-step {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  border-left: 3px solid var(--text-faint);
  background: var(--surface-soft);
}

.next-step.primary,
.next-step.info,
.next-step.success {
  border-left-color: var(--primary);
  background: var(--primary-faint);
}

.next-step.error {
  border-left-color: var(--danger);
  background: var(--danger-soft);
}

.next-title {
  font-size: 14.5px;
  font-weight: 650;
}

.next-desc {
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 12.5px;
  line-height: 1.6;
}

.next-step.error .next-desc {
  color: var(--text);
}

.next-actions {
  display: flex;
  gap: 8px;
  flex: none;
}

.gray-strip {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 16px;
  margin-top: 10px;
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  font-size: 13px;
}

.gray-strip .hint {
  margin-left: 8px;
}

.gray-actions {
  display: flex;
  gap: 6px;
}

/* ---- tabs ---- */
.tab-card {
  padding: 6px 20px 20px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 6px;
}

.info-item {
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  min-width: 0;
}

.info-label {
  color: var(--text-muted);
  font-size: 12px;
}

.info-value {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 13.5px;
  font-weight: 560;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ok-text {
  color: var(--primary);
}

.bad-text {
  color: var(--danger);
}

.two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 24px;
}

.block {
  margin-top: 22px;
}

.block-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.block-title {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 650;
}

.block-head .block-title {
  margin-bottom: 2px;
}

.row-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.row-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  font-size: 13px;
}

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.pane-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 6px 0 14px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

:deep(.reject-reason) {
  color: var(--danger);
  font-size: 12.5px;
}

.domain-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.domain-group {
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  min-width: 0;
}

.domain-label {
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 620;
}

.api-name {
  margin-left: 4px;
  color: var(--text-faint);
  font-size: 11px;
  font-weight: 400;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.domain-values {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 12px;
  word-break: break-all;
}

.icp-alert {
  margin-top: 12px;
}

.tester-form {
  display: flex;
  gap: 8px;
  margin: 6px 0 8px;
}

.tester-input {
  max-width: 300px;
}

.tester-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.qr-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 24px;
  padding: 6px 0;
}

.qr-form {
  max-width: 520px;
}

.choice-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 18px;
}

.choice {
  padding: 13px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.choice.selected {
  border-color: var(--primary);
  background: var(--primary-faint);
}

.choice-title {
  font-size: 14px;
  font-weight: 650;
}

.choice-desc {
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 12px;
}

.field {
  margin-bottom: 16px;
}

.field-label {
  margin-bottom: 7px;
  font-size: 13px;
  font-weight: 600;
}

.field-label .hint {
  margin-left: 6px;
  font-weight: 400;
}

.qr-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 260px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  padding: 16px;
}

.qr-result {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.qr-image {
  width: 220px;
  height: 220px;
  object-fit: contain;
  background: #fff;
  border-radius: var(--radius-md);
}

.qr-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--text-faint);
  font-size: 12px;
}

.screenshot-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.screenshot-grid img {
  width: 100%;
  border-radius: var(--radius-sm);
}

@media (max-width: 1100px) {
  .pipeline {
    grid-template-columns: 1fr;
  }

  .stage-arrow {
    transform: rotate(90deg);
    justify-self: center;
  }

  .info-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .two-col,
  .qr-layout {
    grid-template-columns: 1fr;
  }

  .domain-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .next-step {
    flex-direction: column;
    align-items: stretch;
  }

  .gray-strip {
    grid-template-columns: 1fr;
  }

  .tab-card {
    padding: 6px 12px 16px;
  }

  .domain-grid,
  .choice-grid,
  .screenshot-grid {
    grid-template-columns: 1fr;
  }
}
</style>

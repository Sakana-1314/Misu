<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import {
  ArrowBackOutline,
  CameraOutline,
  CopyOutline,
  DocumentTextOutline,
  MoonOutline,
  QrCodeOutline,
  RefreshOutline,
} from '@vicons/ionicons5'
import { api, ApiError } from '@/api/client'
import type {
  AuditRecord,
  Authorizer,
  DomainResponse,
  GrayPlan,
  HistoryVersion,
  Job,
  LatestAudit,
  Paged,
  ReleaseRecord,
  SettingCategoriesResponse,
  VersionInfo,
} from '@/types/api'
import { errorMessage } from '@/utils/error'
import { downloadBlob } from '@/utils/download'
import { formatTime, fromUnix } from '@/utils/format'
import { grayPlanStatusLabels, jobTypeOptions, permissionLabel } from '@/utils/constants'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'
import CopyButton from '@/components/CopyButton.vue'
import BatchActionModal from '@/components/BatchActionModal.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

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
  release: '发布',
  rollback: '版本回退',
  gray: '分阶段发布',
  cancel_gray: '取消',
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
    message.success('小程序信息已刷新')
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
    message.warning('请填写体验者微信号')
    return
  }
  bindingTester.value = true
  try {
    await api.post(`/apps/${appid}/testers`, { wechatid: testerWechatid.value.trim() })
    message.success('体验者已绑定')
    testerWechatid.value = ''
    await loadTesters()
  } finally {
    bindingTester.value = false
  }
}

function unbindTester(userstr: string) {
  dialog.warning({
    title: '解绑体验者',
    content: `确定解绑体验者 ${userstr.slice(0, 16)}${userstr.length > 16 ? '…' : ''}？`,
    positiveText: '解绑',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.delete(`/apps/${appid}/testers`, { data: { userstr } })
      message.success('体验者已解绑')
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

async function openAction(type: string, preset = '') {
  if (!active.value) {
    message.warning('该小程序已取消授权，请重新授权后再操作')
    return
  }
  if (type === 'modify_domain' && !domainsLoaded.value) {
    await loadDomains()
  }
  rollbackPreset.value = preset
  actionType.value = type
  actionShow.value = true
}

function onJobCreated(job: Job) {
  message.success(`已提交 ${job.total} 个小程序的任务`)
  window.setTimeout(() => void loadAll(), 1500)
}

const auditColumns = computed<DataTableColumns<AuditRecord>>(() => [
  { title: '审核单', key: 'audit_id', width: 110, render: (row) => (row.audit_id ? `#${row.audit_id}` : '—') },
  { title: '版本', key: 'user_version', width: 110, render: (row) => row.user_version || '—' },
  { title: '状态', key: 'status', width: 104, render: (row) => h(StatusTag, { kind: 'audit', value: row.status }) },
  {
    title: '原因 / 说明',
    key: 'reason',
    minWidth: 220,
    render: (row) => h('span', { class: 'cell-muted' }, row.reason || row.user_desc || '—'),
  },
  { title: '提交时间', key: 'submitted_at', width: 170, render: (row) => formatTime(row.submitted_at) },
  {
    title: '操作',
    key: 'actions',
    width: 216,
    render: (row) =>
      h('div', { class: 'inline-actions' }, [
        h(NButton, { onClick: () => refreshAudit(row) }, { default: () => '刷新' }),
        ...(row.screenshot
          ? [h(NButton, { onClick: () => openScreenshot(row) }, { default: () => '截图' })]
          : []),
      ]),
  },
])

const releaseColumns = computed<DataTableColumns<ReleaseRecord>>(() => [
  {
    title: '操作',
    key: 'action',
    width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => releaseActionLabels[row.action] || row.action }),
  },
  { title: '版本', key: 'version', width: 140, render: (row) => row.version || '—' },
  { title: '说明', key: 'description', minWidth: 220, render: (row) => h('span', { class: 'cell-muted' }, row.description || '—') },
  { title: '时间', key: 'released_at', width: 180, render: (row) => formatTime(row.released_at) },
])

function clearScreenshotUrls() {
  for (const url of screenshotUrls.value) URL.revokeObjectURL(url)
  screenshotUrls.value = []
}

async function openScreenshot(row: AuditRecord) {
  screenshotShow.value = true
  screenshotLoading.value = true
  screenshotError.value = ''
  clearScreenshotUrls()
  const ids = (row.screenshot || '').split('|').map((item) => item.trim()).filter(Boolean)
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
    message.warning('生成不限制数量的小程序码需要填写 scene')
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
  if (qrBlob.value) downloadBlob(qrBlob.value, `misu-qrcode-${appid}.jpg`)
}

const actionOptions = jobTypeOptions.map((item) => ({ label: item.label, key: item.value }))

const domainGroups = computed(() => {
  const value = domains.value
  if (!value) return []
  return [
    { label: 'request', values: value.requestdomain || [] },
    { label: 'socket', values: value.wsrequestdomain || [] },
    { label: 'uploadFile', values: value.uploaddomain || [] },
    { label: 'downloadFile', values: value.downloaddomain || [] },
    { label: 'UDP', values: value.udpdomain || [] },
    { label: 'TCP', values: value.tcpdomain || [] },
  ]
})
</script>

<template>
  <div>
    <div v-if="loading && !authorizer" class="surface-card" style="padding: 60px; text-align: center">
      <n-spin size="large" />
    </div>

    <div v-else-if="appError" class="surface-card empty-state">
      <h3>无法加载小程序</h3>
      <p>{{ appError }}</p>
      <n-button type="primary" @click="router.push('/authorizers')">返回</n-button>
    </div>

    <template v-else-if="authorizer">
      <div class="detail-hero surface-card">
        <div class="hero-left">
          <n-button circle class="icon-button" @click="router.push('/authorizers')">
            <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
          </n-button>
          <AppAvatar :src="authorizer.head_img" :name="authorizer.nick_name" :appid="authorizer.appid" :size="64" />
          <div class="hero-info">
            <div class="hero-name">{{ authorizer.nick_name || '未命名小程序' }}</div>
            <div class="hero-id">
              <span class="mono">{{ authorizer.appid }}</span>
              <CopyButton :text="authorizer.appid" label="复制" />
            </div>
            <div class="hero-tags">
              <StatusTag kind="authorizer" :value="authorizer.status" />
              <StatusTag kind="account" :value="authorizer.account_status" />
              <n-tag size="small" :bordered="false" :type="authorizer.verify_type_id === 0 ? 'primary' : 'default'">
                {{ authorizer.verify_type_id === 0 ? '微信认证' : '未认证' }}
              </n-tag>
              <n-tag size="small" :bordered="false">{{ authorizer.principal_name || '主体未知' }}</n-tag>
              <n-tag v-if="!hasCodePermission && active" size="small" type="warning" :bordered="false">缺少代码管理权限</n-tag>
            </div>
          </div>
        </div>
        <div class="hero-actions">
          <n-button :loading="refreshing" @click="refreshInfo">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
            刷新
          </n-button>
          <n-button :disabled="!active" @click="openAction('commit')">上传</n-button>
          <n-button type="primary" :disabled="!active" @click="openAction('submit_audit')">提审</n-button>
          <n-dropdown :options="actionOptions" trigger="click" @select="(key: string | number) => openAction(String(key))">
            <n-button :disabled="!active">更多</n-button>
          </n-dropdown>
        </div>
      </div>

      <n-alert v-if="!active" type="warning" :bordered="false" style="margin: 16px 0">该小程序已取消授权。</n-alert>
      <n-alert v-else-if="!hasCodePermission" type="warning" :bordered="false" style="margin: 16px 0">缺少代码管理权限，部分操作可能失败。</n-alert>

      <div class="version-grid">
        <div class="version-card surface-card">
          <div class="version-head"><span>线上版本</span></div>
          <div class="version-number">{{ versionInfo?.release_info?.release_version || '—' }}</div>
          <div class="version-desc">{{ versionInfo?.release_info?.release_desc || '暂无已发布版本' }}</div>
          <div class="version-time">{{ fromUnix(versionInfo?.release_info?.release_time) }}</div>
        </div>
        <div class="version-card surface-card">
          <div class="version-head"><span>体验版</span></div>
          <div class="version-number">{{ versionInfo?.exp_info?.exp_version || '—' }}</div>
          <div class="version-desc">{{ versionInfo?.exp_info?.exp_desc || '暂无体验版' }}</div>
          <div class="version-time">{{ fromUnix(versionInfo?.exp_info?.exp_time) }}</div>
        </div>
        <div class="version-card surface-card">
          <div class="version-head">
            <span>最新审核</span>
            <StatusTag v-if="latestAudit" kind="audit" :value="latestAudit.status" />
          </div>
          <div class="version-number">{{ latestAudit?.user_version || '—' }}</div>
          <div class="version-desc">{{ latestAudit?.reason || latestAudit?.user_desc || '暂无审核记录' }}</div>
          <div class="version-time">{{ latestAudit ? `提交于 ${fromUnix(latestAudit.submit_audit_time)}` : '—' }}</div>
        </div>
      </div>

      <div class="tab-card surface-card">
        <n-tabs v-model:value="tab" type="line" animated>
          <n-tab-pane name="overview" tab="概览">
            <div class="overview">
              <n-descriptions :column="3" label-placement="top" bordered size="small">
                <n-descriptions-item label="主体名称">{{ authorizer.principal_name || '—' }}</n-descriptions-item>
                <n-descriptions-item label="授权时间">{{ formatTime(authorizer.auth_time) }}</n-descriptions-item>
                <n-descriptions-item label="服务状态">
                  <template v-if="visitStatus !== null">
                    <n-tag size="small" :bordered="false" :type="visitStatus === 1 ? 'primary' : 'error'">
                      {{ visitStatus === 1 ? '正常' : '已暂停服务' }}
                    </n-tag>
                  </template>
                  <span v-else>—</span>
                </n-descriptions-item>
              </n-descriptions>

              <div class="section-block">
                <div class="row-between">
                  <div>
                    <div class="section-title">权限集</div>
                  </div>
                </div>
                <div class="tag-cloud">
                  <n-tag v-for="id in permissionIds" :key="id" size="small" :bordered="false" :type="id === 18 ? 'primary' : 'default'">
                    {{ permissionLabel(id) }}
                  </n-tag>
                  <span v-if="permissionIds.length === 0" class="cell-muted">暂无权限集信息，请刷新授权信息</span>
                </div>
              </div>

              <div class="section-block">
                <div class="row-between">
                  <div>
                    <div class="section-title">已配置类目</div>
                  </div>
                </div>
                <div v-if="categories?.categories?.length" class="category-list">
                  <div v-for="(item, index) in categories.categories" :key="index" class="category-row">
                    <div>
                      <div class="category-name">{{ item.first_name }} / {{ item.second_name }}</div>
                    </div>
                    <n-tag size="small" :bordered="false" :type="item.audit_status === 3 ? 'primary' : item.audit_status === 1 ? 'info' : 'error'">
                      {{ item.audit_status === 3 ? '审核通过' : item.audit_status === 1 ? '审核中' : '未通过' }}
                    </n-tag>
                  </div>
                </div>
                <div v-else class="cell-muted">暂无已配置类目。提审前请先在小程序后台配置类目。</div>
              </div>

              <div class="two-col">
                <div class="section-block">
                  <div class="row-between">
                    <div>
                      <div class="section-title">分阶段发布</div>
                    </div>
                    <n-button :disabled="!active" @click="openAction('gray_release')">灰度</n-button>
                  </div>
                  <div v-if="grayPlan" class="gray-box">
                    <div class="gray-head">
                      <n-tag size="small" :bordered="false" :type="grayPlan.status === 1 ? 'info' : 'default'">
                        {{ grayPlanStatusLabels[grayPlan.status] || `状态 ${grayPlan.status}` }}
                      </n-tag>
                      <span class="gray-percent">{{ grayPlan.gray_percentage }}%</span>
                    </div>
                    <div class="cell-muted">项目成员优先：{{ grayPlan.support_debuger_first ? '是' : '否' }} · 体验成员优先：{{ grayPlan.support_experiencer_first ? '是' : '否' }}</div>
                    <n-button v-if="grayPlan.status === 1 || grayPlan.status === 2" style="margin-top: 10px" :disabled="!active" @click="openAction('cancel_gray_release')">取消</n-button>
                  </div>
                  <div v-else class="cell-muted">暂无进行中的灰度发布。</div>
                </div>

                <div class="section-block">
                  <div class="row-between">
                    <div>
                      <div class="section-title">历史版本</div>
                    </div>
                    <n-button :loading="historyLoading" @click="loadHistory">加载</n-button>
                  </div>
                  <div v-if="historyVersions.length" class="history-list">
                    <div v-for="item in historyVersions" :key="item.app_version" class="history-row">
                      <div>
                        <div class="history-version">{{ item.app_version }}</div>
                        <div class="cell-muted">{{ item.user_desc || item.user_version || '—' }} · {{ fromUnix(item.commit_time) }}</div>
                      </div>
                      <n-button @click="openAction('rollback', item.app_version)">回退</n-button>
                    </div>
                  </div>
                  <div v-else class="cell-muted">{{ historyLoaded ? '暂无' : '—' }}</div>
                </div>
              </div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="audits" tab="审核记录">
            <div class="pane-toolbar">
              <n-button :loading="refreshing" @click="refreshLatestAudit">刷新</n-button>
            </div>
            <n-data-table :columns="auditColumns" :data="audits" :loading="auditsLoading" :pagination="false" size="small" :scroll-x="1000" />
            <div v-if="auditsPageCount > 1" class="pager">
              <n-pagination v-model:page="auditsPage" :page-count="auditsPageCount" @update:page="loadAudits" />
            </div>
            <div v-if="!audits.length && !auditsLoading" class="empty-state">暂无审核记录。</div>
          </n-tab-pane>

          <n-tab-pane name="releases" tab="发布记录">
            <div class="pane-toolbar">
              <n-button :loading="releasesLoading" @click="loadReleases(releasesPage)">刷新</n-button>
            </div>
            <n-data-table :columns="releaseColumns" :data="releases" :loading="releasesLoading" :pagination="false" size="small" :scroll-x="900" />
            <div v-if="releasesPageCount > 1" class="pager">
              <n-pagination v-model:page="releasesPage" :page-count="releasesPageCount" @update:page="loadReleases" />
            </div>
            <div v-if="!releases.length && !releasesLoading" class="empty-state">暂无发布记录。</div>
          </n-tab-pane>

          <n-tab-pane name="domains" tab="服务器域名">
            <div class="pane-toolbar">
              <n-button :loading="domainsLoading" @click="loadDomains">刷新</n-button>
              <n-button type="primary" :disabled="!active" @click="openAction('modify_domain')">编辑</n-button>
            </div>
            <n-alert type="warning" :bordered="false" style="margin-bottom: 14px">修改域名每月最多 50 次。</n-alert>
            <div v-if="domains" class="domain-groups">
              <div v-for="group in domainGroups" :key="group.label" class="domain-group">
                <div class="domain-label">{{ group.label }}</div>
                <div v-if="group.values.length" class="tag-cloud">
                  <n-tag v-for="value in group.values" :key="value" size="small" :bordered="false" class="mono">{{ value }}</n-tag>
                </div>
                <div v-else class="cell-muted">未配置</div>
              </div>
              <div v-if="domains.no_icp_domain?.length" class="domain-group">
                <div class="domain-label error">未备案域名</div>
                <div class="tag-cloud">
                  <n-tag v-for="value in domains.no_icp_domain" :key="value" size="small" type="error" :bordered="false" class="mono">{{ value }}</n-tag>
                </div>
              </div>
            </div>
            <div v-else-if="!domainsLoading" class="empty-state">无法获取域名列表，请检查权限或稍后重试。</div>
          </n-tab-pane>

          <n-tab-pane name="testers" tab="体验者">
            <div class="tester-form">
              <n-input v-model:value="testerWechatid" placeholder="输入体验者微信号" style="max-width: 280px" />
              <n-button type="primary" :loading="bindingTester" :disabled="!active" @click="bindTester">绑定</n-button>
              <n-button :loading="testersLoading" @click="loadTesters">刷新</n-button>
            </div>
            <div v-if="testers.length" class="tester-list">
              <div v-for="tester in testers" :key="tester" class="tester-row">
                <span class="mono">{{ tester }}</span>
                <n-button :disabled="!active" @click="unbindTester(tester)">解绑</n-button>
              </div>
            </div>
            <div v-else-if="!testersLoading" class="empty-state">暂无已绑定的体验者。</div>
          </n-tab-pane>

          <n-tab-pane name="qrcode" tab="二维码">
            <div class="qr-layout">
              <div class="qr-form">
                <n-form label-placement="top">
                  <n-form-item label="二维码类型">
                    <n-radio-group v-model:value="qrType">
                      <n-radio-button value="trial">体验版二维码</n-radio-button>
                      <n-radio-button value="unlimited">不限制数量小程序码</n-radio-button>
                    </n-radio-group>
                  </n-form-item>
                  <n-form-item label="页面路径">
                    <n-input v-model:value="qrPath" placeholder="例如 pages/index/index?foo=bar" />
                  </n-form-item>
                  <n-form-item v-if="qrType === 'unlimited'" label="scene（必填）">
                    <n-input v-model:value="qrScene" placeholder="例如 a=1&b=2，最长 32 可见字符" />
                  </n-form-item>
                  <n-form-item v-if="qrType === 'unlimited'" label="宽度">
                    <n-input-number v-model:value="qrWidth" :min="280" :max="1280" style="width: 160px" />
                  </n-form-item>
                  <n-button type="primary" :loading="qrLoading" @click="generateQR">生成</n-button>
                </n-form>
              </div>
              <div class="qr-preview">
                <div v-if="qrUrl" class="qr-result">
                  <img :src="qrUrl" alt="小程序二维码" class="qr-image" />
                  <n-button @click="downloadQR">下载</n-button>
                </div>
                <div v-else class="qr-empty">
                  <n-icon :size="34"><QrCodeOutline /></n-icon>
                  <span>暂无二维码</span>
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

    <n-modal v-model:show="screenshotShow" preset="card" title="审核失败截图" :style="{ width: 'min(820px, 94vw)' }">
      <div v-if="screenshotLoading" style="padding: 30px; text-align: center"><n-spin /></div>
      <n-alert v-else-if="screenshotError" type="error" :bordered="false">{{ screenshotError }}</n-alert>
      <div v-else class="screenshot-grid">
        <img v-for="url in screenshotUrls" :key="url" :src="url" alt="审核截图" />
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
.detail-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 16px 18px;
  flex-wrap: wrap;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.hero-info {
  min-width: 0;
}

.hero-name {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.hero-id {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
  font-size: 11px;
  margin-top: 3px;
  flex-wrap: wrap;
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
  flex-wrap: wrap;
}

.version-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 12px 0;
}

.version-card {
  padding: 13px 15px;
}

.version-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--text-muted);
  font-size: 12px;
}

.version-number {
  margin-top: 8px;
  font-size: 21px;
  font-weight: 680;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.version-desc {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
  min-height: 18px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.version-time {
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 11px;
}

.tab-card {
  padding: 6px 16px 16px;
}

.overview {
  padding-top: 4px;
}

.section-block {
  margin-top: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 650;
}



.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.category-list {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.category-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 9px 11px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.category-name {
  font-size: 13px;
  font-weight: 560;
}

.two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}

.gray-box {
  margin-top: 8px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.gray-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.gray-percent {
  font-size: 20px;
  font-weight: 700;
  color: var(--primary);
}

.history-list {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.history-version {
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 12px;
  font-weight: 600;
}

.pane-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin: 4px 0 12px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.domain-groups {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.domain-group {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.domain-label {
  font-size: 12px;
  font-weight: 650;
  color: var(--text-muted);
}

.domain-label.error {
  color: #dc2626;
}

.tester-form {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin: 4px 0 16px;
}

.tester-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tester-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 9px 11px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  font-size: 12px;
  word-break: break-all;
}

.qr-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 20px;
  padding: 6px 0;
}

.qr-form {
  max-width: 480px;
}

.qr-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 240px;
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
  border-radius: var(--radius-sm);
}

.qr-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--text-muted);
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
  .version-grid {
    grid-template-columns: 1fr;
  }

  .two-col,
  .domain-groups {
    grid-template-columns: 1fr;
  }

  .qr-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .detail-hero {
    align-items: flex-start;
  }

  .hero-actions {
    width: 100%;
  }

  .tab-card {
    padding: 6px 12px 16px;
  }

  .screenshot-grid {
    grid-template-columns: 1fr;
  }
}
</style>

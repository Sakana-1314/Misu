<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDialog, useMessage } from 'naive-ui'
import { LayersOutline, RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { Authorizer, Job, JobItem, Paged } from '@/types/api'
import { jobStatusMeta, jobTypeLabel } from '@/utils/constants'
import { formatTime, relativeTimeText } from '@/utils/format'
import { errorMessage } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import StatusTag from '@/components/StatusTag.vue'
import AppAvatar from '@/components/AppAvatar.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const jobs = ref<Job[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const filter = ref<'all' | 'active' | 'failed'>('all')
const loading = ref(false)

const current = ref<Job | null>(null)
const items = ref<JobItem[]>([])
const itemsLoading = ref(false)
const apps = ref<Map<string, Authorizer>>(new Map())
let timer: number | undefined

const drawerShow = computed({
  get: () => current.value !== null,
  set: (value: boolean) => {
    if (!value) closeDetail()
  },
})

const isRunning = (job: Job) => job.status === 'queued' || job.status === 'running'
const finishedCount = (job: Job) => job.success_count + job.failed_count + job.skipped_count
const percent = (job: Job) => (job.total ? Math.round((finishedCount(job) / job.total) * 100) : 0)

function progressStatus(job: Job): 'default' | 'success' | 'error' | 'warning' {
  if (job.status === 'failed') return 'error'
  if (job.status === 'partial') return 'warning'
  return 'default'
}

async function load(next = page.value) {
  page.value = next
  loading.value = true
  try {
    const status = filter.value === 'active' ? 'running' : filter.value === 'failed' ? 'failed' : undefined
    const data = await api.get<Paged<Job>>('/jobs', { params: { page: page.value, page_size: pageSize, status } })
    jobs.value = data.items || []
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function loadApps() {
  try {
    const data = await api.get<Paged<Authorizer>>('/authorizers', { params: { page: 1, page_size: 100 }, silent: true })
    apps.value = new Map((data.items || []).map((item) => [item.appid, item]))
  } catch {
    // Names are a nicety; fall back to AppIDs.
  }
}

async function openDetail(id: number) {
  itemsLoading.value = true
  try {
    current.value = await api.get<Job>(`/jobs/${id}`)
    await loadItems()
  } catch (error) {
    message.error(errorMessage(error))
    closeDetail()
  } finally {
    itemsLoading.value = false
  }
  scheduleRefresh()
}

async function loadItems() {
  if (!current.value) return
  const data = await api.get<Paged<JobItem>>(`/jobs/${current.value.id}/items`, { params: { page: 1, page_size: 100 } })
  items.value = data.items || []
}

function scheduleRefresh() {
  if (timer) window.clearTimeout(timer)
  if (!current.value || !isRunning(current.value)) return
  timer = window.setTimeout(async () => {
    if (!current.value) return
    try {
      current.value = await api.get<Job>(`/jobs/${current.value.id}`, { silent: true })
      await loadItems()
      const index = jobs.value.findIndex((job) => job.id === current.value?.id)
      if (index >= 0 && current.value) jobs.value[index] = current.value
    } catch {
      // Keep the last snapshot on transient errors.
    }
    scheduleRefresh()
  }, 2000)
}

function closeDetail() {
  current.value = null
  items.value = []
  if (timer) window.clearTimeout(timer)
  if (route.query.id) void router.replace({ name: 'jobs' })
}

async function retry() {
  if (!current.value) return
  current.value = await api.post<Job>(`/jobs/${current.value.id}/retry`)
  message.success('已重新执行失败的小程序')
  await loadItems()
  scheduleRefresh()
  void load()
}

function cancel() {
  if (!current.value) return
  dialog.warning({
    title: '停止任务',
    content: '尚未开始的小程序将不再执行，已完成的不受影响。',
    positiveText: '停止',
    negativeText: '继续执行',
    onPositiveClick: async () => {
      if (!current.value) return
      current.value = await api.post<Job>(`/jobs/${current.value.id}/cancel`)
      message.success('已请求停止')
      void load()
    },
  })
}

function warnings(item: JobItem): string[] {
  const list = item.response?.warnings
  return Array.isArray(list) ? list.filter((text: unknown) => typeof text === 'string') : []
}

function appName(appid: string) {
  return apps.value.get(appid)?.nick_name || appid
}

watch(filter, () => void load(1))

watch(
  () => route.query.id,
  (id) => {
    if (id && Number(id) !== current.value?.id) void openDetail(Number(id))
  },
)

onMounted(() => {
  void load(1)
  void loadApps()
  if (route.query.id) void openDetail(Number(route.query.id))
})

onBeforeUnmount(() => {
  if (timer) window.clearTimeout(timer)
})
</script>

<template>
  <div>
    <PageHeader title="任务中心" description="上传、提审、发布等批量操作都在后台执行，这里可以查看每个小程序的执行结果。">
      <n-button :loading="loading" @click="load()">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
    </PageHeader>

    <div class="surface-card card-pad">
      <div class="toolbar">
        <n-radio-group v-model:value="filter" size="small">
          <n-radio-button value="all">全部</n-radio-button>
          <n-radio-button value="active">进行中</n-radio-button>
          <n-radio-button value="failed">失败</n-radio-button>
        </n-radio-group>
        <span class="hint">共 {{ total }} 个任务</span>
      </div>

      <n-spin :show="loading">
        <div v-if="jobs.length" class="job-list">
          <button v-for="job in jobs" :key="job.id" type="button" class="job-row" @click="openDetail(job.id)">
            <div class="job-main">
              <div class="job-title">{{ job.title || jobTypeLabel(job.type) }}</div>
              <div class="job-meta">
                #{{ job.id }} · {{ relativeTimeText(job.created_at) }}<template v-if="job.created_by"> · {{ job.created_by }}</template>
              </div>
            </div>
            <div class="job-progress hide-mobile">
              <n-progress
                type="line"
                :percentage="percent(job)"
                :status="progressStatus(job)"
                :show-indicator="false"
                :height="6"
                :processing="isRunning(job)"
              />
              <div class="job-counts">
                <span>{{ finishedCount(job) }}/{{ job.total }}</span>
                <span v-if="job.failed_count" class="failed">{{ job.failed_count }} 个失败</span>
              </div>
            </div>
            <StatusTag kind="job" :value="job.status" />
          </button>
        </div>
        <EmptyState
          v-else-if="!loading"
          :icon="LayersOutline"
          title="还没有任务"
          description="在「小程序」列表中勾选小程序，即可批量上传代码、提交审核或发布。"
        >
          <n-button type="primary" @click="router.push('/authorizers')">去选择小程序</n-button>
        </EmptyState>
      </n-spin>

      <div v-if="total > pageSize" class="pager">
        <n-pagination :page="page" :page-size="pageSize" :item-count="total" @update:page="load" />
      </div>
    </div>

    <n-drawer v-model:show="drawerShow" :width="560" placement="right">
      <n-drawer-content v-if="current" closable :native-scrollbar="false">
        <template #header>
          <div class="drawer-title">{{ current.title || jobTypeLabel(current.type) }}</div>
        </template>

        <div class="summary">
          <div class="summary-head">
            <StatusTag kind="job" :value="current.status" />
            <span class="hint">创建于 {{ formatTime(current.created_at) }}</span>
          </div>
          <n-progress
            type="line"
            :percentage="percent(current)"
            :status="progressStatus(current)"
            :height="8"
            :processing="isRunning(current)"
            indicator-placement="inside"
          />
          <div class="summary-grid">
            <div><b>{{ current.total }}</b><span>小程序</span></div>
            <div><b class="ok">{{ current.success_count }}</b><span>成功</span></div>
            <div><b :class="{ failed: current.failed_count }">{{ current.failed_count }}</b><span>失败</span></div>
            <div><b>{{ current.skipped_count }}</b><span>跳过</span></div>
          </div>
          <n-alert v-if="current.error" type="error" :bordered="false" style="margin-top: 12px">{{ current.error }}</n-alert>
          <div class="summary-actions">
            <n-button v-if="isRunning(current)" @click="cancel">停止任务</n-button>
            <n-button v-if="!isRunning(current) && current.failed_count > 0" type="primary" @click="retry">重试失败的小程序</n-button>
          </div>
        </div>

        <div class="section-label">执行明细</div>
        <n-spin :show="itemsLoading">
          <div class="item-list">
            <div v-for="item in items" :key="item.id" class="item-row">
              <AppAvatar :src="apps.get(item.authorizer_appid)?.head_img" :name="appName(item.authorizer_appid)" :appid="item.authorizer_appid" :size="32" />
              <div class="item-main">
                <div class="item-name">{{ appName(item.authorizer_appid) }}</div>
                <div v-if="item.error_message" class="item-error">{{ item.error_message }}</div>
                <div v-else class="item-sub mono">{{ item.authorizer_appid }}</div>
                <ul v-if="warnings(item).length" class="item-warnings">
                  <li v-for="(text, index) in warnings(item)" :key="index">{{ text }}</li>
                </ul>
              </div>
              <StatusTag kind="item" :value="item.status" />
            </div>
          </div>
        </n-spin>
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.job-list {
  display: flex;
  flex-direction: column;
}

.job-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 240px 84px;
  align-items: center;
  gap: 20px;
  width: 100%;
  padding: 13px 10px;
  border: none;
  border-bottom: 1px solid var(--border);
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
}

.job-row:last-child {
  border-bottom: none;
}

.job-row:hover {
  background: var(--surface-soft);
}

.job-row > :last-child {
  justify-self: end;
}

.job-title {
  font-size: 13.5px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.job-meta {
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 12px;
}

.job-counts {
  display: flex;
  justify-content: space-between;
  margin-top: 5px;
  color: var(--text-muted);
  font-size: 11.5px;
}

.failed {
  color: var(--danger);
}

.ok {
  color: var(--primary);
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.drawer-title {
  font-size: 16px;
  font-weight: 650;
}

.summary {
  padding: 14px 16px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.summary-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin-top: 14px;
  text-align: center;
}

.summary-grid b {
  display: block;
  font-size: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.summary-grid span {
  color: var(--text-muted);
  font-size: 12px;
}

.summary-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.summary-actions:empty {
  display: none;
}

.item-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.item-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.item-main {
  flex: 1;
  min-width: 0;
}

.item-name {
  font-size: 13px;
  font-weight: 600;
}

.item-sub {
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 11px;
}

.item-warnings {
  margin: 6px 0 0;
  padding-left: 16px;
  color: var(--warning);
  font-size: 12px;
  line-height: 1.6;
}

.item-error {
  margin-top: 3px;
  color: var(--danger);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-all;
}

@media (max-width: 768px) {
  .job-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}
</style>

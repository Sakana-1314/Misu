<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  AddOutline,
  AppsOutline,
  ChevronForwardOutline,
  CloudUploadOutline,
  HourglassOutline,
  LayersOutline,
  QrCodeOutline,
  RefreshOutline,
  RocketOutline,
  TimeOutline,
} from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { DashboardData } from '@/types/api'
import { relativeTimeText } from '@/utils/format'
import { jobTypeLabel } from '@/utils/constants'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'

const router = useRouter()
const data = ref<DashboardData | null>(null)
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    data.value = await api.get<DashboardData>('/dashboard')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

const stats = computed(() => data.value?.stats)
const audits = computed(() => data.value?.recent_audits || [])
const jobs = computed(() => data.value?.recent_jobs || [])

const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

const tiles = computed(() => [
  { label: '已接入小程序', value: stats.value?.authorizers ?? 0, icon: AppsOutline, to: '/authorizers' },
  { label: '审核中', value: stats.value?.auditing ?? 0, icon: HourglassOutline, to: '/authorizers' },
  { label: '近 7 天发布', value: stats.value?.released_7d ?? 0, icon: RocketOutline, to: '/authorizers' },
  { label: '进行中的任务', value: stats.value?.pending_jobs ?? 0, icon: LayersOutline, to: '/jobs' },
])

const shortcuts = [
  { title: '接入小程序', desc: '让管理员扫码授权', icon: AddOutline, to: '/authorizers?add=1' },
  { title: '批量上传代码', desc: '选择小程序后上传', icon: CloudUploadOutline, to: '/authorizers' },
  { title: '管理代码模板', desc: '把草稿加入模板库', icon: LayersOutline, to: '/templates' },
  { title: '连接公众平台', desc: '扫码查看名下账号', icon: QrCodeOutline, to: '/connect' },
]
</script>

<template>
  <div>
    <PageHeader :title="greeting" description="这里是所有小程序的最新动态。">
      <n-button :loading="loading" @click="load">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
    </PageHeader>

    <div class="stack-16">
      <n-alert v-if="data && !data.wechat_configured" type="warning" :bordered="false" title="第三方平台还没有配置完成">
        服务端尚未收到微信推送的验证票据，暂时无法管理小程序。请检查第三方平台的配置与消息接收地址。
      </n-alert>

      <div class="tiles">
        <button v-for="tile in tiles" :key="tile.label" type="button" class="tile surface-card" @click="router.push(tile.to)">
          <div class="tile-top">
            <span class="tile-label">{{ tile.label }}</span>
            <span class="tile-icon"><n-icon><component :is="tile.icon" /></n-icon></span>
          </div>
          <div class="metric-value">
            <n-skeleton v-if="loading && !data" text style="width: 40px" />
            <template v-else>{{ tile.value }}</template>
          </div>
        </button>
      </div>

      <div class="columns">
        <div class="surface-card card-pad">
          <div class="card-head">
            <div class="card-title">最近审核</div>
            <router-link to="/authorizers" class="more-link">全部小程序</router-link>
          </div>
          <div v-if="audits.length">
            <div v-for="audit in audits" :key="audit.id" class="list-row" @click="router.push(`/authorizers/${audit.authorizer_appid}`)">
              <div class="row-main">
                <AppAvatar :src="audit.head_img" :name="audit.nick_name" :appid="audit.authorizer_appid" :size="36" />
                <div class="list-row-main">
                  <div class="list-row-title">
                    {{ audit.nick_name || audit.authorizer_appid }}
                    <span class="version">{{ audit.user_version }}</span>
                  </div>
                  <div class="list-row-meta" :class="{ reject: audit.status === 1 }">
                    {{ audit.status === 1 && audit.reason ? audit.reason : relativeTimeText(audit.submitted_at || audit.updated_at) }}
                  </div>
                </div>
              </div>
              <StatusTag kind="audit" :value="audit.status" />
            </div>
          </div>
          <EmptyState v-else-if="!loading" compact :icon="TimeOutline" title="还没有审核记录" description="提交审核后，进度会显示在这里。" />
        </div>

        <div class="side">
          <div class="surface-card card-pad">
            <div class="card-head">
              <div class="card-title">最近任务</div>
              <router-link to="/jobs" class="more-link">任务中心</router-link>
            </div>
            <div v-if="jobs.length">
              <div v-for="job in jobs" :key="job.id" class="list-row" @click="router.push({ name: 'jobs', query: { id: String(job.id) } })">
                <div class="list-row-main">
                  <div class="list-row-title">{{ job.title || jobTypeLabel(job.type) }}</div>
                  <div class="list-row-meta">{{ relativeTimeText(job.created_at) }} · {{ job.success_count }}/{{ job.total }} 完成</div>
                </div>
                <StatusTag kind="job" :value="job.status" />
              </div>
            </div>
            <EmptyState v-else-if="!loading" compact :icon="LayersOutline" title="还没有任务" />
          </div>

          <div class="surface-card card-pad">
            <div class="card-title" style="margin-bottom: 10px">快捷操作</div>
            <div class="shortcuts">
              <button v-for="item in shortcuts" :key="item.title" type="button" class="shortcut" @click="router.push(item.to)">
                <span class="shortcut-icon"><n-icon><component :is="item.icon" /></n-icon></span>
                <span class="shortcut-text">
                  <span class="shortcut-title">{{ item.title }}</span>
                  <span class="shortcut-desc">{{ item.desc }}</span>
                </span>
                <n-icon class="shortcut-arrow"><ChevronForwardOutline /></n-icon>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tiles {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.tile {
  padding: 16px 18px;
  border: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.tile:hover {
  transform: translateY(-1px);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), 0 10px 26px -10px rgba(37, 99, 235, 0.22);
}

.tile-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.tile-label {
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 560;
}

.tile-icon {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 16px;
}

.tile .metric-value {
  margin-top: 10px;
}

.columns {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.side {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.more-link {
  color: var(--text-muted);
  font-size: 12px;
}

.more-link:hover {
  color: var(--primary);
}

.row-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.version {
  margin-left: 6px;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 12px;
}

.list-row-meta.reject {
  color: var(--danger);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.shortcuts {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.shortcut {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
}

.shortcut:hover {
  background: var(--surface-hover);
}

.shortcut-icon {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  background: var(--surface);
  color: var(--primary);
  font-size: 16px;
}

.shortcut-text {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.shortcut-title {
  font-size: 13px;
  font-weight: 600;
}

.shortcut-desc {
  color: var(--text-muted);
  font-size: 11.5px;
}

.shortcut-arrow {
  color: var(--text-faint);
}

@media (max-width: 1100px) {
  .columns {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .tiles {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

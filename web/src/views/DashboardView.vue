<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { AppsOutline, HourglassOutline, RefreshOutline, RocketOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { Authorizer, DashboardData, Paged, SyncResult } from '@/types/api'
import StatCard from '@/components/StatCard.vue'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'

const router = useRouter()
const message = useMessage()
const data = ref<DashboardData | null>(null)
const apps = ref<Authorizer[]>([])
const loading = ref(true)
const syncing = ref(false)

async function load() {
  loading.value = true
  try {
    const [dashboardResult, appsResult] = await Promise.allSettled([
      api.get<DashboardData>('/dashboard'),
      api.get<Paged<Authorizer>>('/authorizers', { params: { page: 1, page_size: 5 } }),
    ])
    if (dashboardResult.status === 'fulfilled') data.value = dashboardResult.value
    if (appsResult.status === 'fulfilled') apps.value = appsResult.value.items || []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

const stats = computed(() => data.value?.stats)

async function syncNow() {
  syncing.value = true
  try {
    const res = await api.post<SyncResult>('/authorizers/sync', null, { timeout: 300000 })
    message.success(`已同步 ${res.total} 个小程序`)
    await load()
  } finally {
    syncing.value = false
  }
}

</script>

<template>
  <div>
    <div class="page-toolbar">
      <n-button :loading="syncing" @click="syncNow">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        同步
      </n-button>
    </div>

    <n-spin :show="loading">
      <div class="stack-16">
        <n-alert v-if="data && !data.wechat_configured" type="error" :bordered="false">平台未连接</n-alert>
        <n-alert v-else-if="data && !data.platform_published" type="warning" :bordered="false">
          当前为测试模式，仅授权测试列表内的小程序可用。
        </n-alert>

        <div class="stat-grid">
          <StatCard label="小程序" :value="stats?.authorizers ?? 0" tone="primary">
            <template #icon><AppsOutline /></template>
          </StatCard>
          <StatCard label="审核中" :value="stats?.auditing ?? 0" tone="slate">
            <template #icon><HourglassOutline /></template>
          </StatCard>
          <StatCard label="近 7 天发布" :value="stats?.released_7d ?? 0" tone="slate">
            <template #icon><RocketOutline /></template>
          </StatCard>
        </div>

        <div class="surface-card app-card">
          <div class="card-head">
            <span>小程序</span>
            <router-link to="/authorizers">查看全部</router-link>
          </div>
          <div v-if="apps.length">
            <div v-for="app in apps" :key="app.appid" class="list-row" @click="router.push(`/authorizers/${app.appid}`)">
              <div class="app-main">
                <AppAvatar :src="app.head_img" :name="app.nick_name" :appid="app.appid" :size="36" />
                <div class="list-row-main">
                  <div class="list-row-title">{{ app.nick_name || app.appid }}</div>
                </div>
              </div>
              <StatusTag kind="authorizer" :value="app.status" />
            </div>
          </div>
          <div v-else class="empty-state">暂无小程序</div>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.app-card {
  padding: 6px 18px 10px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 0 8px;
  font-size: 14px;
  font-weight: 650;
}

.card-head a {
  font-size: 12px;
  color: var(--text-muted);
}

.card-head a:hover {
  color: var(--primary);
}

.app-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

@media (max-width: 900px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>

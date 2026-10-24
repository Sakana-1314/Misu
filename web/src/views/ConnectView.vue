<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'

interface MPAccount {
  appid: string
  name: string
  avatar: string
  user_name: string
  type: number
}

interface MPStatus {
  connected: boolean
  nick_name: string
  avatar: string
  last_login_at: string | null
  accounts: MPAccount[]
  account_count: number
}

type Phase = 'idle' | 'waiting' | 'scanned' | 'expired'

const message = useMessage()
const status = ref<MPStatus | null>(null)
const loading = ref(false)
const starting = ref(false)
const phase = ref<Phase>('idle')
const qrVersion = ref(0)
const refreshing = ref(false)
let timer: number | undefined

const connected = computed(() => status.value?.connected === true)
const accounts = computed(() => status.value?.accounts || [])

function stopPolling() {
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

async function loadStatus(silent = false) {
  if (!silent) loading.value = true
  try {
    status.value = await api.get<MPStatus>('/mpweb/status', { silent: true })
  } finally {
    loading.value = false
  }
}

async function startLogin() {
  starting.value = true
  stopPolling()
  try {
    await api.post('/mpweb/login', null, { silent: true })
    phase.value = 'waiting'
    qrVersion.value += 1
    timer = window.setInterval(() => void poll(), 2000)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取二维码失败')
  } finally {
    starting.value = false
  }
}

async function poll() {
  try {
    const state = await api.get<{ status: string; nick_name?: string }>('/mpweb/login/status', { silent: true })
    if (state.status === 'confirmed') {
      stopPolling()
      phase.value = 'idle'
      message.success(`已连接${state.nick_name ? `：${state.nick_name}` : ''}`)
      await loadStatus(true)
      return
    }
    if (state.status === 'expired') {
      stopPolling()
      phase.value = 'expired'
      return
    }
    if (state.status === 'scanned') {
      phase.value = 'scanned'
    }
  } catch {
    stopPolling()
    phase.value = 'expired'
  }
}

async function refreshAccounts() {
  refreshing.value = true
  try {
    const data = await api.post<{ items: MPAccount[] }>('/mpweb/accounts/refresh', null, { silent: true })
    if (status.value) status.value.accounts = data.items || []
  } catch (error) {
    message.error(error instanceof Error ? error.message : '刷新失败')
  } finally {
    refreshing.value = false
  }
}

async function logout() {
  try {
    await api.post('/mpweb/logout', null, { silent: true })
    phase.value = 'idle'
    stopPolling()
    await loadStatus(true)
    message.success('已断开连接')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '断开失败')
  }
}

onMounted(() => {
  void loadStatus()
})

onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="stack-12">
    <div class="surface-card connect-card">
      <div class="connect-head">
        <div>
          <div class="connect-title">网页版连接</div>
          <div class="connect-sub">扫码登录微信公众平台，直接管理该微信名下的全部小程序。</div>
        </div>
        <n-tag :type="connected ? 'primary' : 'default'" :bordered="false" size="small">
          {{ connected ? '已连接' : '未连接' }}
        </n-tag>
      </div>

      <div v-if="connected" class="account-row">
        <n-avatar v-if="status?.avatar" round :size="42" :src="status.avatar" />
        <n-avatar v-else round :size="42" color="#2563EB">{{ (status?.nick_name || '微').slice(0, 1) }}</n-avatar>
        <div class="account-meta">
          <div class="account-name">{{ status?.nick_name || '微信用户' }}</div>
          <div class="account-sub">{{ accounts.length }} 个小程序</div>
        </div>
        <div class="account-actions">
          <n-button :loading="refreshing" @click="refreshAccounts">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
            刷新
          </n-button>
          <n-button @click="logout">断开</n-button>
        </div>
      </div>

      <template v-else>
        <div class="login-body">
          <div class="qr-panel">
            <template v-if="phase === 'waiting' || phase === 'scanned'">
              <img :src="`/api/mpweb/login/qrcode?v=${qrVersion}`" alt="微信扫码登录" class="qr" />
              <div class="qr-hint">
                {{ phase === 'scanned' ? '已扫码，请在微信中确认' : '请使用微信扫码登录' }}
              </div>
            </template>
            <template v-else-if="phase === 'expired'">
              <div class="qr-placeholder">
                <span>二维码已失效</span>
                <n-button type="primary" :loading="starting" @click="startLogin">刷新</n-button>
              </div>
            </template>
            <template v-else>
              <div class="qr-placeholder">
                <span>需要扫码登录后才能管理小程序</span>
                <n-button type="primary" :loading="starting" @click="startLogin">扫码</n-button>
              </div>
            </template>
          </div>
        </div>
      </template>
    </div>

    <div v-if="connected" class="surface-card account-card">
      <div class="card-head">
        <span>名下小程序</span>
        <span class="card-count">{{ accounts.length }}</span>
      </div>
      <div v-if="accounts.length">
        <div v-for="account in accounts" :key="account.appid" class="list-row">
          <div class="app-main">
            <AppAvatar :src="account.avatar" :name="account.name" :appid="account.appid" :size="34" />
            <div class="list-row-main">
              <div class="list-row-title">{{ account.name || account.appid }}</div>
              <div class="list-row-meta mono">{{ account.appid }}</div>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="empty-state">暂无小程序</div>
    </div>
  </div>
</template>

<style scoped>
.connect-card {
  padding: 18px 20px;
}

.connect-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.connect-title {
  font-size: 15px;
  font-weight: 650;
}

.connect-sub {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.account-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.account-meta {
  flex: 1;
  min-width: 0;
}

.account-name {
  font-size: 14px;
  font-weight: 620;
}

.account-sub {
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 12px;
}

.account-actions {
  display: flex;
  gap: 8px;
}

.login-body {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

.qr-panel {
  width: 232px;
  padding: 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  text-align: center;
}

.qr {
  width: 200px;
  height: 200px;
  display: block;
  margin: 0 auto;
  border-radius: var(--radius-sm);
  background: #fff;
}

.qr-hint {
  margin-top: 10px;
  color: var(--text-muted);
  font-size: 12px;
}

.qr-placeholder {
  height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-muted);
  font-size: 12px;
}

.account-card {
  padding: 4px 14px 10px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 2px 8px;
  font-size: 14px;
  font-weight: 650;
}

.card-count {
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
}

.app-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}
</style>

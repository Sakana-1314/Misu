<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useDialog, useMessage } from 'naive-ui'
import {
  AlertCircleOutline,
  CheckmarkCircle,
  ChevronForwardOutline,
  PhonePortraitOutline,
  RefreshOutline,
  ScanOutline,
  ShieldCheckmarkOutline,
} from '@vicons/ionicons5'
import { api } from '@/api/client'
import { errorMessage } from '@/utils/error'
import { formatTime } from '@/utils/format'
import PageHeader from '@/components/PageHeader.vue'
import AppAvatar from '@/components/AppAvatar.vue'
import EmptyState from '@/components/EmptyState.vue'
import AddAuthorizerModal from '@/components/AddAuthorizerModal.vue'

interface MPAccount {
  appid?: string
  user_name: string
  name: string
  avatar: string
  bizuin: number
  kind: 'miniprogram' | 'official' | 'service'
  current: boolean
  authorizer_appid?: string
}

interface MPStatus {
  connected: boolean
  expired: boolean
  nick_name: string
  avatar: string
  user_name: string
  last_login_at: string | null
  accounts: MPAccount[]
  account_count: number
  switch_permitted: boolean
}

interface ScanState {
  status: 'waiting' | 'scanned' | 'confirmed' | 'expired' | 'cancelled' | 'failed'
  message?: string
  nick_name?: string
  account_count: number
  expires_at?: number
}

type Phase = 'loading' | 'waiting' | 'scanned' | 'confirmed' | 'expired' | 'cancelled' | 'failed'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const status = ref<MPStatus | null>(null)
const loading = ref(true)
const phase = ref<Phase>('loading')
const phaseMessage = ref('')
const qrVersion = ref(0)
const expiresAt = ref(0)
const now = ref(Date.now())
const refreshing = ref(false)
const addShow = ref(false)
const addAppid = ref('')
let pollTimer: number | undefined
let clockTimer: number | undefined
let alive = true

const connected = computed(() => status.value?.connected === true)
const accounts = computed(() => status.value?.accounts || [])

const kindLabels: Record<MPAccount['kind'], string> = {
  miniprogram: '小程序',
  official: '公众号',
  service: '服务号',
}

const groupedAccounts = computed(() =>
  (['miniprogram', 'official', 'service'] as const)
    .map((kind) => ({ kind, label: kindLabels[kind], items: accounts.value.filter((item) => item.kind === kind) }))
    .filter((group) => group.items.length > 0),
)

const remaining = computed(() => Math.max(0, Math.floor(expiresAt.value - now.value / 1000)))
const remainingText = computed(() => {
  const m = Math.floor(remaining.value / 60)
  const s = remaining.value % 60
  return `${m}:${String(s).padStart(2, '0')}`
})

const steps = computed(() => {
  const current = phase.value === 'scanned' ? 1 : phase.value === 'confirmed' ? 2 : 0
  return [
    { title: '打开微信扫一扫', desc: '用公众号或小程序的管理员微信扫码', done: current > 0, active: current === 0 },
    { title: '在手机上确认', desc: '有多个账号时，在手机上选择要登录的那个', done: current > 1, active: current === 1 },
    { title: '完成连接', desc: '登录状态会安全保存在服务器上', done: false, active: current === 2 },
  ]
})

function clearPoll() {
  if (pollTimer) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

async function loadStatus() {
  try {
    status.value = await api.get<MPStatus>('/mpweb/status', { silent: true })
  } catch (error) {
    message.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function startLogin() {
  clearPoll()
  phase.value = 'loading'
  phaseMessage.value = ''
  try {
    const state = await api.post<ScanState>('/mpweb/login', null, { silent: true })
    expiresAt.value = state.expires_at || Math.floor(Date.now() / 1000) + 240
    qrVersion.value += 1
    phase.value = 'waiting'
    schedulePoll()
  } catch (error) {
    phase.value = 'failed'
    phaseMessage.value = errorMessage(error)
  }
}

function schedulePoll() {
  clearPoll()
  if (!alive) return
  pollTimer = window.setTimeout(() => void poll(), 1500)
}

async function poll() {
  try {
    const state = await api.get<ScanState>('/mpweb/login/status', { silent: true })
    phaseMessage.value = state.message || ''
    switch (state.status) {
      case 'confirmed':
        phase.value = 'confirmed'
        message.success(state.nick_name ? `已连接「${state.nick_name}」` : '连接成功')
        await loadStatus()
        return
      case 'expired':
      case 'cancelled':
      case 'failed':
        phase.value = state.status
        return
      case 'scanned':
        phase.value = 'scanned'
        break
      default:
        phase.value = 'waiting'
    }
    schedulePoll()
  } catch (error) {
    phase.value = 'failed'
    phaseMessage.value = errorMessage(error)
  }
}

async function refreshAccounts() {
  refreshing.value = true
  try {
    await api.post('/mpweb/accounts/refresh', null, { silent: true })
    await loadStatus()
    message.success('账号列表已更新')
  } catch (error) {
    message.error(errorMessage(error))
    await loadStatus()
    if (!connected.value) void startLogin()
  } finally {
    refreshing.value = false
  }
}

function logout() {
  dialog.warning({
    title: '断开公众平台连接',
    content: '断开后需要重新扫码才能再次连接。微信端的登录不受影响。',
    positiveText: '断开',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.post('/mpweb/logout', null, { silent: true })
      message.success('已断开连接')
      await loadStatus()
      void startLogin()
    },
  })
}

function openAuthorize(account: MPAccount) {
  addAppid.value = account.appid || ''
  addShow.value = true
}

onMounted(async () => {
  clockTimer = window.setInterval(() => {
    now.value = Date.now()
    if (phase.value === 'waiting' && expiresAt.value && remaining.value === 0) {
      clearPoll()
      phase.value = 'expired'
    }
  }, 1000)
  await loadStatus()
  if (!connected.value) void startLogin()
})

onBeforeUnmount(() => {
  alive = false
  clearPoll()
  if (clockTimer) window.clearInterval(clockTimer)
})
</script>

<template>
  <div>
    <PageHeader
      title="公众平台连接"
      description="用管理员微信扫码登录微信公众平台，即可查看你名下的公众号与小程序，无需为每个账号单独授权。"
    />

    <div v-if="loading" class="surface-card loading-card"><n-spin /></div>

    <!-- Connected -->
    <div v-else-if="connected" class="stack-16">
      <div class="surface-card profile-card">
        <div class="profile-main">
          <div class="profile-avatar">
            <AppAvatar :src="status?.avatar" :name="status?.nick_name" :appid="status?.user_name || 'mp'" :size="54" />
            <span class="profile-dot" />
          </div>
          <div class="profile-meta">
            <div class="profile-name">{{ status?.nick_name || '公众平台账号' }}</div>
            <div class="profile-sub">
              <n-tag size="small" type="primary" :bordered="false" round>
                <template #icon><n-icon><ShieldCheckmarkOutline /></n-icon></template>
                已连接
              </n-tag>
              <span>上次扫码 {{ formatTime(status?.last_login_at) }}</span>
            </div>
          </div>
        </div>
        <div class="profile-actions">
          <n-button :loading="refreshing" @click="refreshAccounts">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
            刷新账号列表
          </n-button>
          <n-button @click="logout">断开连接</n-button>
        </div>
      </div>

      <div class="surface-card card-pad">
        <div class="card-head">
          <div>
            <div class="card-title">可管理的账号</div>
            <div class="card-sub">该微信作为管理员绑定的全部账号。已接入第三方平台的小程序可直接进入详情页批量管理。</div>
          </div>
          <span class="count-pill">{{ accounts.length }}</span>
        </div>

        <n-alert v-if="accounts.length > 1 && !status?.switch_permitted" type="info" :bordered="false" class="switch-hint">
          切换到其他账号需要重新扫码，并在手机上选择对应账号。
        </n-alert>

        <template v-if="accounts.length">
          <div v-for="group in groupedAccounts" :key="group.kind" class="account-group">
            <div class="section-label">{{ group.label }} · {{ group.items.length }}</div>
            <div class="account-list">
              <div v-for="account in group.items" :key="account.user_name || account.name" class="account-row">
                <AppAvatar :src="account.avatar" :name="account.name" :appid="account.user_name || account.name" :size="38" />
                <div class="account-text">
                  <div class="account-name">
                    {{ account.name || '未命名账号' }}
                    <n-tag v-if="account.current" size="tiny" :bordered="false" type="primary">当前登录</n-tag>
                  </div>
                  <div class="account-id mono">{{ account.appid || account.user_name || '—' }}</div>
                </div>
                <div class="account-side">
                  <n-button
                    v-if="account.authorizer_appid"
                    size="small"
                    secondary
                    type="primary"
                    @click="router.push(`/authorizers/${account.authorizer_appid}`)"
                  >
                    管理
                    <template #icon><n-icon><ChevronForwardOutline /></n-icon></template>
                  </n-button>
                  <template v-else-if="account.kind === 'miniprogram'">
                    <span class="hint hide-mobile">未接入平台</span>
                    <n-button size="small" @click="openAuthorize(account)">接入</n-button>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </template>
        <EmptyState v-else compact title="暂未读取到账号" description="点击「刷新账号列表」重新获取。" />
      </div>
    </div>

    <!-- Login -->
    <div v-else class="surface-card login-card">
      <div class="login-intro">
        <n-alert v-if="status?.expired && status.nick_name" type="warning" :bordered="false" class="expired-alert">
          「{{ status.nick_name }}」的登录已过期，重新扫码即可恢复。
        </n-alert>
        <div class="intro-title">扫码连接微信公众平台</div>
        <div class="intro-desc">登录后即可在这里查看你作为管理员的所有公众号与小程序。</div>

        <ol class="steps">
          <li v-for="(step, index) in steps" :key="step.title" :class="{ done: step.done, active: step.active }">
            <span class="step-index">
              <n-icon v-if="step.done"><CheckmarkCircle /></n-icon>
              <template v-else>{{ index + 1 }}</template>
            </span>
            <div>
              <div class="step-title">{{ step.title }}</div>
              <div class="step-desc">{{ step.desc }}</div>
            </div>
          </li>
        </ol>

        <div class="safety">
          <n-icon><ShieldCheckmarkOutline /></n-icon>
          <span>登录凭据加密保存在你的服务器上，可随时断开。</span>
        </div>
      </div>

      <div class="qr-side">
        <div class="qr-box" :class="phase">
          <img
            v-if="qrVersion > 0 && phase !== 'loading'"
            :key="qrVersion"
            :src="`/api/mpweb/login/qrcode?v=${qrVersion}`"
            alt="微信扫码登录二维码"
            class="qr-img"
          />
          <div v-if="phase === 'loading'" class="qr-overlay plain"><n-spin size="small" /><span>正在生成二维码…</span></div>

          <transition name="fade">
            <div v-if="phase === 'scanned'" class="qr-overlay">
              <n-icon :size="40" class="ok"><PhonePortraitOutline /></n-icon>
              <div class="overlay-title">扫码成功</div>
              <div class="overlay-desc">{{ phaseMessage || '请在手机上确认登录' }}</div>
            </div>
          </transition>
          <transition name="fade">
            <div v-if="phase === 'confirmed'" class="qr-overlay">
              <n-icon :size="42" class="ok"><CheckmarkCircle /></n-icon>
              <div class="overlay-title">登录成功</div>
              <div class="overlay-desc">正在读取账号信息…</div>
            </div>
          </transition>
          <transition name="fade">
            <button v-if="phase === 'expired' || phase === 'cancelled'" type="button" class="qr-overlay clickable" @click="startLogin">
              <n-icon :size="34"><RefreshOutline /></n-icon>
              <div class="overlay-title">{{ phase === 'expired' ? '二维码已过期' : '已取消登录' }}</div>
              <div class="overlay-desc">点击刷新二维码</div>
            </button>
          </transition>
          <transition name="fade">
            <div v-if="phase === 'failed'" class="qr-overlay">
              <n-icon :size="36" class="bad"><AlertCircleOutline /></n-icon>
              <div class="overlay-title">无法登录</div>
              <div class="overlay-desc">{{ phaseMessage || '请稍后重试' }}</div>
              <n-button size="small" type="primary" style="margin-top: 10px" @click="startLogin">重新扫码</n-button>
            </div>
          </transition>
        </div>

        <div class="qr-caption">
          <template v-if="phase === 'waiting'">
            <n-icon><ScanOutline /></n-icon>
            <span>微信扫一扫 · <span class="mono">{{ remainingText }}</span> 后过期</span>
          </template>
          <template v-else-if="phase === 'scanned'">等待手机确认…</template>
          <template v-else>&nbsp;</template>
        </div>
        <n-button v-if="phase === 'waiting' || phase === 'scanned'" text size="small" class="qr-refresh" @click="startLogin">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
          换一张
        </n-button>
      </div>
    </div>

    <AddAuthorizerModal v-model:show="addShow" :initial-appid="addAppid" @synced="loadStatus" />
  </div>
</template>

<style scoped>
.loading-card {
  padding: 80px;
  text-align: center;
}

/* ---- login ---- */
.login-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  overflow: hidden;
}

.login-intro {
  padding: 30px 32px;
}

.expired-alert {
  margin-bottom: 18px;
}

.intro-title {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.intro-desc {
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 13px;
}

.steps {
  list-style: none;
  margin: 26px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.steps li {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  color: var(--text-muted);
  transition: color 0.2s ease;
}

.steps li.active,
.steps li.done {
  color: var(--text);
}

.step-index {
  flex: none;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--surface-soft);
  font-size: 12px;
  font-weight: 650;
  transition: all 0.2s ease;
}

.active .step-index {
  background: var(--primary);
  color: #fff;
  box-shadow: 0 0 0 4px var(--primary-soft);
}

.done .step-index {
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 18px;
}

.step-title {
  font-size: 14px;
  font-weight: 600;
  line-height: 26px;
}

.step-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.safety {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 28px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  color: var(--text-muted);
  font-size: 12px;
}

.safety .n-icon {
  color: var(--primary);
  font-size: 16px;
}

.qr-side {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px 24px;
  background: linear-gradient(180deg, var(--primary-faint), var(--surface-soft));
}

.qr-box {
  position: relative;
  width: 224px;
  height: 224px;
  padding: 12px;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 10px 30px -12px rgba(37, 99, 235, 0.35), 0 0 0 1px rgba(15, 23, 42, 0.04);
}

.qr-img {
  display: block;
  width: 200px;
  height: 200px;
  transition: filter 0.2s ease, opacity 0.2s ease;
}

.qr-box.scanned .qr-img,
.qr-box.confirmed .qr-img,
.qr-box.expired .qr-img,
.qr-box.cancelled .qr-img,
.qr-box.failed .qr-img {
  filter: blur(3px);
  opacity: 0.35;
}

.qr-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 16px;
  border: none;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
  color: #172033;
  text-align: center;
  font: inherit;
}

.qr-overlay.plain {
  gap: 10px;
  color: #6b7789;
  font-size: 12px;
  background: #fff;
}

.qr-overlay.clickable {
  cursor: pointer;
  color: #2563eb;
}

.qr-overlay.clickable:hover .overlay-title {
  text-decoration: underline;
}

.qr-overlay .ok {
  color: #2563eb;
}

.qr-overlay .bad {
  color: #dc2626;
}

.overlay-title {
  margin-top: 6px;
  font-size: 15px;
  font-weight: 650;
}

.overlay-desc {
  font-size: 12px;
  color: #6b7789;
  line-height: 1.6;
}

.qr-caption {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  color: var(--text-muted);
  font-size: 12.5px;
}

.qr-caption .n-icon {
  color: var(--primary);
}

.qr-refresh {
  margin-top: 6px;
  color: var(--text-muted);
}

/* ---- connected ---- */
.profile-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  flex-wrap: wrap;
}

.profile-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.profile-avatar {
  position: relative;
}

.profile-dot {
  position: absolute;
  right: 0;
  bottom: 2px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--primary);
  box-shadow: 0 0 0 2px var(--surface);
}

.profile-name {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.profile-sub {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 5px;
  color: var(--text-muted);
  font-size: 12px;
}

.profile-actions {
  display: flex;
  gap: 8px;
}

.count-pill {
  flex: none;
  min-width: 28px;
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--surface-soft);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
  text-align: center;
}

.switch-hint {
  margin-bottom: 4px;
}

.account-group .section-label {
  margin-top: 14px;
}

.account-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.account-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  transition: background 0.15s ease;
}

.account-row:hover {
  background: var(--surface-hover);
}

.account-text {
  flex: 1;
  min-width: 0;
}

.account-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13.5px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.account-id {
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 11px;
}

.account-side {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

@media (max-width: 900px) {
  .login-card {
    grid-template-columns: 1fr;
  }

  .qr-side {
    order: -1;
  }

  .account-list {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 600px) {
  .login-intro {
    padding: 22px 18px;
  }

  .profile-actions {
    width: 100%;
  }
}
</style>

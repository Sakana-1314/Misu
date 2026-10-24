<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlertCircleOutline, ArrowForwardOutline, CheckmarkOutline } from '@vicons/ionicons5'
import { api, ApiError } from '@/api/client'
import type { Authorizer } from '@/types/api'
import AppAvatar from '@/components/AppAvatar.vue'

const route = useRoute()
const router = useRouter()
const status = ref<'loading' | 'success' | 'error'>('loading')
const errorText = ref('')
const app = ref<Authorizer | null>(null)

onMounted(async () => {
  const authCode = typeof route.query.auth_code === 'string' ? route.query.auth_code : ''
  const state = typeof route.query.state === 'string' ? route.query.state : ''
  if (!authCode) {
    status.value = 'error'
    errorText.value = '回调地址缺少 auth_code，可能是授权已过期或链接被重复打开。'
    return
  }
  try {
    app.value = await api.post<Authorizer>('/authorizers/exchange', { auth_code: authCode, state }, { silent: true })
    status.value = 'success'
  } catch (error) {
    status.value = 'error'
    errorText.value = error instanceof ApiError ? error.message : '授权处理失败，请重试。'
  }
})

const active = computed(() => app.value?.status === 'active')

function goList() {
  router.push('/authorizers')
}

function goDetail() {
  if (app.value) router.push(`/authorizers/${app.value.appid}`)
}
</script>

<template>
  <div class="callback-page">
    <div class="card">
      <div class="brand">
        <img src="/logo.svg?v=2" alt="logo" class="logo" />
        <span>御坂微枢</span>
      </div>

      <div v-if="status === 'loading'" class="state">
        <n-spin :size="26" />
        <p class="state-text">正在处理授权结果…</p>
      </div>

      <template v-else-if="status === 'success' && app">
        <div class="badge success">
          <n-icon :size="22"><CheckmarkOutline /></n-icon>
        </div>
        <h1>授权成功</h1>
        <p class="hint">已加入你的小程序列表</p>

        <div class="app-row">
          <AppAvatar :src="app.head_img" :name="app.nick_name" :appid="app.appid" :size="44" />
          <div class="app-meta">
            <div class="app-name">{{ app.nick_name || '未命名小程序' }}</div>
            <div class="app-id mono">{{ app.appid }}</div>
          </div>
          <n-tag size="small" :bordered="false" :type="active ? 'primary' : 'default'">
            {{ active ? '已授权' : '已取消' }}
          </n-tag>
        </div>

        <dl class="facts">
          <div>
            <dt>主体</dt>
            <dd>{{ app.principal_name || '—' }}</dd>
          </div>
          <div>
            <dt>认证</dt>
            <dd>{{ app.verify_type_id === 0 ? '微信认证' : '未认证' }}</dd>
          </div>
        </dl>

        <div class="actions">
          <n-button type="primary" @click="goDetail">
            管理
            <template #icon><n-icon><ArrowForwardOutline /></n-icon></template>
          </n-button>
          <n-button @click="goList">列表</n-button>
        </div>
      </template>

      <template v-else>
        <div class="badge error">
          <n-icon :size="22"><AlertCircleOutline /></n-icon>
        </div>
        <h1>授权失败</h1>
        <p class="hint">{{ errorText }}</p>
        <div class="actions">
          <n-button type="primary" @click="router.push('/authorizers?add=1')">重试</n-button>
          <n-button @click="goList">列表</n-button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.callback-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--page-bg);
}

.card {
  width: min(420px, 100%);
  padding: 26px 26px 22px;
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), 0 18px 48px rgba(15, 23, 42, 0.07);
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 620;
  color: var(--text-muted);
}

.logo {
  width: 22px;
  height: 22px;
  border-radius: 6px;
}

.state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 44px 0 30px;
}

.state-text {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.badge {
  width: 48px;
  height: 48px;
  margin-top: 24px;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.badge.success {
  background: var(--primary-soft);
  color: var(--primary);
}

.badge.error {
  background: rgba(239, 68, 68, 0.09);
  color: #dc2626;
}

h1 {
  margin: 14px 0 4px;
  font-size: 20px;
  font-weight: 680;
  letter-spacing: -0.02em;
}

.hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.7;
  word-break: break-word;
}

.app-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.app-meta {
  flex: 1;
  min-width: 0;
}

.app-name {
  font-size: 14px;
  font-weight: 620;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-id {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-muted);
  word-break: break-all;
}

.facts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1px;
  margin: 12px 0 0;
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--border);
}

.facts > div {
  padding: 10px 14px;
  background: var(--surface-soft);
}

.facts dt {
  font-size: 11px;
  color: var(--text-muted);
}

.facts dd {
  margin: 4px 0 0;
  font-size: 13px;
  font-weight: 560;
  word-break: break-all;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 20px;
}

@media (max-width: 480px) {
  .card {
    padding: 22px 18px 18px;
  }

  .facts {
    grid-template-columns: 1fr;
  }
}
</style>

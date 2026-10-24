<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlertCircleOutline, CheckmarkCircleOutline, ArrowBackOutline } from '@vicons/ionicons5'
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
    errorText.value = error instanceof ApiError ? error.message : '授权处理失败，请重新发起授权。'
  }
})

function goList() {
  router.push('/authorizers')
}

function goHome() {
  router.push('/')
}
</script>

<template>
  <div class="callback-page">
    <div class="callback-card surface-card">
      <img src="/logo.svg?v=2" alt="logo" class="logo" />
      <template v-if="status === 'loading'">
        <n-spin size="large" style="margin: 18px 0" />
        <h1>正在处理授权结果</h1>

      </template>

      <template v-else-if="status === 'success' && app">
        <div class="result-icon success"><n-icon :size="32"><CheckmarkCircleOutline /></n-icon></div>
        <h1>小程序授权成功</h1>

        <div class="app-box">
          <AppAvatar :src="app.head_img" :name="app.nick_name" :appid="app.appid" :size="52" />
          <div class="app-info">
            <div class="app-name">{{ app.nick_name || '未命名小程序' }}</div>
            <div class="app-id mono">{{ app.appid }}</div>
            <div class="app-tags">
              <n-tag size="small" :bordered="false" :type="app.status === 'active' ? 'primary' : 'default'">
                {{ app.status === 'active' ? '已授权' : '已取消授权' }}
              </n-tag>
              <n-tag size="small" :bordered="false">{{ app.principal_name || '主体未知' }}</n-tag>
            </div>
          </div>
        </div>
        <div class="actions">
          <n-button type="primary" @click="goList">查看小程序列表</n-button>
          <n-button quaternary @click="goHome">返回首页</n-button>
        </div>
      </template>

      <template v-else>
        <div class="result-icon error"><n-icon :size="32"><AlertCircleOutline /></n-icon></div>
        <h1>授权处理失败</h1>
        <p class="muted">{{ errorText }}</p>
        <div class="actions">
          <n-button type="primary" @click="router.push('/authorizers?add=1')">重新发起授权</n-button>
          <n-button quaternary @click="goHome">
            <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
            返回首页
          </n-button>
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
  background:
    radial-gradient(700px 500px at 15% 0%, rgba(37, 99, 235, 0.12), transparent 60%),
    radial-gradient(700px 500px at 100% 100%, rgba(29, 78, 216, 0.08), transparent 60%),
    var(--page-bg);
}

.callback-card {
  width: min(520px, 100%);
  padding: 40px 38px 34px;
  text-align: center;
}

.logo {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  box-shadow: 0 8px 22px rgba(7, 193, 96, 0.2);
}

h1 {
  margin: 18px 0 8px;
  font-size: 22px;
  letter-spacing: -0.02em;
}

.muted {
  margin: 0 auto;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.7;
  max-width: 380px;
}

.result-icon {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-top: 22px;
}

.result-icon.success {
  background: rgba(37, 99, 235, 0.12);
  color: var(--primary);
}

.result-icon.error {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
}

.app-box {
  display: flex;
  align-items: center;
  gap: 14px;
  text-align: left;
  padding: 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  margin: 22px 0;
}

.app-info {
  min-width: 0;
}

.app-name {
  font-size: 15px;
  font-weight: 650;
}

.app-id {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 3px;
  word-break: break-all;
}

.app-tags {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 22px;
}

.login-hint {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 26px;
  color: var(--text-muted);
  font-size: 12px;
}

@media (max-width: 520px) {
  .callback-card {
    padding: 30px 22px;
  }
}
</style>

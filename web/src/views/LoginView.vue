<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const form = reactive({ username: '', password: '' })
const loading = ref(false)

async function submit() {
  if (loading.value) return
  if (!form.username.trim() || !form.password) {
    message.warning('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(form.username.trim(), form.password)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect.startsWith('/') ? redirect : '/')
  } catch {
    // The API interceptor shows the error.
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <img src="/logo.svg?v=2" alt="御坂微枢" class="logo" />
        <div class="name">御坂微枢</div>
      </div>
      <n-form label-placement="top" :show-feedback="false" @submit.prevent="submit">
        <n-form-item label="账号">
          <n-input v-model:value="form.username" placeholder="请输入账号" autocomplete="username" />
        </n-form-item>
        <n-form-item label="密码" style="margin-top: 14px">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            placeholder="请输入密码"
            autocomplete="current-password"
          />
        </n-form-item>
        <n-button type="primary" block class="block-button" attr-type="submit" :loading="loading" style="margin-top: 22px">登录</n-button>
      </n-form>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background:
    radial-gradient(700px 500px at 15% 0%, rgba(37, 99, 235, 0.22), transparent 60%),
    radial-gradient(700px 500px at 100% 100%, rgba(29, 78, 216, 0.18), transparent 60%),
    linear-gradient(140deg, #0b1730 0%, #10265a 55%, #1d4ed8 100%);
}

.login-card {
  width: min(380px, 100%);
  padding: 34px 32px 30px;
  border-radius: 18px;
  background: var(--surface);
  border: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow: 0 24px 70px rgba(2, 8, 23, 0.35);
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 28px;
}

.logo {
  width: 38px;
  height: 38px;
  border-radius: 11px;
}

.name {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
}
</style>

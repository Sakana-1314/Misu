<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="notfound">
    <div class="card surface-card">
      <img src="/logo.svg?v=2" alt="logo" class="logo" />
      <div class="code">404</div>
      <h1>页面不存在</h1>
      <div class="actions">
        <n-button type="primary" @click="router.push('/')">返回仪表盘</n-button>
        <n-button quaternary @click="router.back()">返回上一页</n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.notfound {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--page-bg);
  padding: 24px;
}

.card {
  width: min(440px, 100%);
  padding: 44px 36px;
  text-align: center;
}

.logo {
  width: 46px;
  height: 46px;
  border-radius: 13px;
}

.code {
  margin-top: 26px;
  font-size: 72px;
  font-weight: 800;
  letter-spacing: -0.06em;
  line-height: 1;
  color: var(--primary);
  opacity: 0.9;
}

h1 {
  margin: 14px 0 8px;
  font-size: 20px;
}

p {
  margin: 0 0 24px;
  color: var(--text-muted);
  font-size: 13px;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 10px;
}
</style>

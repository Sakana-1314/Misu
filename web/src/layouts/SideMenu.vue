<script setup lang="ts">
import { h, type Component } from 'vue'
import { NIcon } from 'naive-ui'
import {
  AppsOutline,
  ClipboardOutline,
  CodeSlashOutline,
  DocumentTextOutline,
  GridOutline,
} from '@vicons/ionicons5'

defineProps<{ activeKey: string }>()
const emit = defineEmits<{ (e: 'select', key: string): void }>()

const renderIcon = (icon: Component) => () => h(NIcon, null, { default: () => h(icon) })

const menuOptions = [
  { label: '仪表盘', key: 'dashboard', icon: renderIcon(GridOutline) },
  { label: '小程序', key: 'authorizers', icon: renderIcon(AppsOutline) },
  { label: '代码模板', key: 'templates', icon: renderIcon(CodeSlashOutline) },
  { label: '审核配置', key: 'audit-profiles', icon: renderIcon(ClipboardOutline) },
  { label: '操作日志', key: 'logs', icon: renderIcon(DocumentTextOutline) },
]
</script>

<template>
  <div class="side-menu">
    <div class="brand">
      <img src="/logo.svg?v=2" alt="logo" class="logo" />
      <div class="brand-name">御坂微枢</div>
    </div>
    <div class="nav">
      <n-menu
        :value="activeKey"
        :options="menuOptions"
        :indent="18"
        :root-indent="14"
        @update:value="(key: string) => emit('select', key)"
      />
    </div>
  </div>
</template>

<style scoped>
.side-menu {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 20px 14px 16px;
  background: var(--surface);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 2px 8px 24px;
}

.logo {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.24);
}

.brand-name {
  font-weight: 700;
  font-size: 15px;
  letter-spacing: -0.01em;
  white-space: nowrap;
}

.nav {
  flex: 1;
  overflow-y: auto;
}
</style>

<script setup lang="ts">
import { h, type Component } from 'vue'
import { NIcon } from 'naive-ui'
import { AppsOutline, CodeSlashOutline, DocumentTextOutline, GridOutline, LinkOutline } from '@vicons/ionicons5'

defineProps<{ activeKey: string; collapsed?: boolean }>()
const emit = defineEmits<{ (e: 'select', key: string): void }>()

const renderIcon = (icon: Component) => () => h(NIcon, null, { default: () => h(icon) })

const menuOptions = [
  { label: '仪表盘', key: 'dashboard', icon: renderIcon(GridOutline) },
  { label: '小程序', key: 'authorizers', icon: renderIcon(AppsOutline) },
  { label: '代码模板', key: 'templates', icon: renderIcon(CodeSlashOutline) },
  { label: '网页版连接', key: 'connect', icon: renderIcon(LinkOutline) },
  { label: '操作日志', key: 'logs', icon: renderIcon(DocumentTextOutline) },
]
</script>

<template>
  <div class="side-menu" :class="{ collapsed }">
    <div class="brand">
      <img src="/logo.svg?v=2" alt="logo" class="logo" />
      <div v-if="!collapsed" class="brand-name">御坂微枢</div>
    </div>
    <div class="nav">
      <n-menu
        :value="activeKey"
        :options="menuOptions"
        :collapsed="collapsed"
        :collapsed-width="56"
        :collapsed-icon-size="20"
        :indent="16"
        :root-indent="12"
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
  padding: 14px 10px 12px;
  background: var(--surface);
}

.side-menu.collapsed {
  padding-left: 6px;
  padding-right: 6px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 2px 6px 16px;
}

.collapsed .brand {
  justify-content: center;
  padding-left: 0;
  padding-right: 0;
}

.logo {
  width: 30px;
  height: 30px;
  border-radius: var(--radius-sm);
  flex: none;
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
  overflow-x: hidden;
}

:deep(.n-menu-item-content) {
  border-radius: var(--radius-sm);
}

:deep(.n-menu-item-content--selected) {
  background: var(--primary-soft);
}

:deep(.n-menu-item-content--selected .n-menu-item-content-header) {
  color: var(--primary);
  font-weight: 620;
}
</style>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NIcon, useDialog } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import {
  ChevronDownOutline,
  LogOutOutline,
  MenuOutline,
  MoonOutline,
  ChevronBackOutline,
  ChevronForwardOutline,
  SunnyOutline,
} from '@vicons/ionicons5'
import SideMenu from './SideMenu.vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const router = useRouter()
const dialog = useDialog()
const auth = useAuthStore()
const themeStore = useThemeStore()

const mobileOpen = ref(false)
const isMobile = ref(window.innerWidth < 980)
const collapsed = ref(localStorage.getItem('misu.sidebar.collapsed') === '1')
const sidebarWidth = computed(() => (collapsed.value ? 64 : 208))

const activeKey = computed(() => {
  const name = String(route.name || '')
  if (name === 'authorizer-detail') return 'authorizers'
  return name
})

const pageTitle = computed(() => (route.meta.title as string) || '控制台')
const username = computed(() => auth.state.admin?.username || 'admin')

const userOptions: DropdownOption[] = [
  { key: 'logout', label: '退出登录', icon: () => h(NIcon, null, { default: () => h(LogOutOutline) }) },
]

function toggleCollapsed() {
  collapsed.value = !collapsed.value
  localStorage.setItem('misu.sidebar.collapsed', collapsed.value ? '1' : '0')
}

function handleMenuSelect(key: string) {
  mobileOpen.value = false
  if (key !== route.name) {
    router.push({ name: key })
  }
}

function handleUserSelect(key: string) {
  if (key !== 'logout') return
  dialog.warning({
    title: '退出登录',
    content: '确定退出当前账号？',
    positiveText: '退出',
    negativeText: '取消',
    onPositiveClick: async () => {
      await auth.logout()
      router.replace('/login')
    },
  })
}

function onResize() {
  isMobile.value = window.innerWidth < 980
  if (!isMobile.value) mobileOpen.value = false
}

onMounted(() => {
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
})
</script>

<template>
  <div class="layout">
    <aside v-if="!isMobile" class="sidebar" :style="{ width: `${sidebarWidth}px` }">
      <SideMenu :active-key="activeKey" :collapsed="collapsed" @select="handleMenuSelect" />
    </aside>

    <n-drawer v-model:show="mobileOpen" placement="left" :width="220">
      <n-drawer-content body-content-style="padding:0">
        <SideMenu :active-key="activeKey" @select="handleMenuSelect" />
      </n-drawer-content>
    </n-drawer>

    <div class="main" :class="{ 'is-mobile': isMobile }" :style="{ marginLeft: isMobile ? '0' : `${sidebarWidth}px` }">
      <header class="topbar">
        <div class="left">
          <n-button v-if="isMobile" circle class="icon-button" @click="mobileOpen = true">
            <template #icon><n-icon><MenuOutline /></n-icon></template>
          </n-button>
          <n-button v-else circle class="icon-button" :title="collapsed ? '展开导航' : '收起导航'" @click="toggleCollapsed">
            <template #icon>
              <n-icon>
                <ChevronForwardOutline v-if="collapsed" />
                <ChevronBackOutline v-else />
              </n-icon>
            </template>
          </n-button>
          <div class="title">{{ pageTitle }}</div>
        </div>
        <div class="right">
          <n-button circle class="icon-button" :title="themeStore.state.dark ? '切换浅色' : '切换深色'" @click="themeStore.toggle()">
            <template #icon>
              <n-icon>
                <SunnyOutline v-if="themeStore.state.dark" />
                <MoonOutline v-else />
              </n-icon>
            </template>
          </n-button>
          <n-dropdown :options="userOptions" trigger="click" placement="bottom-end" @select="handleUserSelect">
            <button class="account-button" type="button">
              <n-avatar round :size="26" color="#2563EB">{{ username.slice(0, 1).toUpperCase() }}</n-avatar>
              <span class="account-name">{{ username }}</span>
              <n-icon class="account-chevron"><ChevronDownOutline /></n-icon>
            </button>
          </n-dropdown>
        </div>
      </header>

      <main class="content">
        <div class="page-container">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout {
  min-height: 100vh;
}

.sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 20;
  border-right: 1px solid var(--border);
  transition: width 0.18s ease;
}

.main {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  transition: margin-left 0.18s ease;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 15;
  height: 54px;
  padding: 0 18px 0 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.title {
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
}

.account-button {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 3px 8px 3px 3px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: background 0.15s ease;
}

.account-button:hover {
  background: var(--surface-soft);
}

.account-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.account-name {
  font-size: 13px;
  font-weight: 560;
  max-width: 110px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-chevron {
  color: var(--text-muted);
  font-size: 13px;
}

.content {
  padding: 16px 18px 24px;
  flex: 1;
}

@media (max-width: 768px) {
  .topbar {
    padding: 0 12px;
    height: 52px;
  }

  .content {
    padding: 12px;
  }

  .account-name,
  .account-chevron {
    display: none;
  }

  .account-button {
    padding: 3px;
  }
}
</style>

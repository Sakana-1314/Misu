<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDialog } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import { ChevronDownOutline, LogOutOutline, MenuOutline, MoonOutline, SunnyOutline } from '@vicons/ionicons5'
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

const activeKey = computed(() => {
  const name = String(route.name || '')
  if (name === 'authorizer-detail') return 'authorizers'
  return name
})

const pageTitle = computed(() => (route.meta.title as string) || '控制台')
const username = computed(() => auth.state.admin?.username || 'admin')

const userOptions: DropdownOption[] = [
  { label: username.value, key: 'name', disabled: true },
  { type: 'divider', key: 'divider' },
  { label: '退出登录', key: 'logout', icon: () => h(LogOutOutline) },
]

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
    <aside v-if="!isMobile" class="sidebar">
      <SideMenu :active-key="activeKey" @select="handleMenuSelect" />
    </aside>

    <n-drawer v-model:show="mobileOpen" placement="left" :width="280">
      <n-drawer-content body-content-style="padding:0">
        <SideMenu :active-key="activeKey" @select="handleMenuSelect" />
      </n-drawer-content>
    </n-drawer>

    <div class="main" :class="{ 'is-mobile': isMobile }">
      <header class="topbar">
        <div class="left">
          <n-button v-if="isMobile" circle class="icon-button" @click="mobileOpen = true">
            <template #icon><n-icon><MenuOutline /></n-icon></template>
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
          <n-dropdown :options="userOptions" trigger="click" @select="handleUserSelect">
            <button class="user-button">
              <n-avatar round :size="28" color="#2563EB">
                {{ username.slice(0, 1).toUpperCase() }}
              </n-avatar>
              <span class="username">{{ username }}</span>
              <n-icon class="chevron"><ChevronDownOutline /></n-icon>
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
  width: 236px;
  border-right: 1px solid var(--border);
  z-index: 20;
}

.main {
  margin-left: 236px;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.main.is-mobile {
  margin-left: 0;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 15;
  height: 64px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  backdrop-filter: blur(14px);
  border-bottom: 1px solid var(--border);
}

.left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.title {
  font-size: 17px;
  font-weight: 650;
  letter-spacing: -0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

.user-button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px 4px 4px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  cursor: pointer;
  color: var(--text);
}

.user-button:hover {
  background: var(--surface-soft);
}

.username {
  font-size: 13px;
  font-weight: 560;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chevron {
  color: var(--text-muted);
  font-size: 14px;
}

.content {
  padding: 22px;
  flex: 1;
}

@media (max-width: 768px) {
  .topbar {
    padding: 0 16px;
    height: 60px;
  }

  .content {
    padding: 16px;
  }

  .username,
  .chevron {
    display: none;
  }

  .user-button {
    padding: 3px;
  }
}
</style>

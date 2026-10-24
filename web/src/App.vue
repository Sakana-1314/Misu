<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { darkTheme, dateZhCN, zhCN, type GlobalThemeOverrides } from 'naive-ui'
import AppRoot from '@/AppRoot.vue'
import { useThemeStore } from '@/stores/theme'

const themeStore = useThemeStore()
const activeTheme = computed(() => (themeStore.state.dark ? darkTheme : null))

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#2563EB',
    primaryColorHover: '#3B82F6',
    primaryColorPressed: '#1D4ED8',
    primaryColorSuppl: '#2563EB',
    infoColor: '#2563EB',
    successColor: '#2563EB',
    warningColor: '#F59E0B',
    errorColor: '#EF4444',
    borderRadius: '8px',
    borderRadiusSmall: '6px',
    fontFamily:
      'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  },
  Card: { borderRadius: '10px' },
  Button: { borderRadiusMedium: '7px', borderRadiusSmall: '6px', borderRadiusTiny: '5px' },
  Input: { borderRadius: '7px' },
  Tag: { borderRadius: '5px' },
  DataTable: { borderRadius: '8px' },
  Dropdown: { borderRadius: '8px' },
  Modal: { borderRadius: '10px' },
}

watchEffect(() => {
  document.documentElement.classList.toggle('dark', themeStore.state.dark)
})
</script>

<template>
  <n-config-provider :theme="activeTheme" :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <n-global-style />
    <n-loading-bar-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <n-message-provider placement="bottom-right" :max="4">
            <AppRoot />
          </n-message-provider>
        </n-notification-provider>
      </n-dialog-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>

import { reactive, watch } from 'vue'

const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('misu-theme') : null
const state = reactive({ dark: stored === 'dark' })

watch(
  () => state.dark,
  (dark) => {
    localStorage.setItem('misu-theme', dark ? 'dark' : 'light')
  },
)

export function useThemeStore() {
  return {
    state,
    toggle() {
      state.dark = !state.dark
    },
  }
}

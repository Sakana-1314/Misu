import { computed, reactive } from 'vue'
import { api } from '@/api/client'
import type { AdminInfo } from '@/types/api'

const state = reactive<{
  admin: AdminInfo | null
  loaded: boolean
  loading: boolean
}>({
  admin: null,
  loaded: false,
  loading: false,
})

const isAuthenticated = computed(() => !!state.admin)

async function fetchMe(force = false): Promise<boolean> {
  if (state.loaded && !force) return isAuthenticated.value
  state.loading = true
  try {
    const data = await api.get<AdminInfo>('/auth/me', { silent: true })
    state.admin = data
    state.loaded = true
    return true
  } catch {
    state.admin = null
    state.loaded = true
    return false
  } finally {
    state.loading = false
  }
}

async function login(username: string, password: string): Promise<AdminInfo> {
  const data = await api.post<AdminInfo>('/auth/login', { username, password })
  state.admin = data
  state.loaded = true
  return data
}

async function logout() {
  try {
    await api.post('/auth/logout', null, { silent: true })
  } catch {
    // ignore
  } finally {
    state.admin = null
    state.loaded = true
  }
}

export function useAuthStore() {
  return { state, isAuthenticated, fetchMe, login, logout }
}

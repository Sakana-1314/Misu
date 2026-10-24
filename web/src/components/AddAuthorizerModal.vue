<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '@/api/client'
import type { PreauthResult, SyncResult } from '@/types/api'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'synced'): void
}>()

const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value),
})

const message = useMessage()
const appid = ref('')
const loading = ref(false)
const syncing = ref(false)
const error = ref('')
const generated = ref<PreauthResult | null>(null)
const appidPattern = /^wx[0-9a-f]{16}$/

const validAppID = computed(() => appidPattern.test(appid.value.trim().toLowerCase()))

function reset() {
  appid.value = ''
  error.value = ''
  generated.value = null
  loading.value = false
  syncing.value = false
}

watch(
  () => props.show,
  (show) => {
    if (show) reset()
  },
)

async function startAuth() {
  const value = appid.value.trim().toLowerCase()
  if (!validAppID.value) {
    error.value = '请输入正确的小程序 AppID'
    return
  }
  loading.value = true
  error.value = ''
  try {
    generated.value = await api.post<PreauthResult>('/authorizers/preauth', { appid: value }, { silent: true })
    window.location.assign(generated.value.auth_url)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '生成授权链接失败'
    loading.value = false
  }
}

async function sync() {
  syncing.value = true
  try {
    const res = await api.post<SyncResult>('/authorizers/sync', null, { timeout: 300000, silent: true })
    message.success(`已同步 ${res.total} 个小程序`)
    emit('synced')
    visible.value = false
  } catch (err) {
    message.error(err instanceof Error ? err.message : '同步失败')
  } finally {
    syncing.value = false
  }
}
</script>

<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    title="新增小程序"
    :style="{ width: 'min(460px, 94vw)' }"
    :mask-closable="!loading && !syncing"
    :closable="!loading && !syncing"
  >
    <n-form label-placement="top" @submit.prevent="startAuth">
      <n-form-item label="小程序 AppID">
        <n-input
          v-model:value="appid"
          placeholder="wx1234567890abcdef"
          maxlength="18"
          :disabled="loading"
          @keyup.enter="startAuth"
        />
      </n-form-item>
      <n-alert v-if="error" type="error" :bordered="false">{{ error }}</n-alert>
      <n-alert v-else type="info" :bordered="false">
        将跳转微信授权页，确认授权后自动返回。
      </n-alert>
    </n-form>
    <template #footer>
      <div class="modal-footer">
        <n-button :disabled="loading || syncing" @click="sync">同步</n-button>
        <n-button type="primary" :loading="loading" :disabled="!validAppID" @click="startAuth">授权</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  width: 100%;
}
</style>

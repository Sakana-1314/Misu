<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '@/api/client'
import type { PreauthResult, SyncResult } from '@/types/api'
import QrCodeView from './QrCodeView.vue'

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
const loading = ref(false)
const syncing = ref(false)
const result = ref<PreauthResult | null>(null)
const error = ref('')
const popupBlocked = ref(false)

const bridgeUrl = computed(() => {
  if (!result.value) return ''
  const url = new URL('/auth-open.html', window.location.origin)
  url.searchParams.set('url', result.value.auth_url)
  return url.toString()
})

async function generate() {
  loading.value = true
  error.value = ''
  popupBlocked.value = false
  try {
    result.value = await api.post<PreauthResult>('/authorizers/preauth', null, { silent: true })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '生成授权链接失败'
  } finally {
    loading.value = false
  }
}

function openAuthPage() {
  if (!bridgeUrl.value) return
  const popup = window.open(
    bridgeUrl.value,
    'wechat_authorization',
    'popup=yes,width=520,height=760,menubar=no,toolbar=no,location=yes,status=no,resizable=yes,scrollbars=yes',
  )
  if (!popup) {
    popupBlocked.value = true
    return
  }
  try {
    popup.opener = null
  } catch {
    // ignore
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

watch(
  () => props.show,
  (show) => {
    if (!show) return
    result.value = null
    error.value = ''
    popupBlocked.value = false
    void generate()
  },
  { immediate: true },
)
</script>

<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    title="新增小程序"
    :style="{ width: 'min(500px, 94vw)' }"
    :mask-closable="!syncing"
    :closable="!syncing"
  >
    <div v-if="loading" class="center"><n-spin /></div>
    <n-alert v-else-if="error" type="error" :bordered="false">{{ error }}</n-alert>
    <div v-else-if="result" class="add-body">
      <QrCodeView :value="result.auth_url" :size="196" alt="微信授权二维码" />
      <n-alert v-if="popupBlocked" type="warning" :bordered="false">
        浏览器拦截了弹窗，
        <a :href="bridgeUrl" target="_blank" rel="noopener">直接打开授权页</a>
      </n-alert>
    </div>
    <template #footer>
      <div class="modal-footer">
        <n-button :disabled="syncing" @click="visible = false">关闭</n-button>
        <n-button :disabled="loading" @click="generate">重试</n-button>
        <n-button :loading="syncing" @click="sync">同步</n-button>
        <n-button type="primary" :disabled="!result" @click="openAuthPage">授权</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.center {
  padding: 40px 0;
  text-align: center;
}

.add-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
}
</style>

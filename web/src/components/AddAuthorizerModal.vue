<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { LinkOutline, OpenOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { PreauthResult, SyncResult } from '@/types/api'
import { errorMessage } from '@/utils/error'
import CopyButton from '@/components/CopyButton.vue'

const props = withDefaults(defineProps<{ show: boolean; initialAppid?: string }>(), { initialAppid: '' })
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
const loading = ref<'' | 'go' | 'link'>('')
const syncing = ref(false)
const error = ref('')
const generated = ref<PreauthResult | null>(null)
const appidPattern = /^wx[0-9a-f]{16}$/

const normalized = computed(() => appid.value.trim().toLowerCase())
const validAppID = computed(() => appidPattern.test(normalized.value))
const showFormatHint = computed(() => normalized.value.length >= 4 && !validAppID.value)
const busy = computed(() => loading.value !== '' || syncing.value)

watch(
  () => props.show,
  (show) => {
    if (!show) return
    appid.value = props.initialAppid || ''
    error.value = ''
    generated.value = null
    loading.value = ''
  },
)

async function createLink(mode: 'go' | 'link') {
  if (!validAppID.value) {
    error.value = '请输入正确的 AppID，例如 wx1234567890abcdef'
    return
  }
  loading.value = mode
  error.value = ''
  try {
    generated.value = await api.post<PreauthResult>('/authorizers/preauth', { appid: normalized.value }, { silent: true })
    if (mode === 'go') {
      window.location.assign(generated.value.auth_url)
      return
    }
  } catch (err) {
    error.value = errorMessage(err)
  }
  loading.value = ''
}

async function sync() {
  syncing.value = true
  try {
    const res = await api.post<SyncResult>('/authorizers/sync', null, { timeout: 300000, silent: true })
    message.success(res.added > 0 ? `发现 ${res.added} 个新接入的小程序` : `已同步，共 ${res.total} 个小程序`)
    emit('synced')
    visible.value = false
  } catch (err) {
    message.error(errorMessage(err))
  } finally {
    syncing.value = false
  }
}
</script>

<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    title="接入小程序"
    :style="{ width: 'min(500px, 94vw)' }"
    :mask-closable="!busy"
    :closable="!busy"
  >
    <div class="intro">
      接入后即可在这里上传代码、提交审核和发布。需要小程序管理员在微信中确认授权。
    </div>

    <ol class="flow">
      <li><span>1</span>填写小程序 AppID</li>
      <li><span>2</span>管理员扫码确认授权</li>
      <li><span>3</span>自动返回并完成接入</li>
    </ol>

    <n-form-item
      label="小程序 AppID"
      :show-feedback="true"
      :validation-status="error || showFormatHint ? 'error' : undefined"
      :feedback="error || (showFormatHint ? 'AppID 以 wx 开头，共 18 位' : '可在小程序后台「设置 → 基本设置」中找到')"
    >
      <n-input
        v-model:value="appid"
        placeholder="wx1234567890abcdef"
        maxlength="18"
        size="large"
        :disabled="busy"
        @keyup.enter="createLink('go')"
      />
    </n-form-item>

    <div v-if="generated && !loading" class="link-box">
      <div class="link-title">
        <n-icon><LinkOutline /></n-icon>
        授权链接已生成
      </div>
      <div class="hint">把链接发给小程序管理员，在电脑浏览器中打开并用微信扫码确认即可。</div>
      <div class="link-row">
        <span class="mono link-text">{{ generated.auth_url }}</span>
        <CopyButton :text="generated.auth_url" label="复制链接" />
      </div>
    </div>

    <template #footer>
      <div class="footer">
        <n-tooltip>
          <template #trigger>
            <n-button text :loading="syncing" :disabled="loading !== ''" @click="sync">已在别处授权过？同步列表</n-button>
          </template>
          从微信开放平台拉取最新的已授权小程序
        </n-tooltip>
        <div class="footer-actions">
          <n-button :loading="loading === 'link'" :disabled="!validAppID || syncing || loading === 'go'" @click="createLink('link')">
            生成授权链接
          </n-button>
          <n-button type="primary" :loading="loading === 'go'" :disabled="!validAppID || syncing || loading === 'link'" @click="createLink('go')">
            <template #icon><n-icon><OpenOutline /></n-icon></template>
            前往授权
          </n-button>
        </div>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.intro {
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.7;
}

.flow {
  display: flex;
  gap: 8px;
  margin: 16px 0 18px;
  padding: 0;
  list-style: none;
}

.flow li {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  font-size: 12px;
  font-weight: 560;
}

.flow span {
  flex: none;
  width: 20px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 11px;
  font-weight: 700;
}

.link-box {
  margin-top: 4px;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--primary-faint);
  border: 1px solid var(--primary-soft);
}

.link-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 13px;
  font-weight: 620;
  color: var(--primary);
}

.link-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.link-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  color: var(--text-muted);
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.footer-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

@media (max-width: 520px) {
  .flow {
    flex-direction: column;
  }
}
</style>

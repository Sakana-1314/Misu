<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { AddOutline, DocumentOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'

interface MediaFile {
  id: number
  name: string
  mime: string
  size: number
}

const props = withDefaults(
  defineProps<{
    modelValue: number[]
    accept?: string
    max?: number
  }>(),
  {
    accept: 'image/*',
    max: 5,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: number[]): void
}>()

const message = useMessage()
const input = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const files = ref<MediaFile[]>([])

const full = computed(() => files.value.length >= props.max)

watch(
  () => props.modelValue,
  (ids) => {
    if (ids.length === 0) {
      files.value = []
      return
    }
    files.value = files.value.filter((file) => ids.includes(file.id))
  },
  { deep: true },
)

function isImage(file: MediaFile) {
  return (file.mime || '').startsWith('image/')
}

function assetURL(file: MediaFile) {
  return `/api/audit-assets/${file.id}`
}

function sizeText(bytes: number) {
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

function pick() {
  if (full.value) {
    message.warning(`最多上传 ${props.max} 个文件`)
    return
  }
  input.value?.click()
}

async function onSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const selected = Array.from(target.files || [])
  target.value = ''
  if (selected.length === 0) return
  if (files.value.length + selected.length > props.max) {
    message.warning(`最多上传 ${props.max} 个文件`)
    return
  }
  uploading.value = true
  try {
    for (const file of selected) {
      if (file.size > 10 << 20) {
        message.warning(`${file.name} 超过 10MB`)
        continue
      }
      const form = new FormData()
      form.append('file', file)
      const asset = await api.upload<{ id: number; filename: string; mime: string; size: number }>(
        '/audit-assets',
        form,
        { silent: true },
      )
      files.value.push({ id: asset.id, name: asset.filename || file.name, mime: asset.mime || file.type, size: asset.size || file.size })
    }
    emit('update:modelValue', files.value.map((file) => file.id))
  } catch (error) {
    message.error(error instanceof Error ? error.message : '上传失败')
  } finally {
    uploading.value = false
  }
}

function remove(id: number) {
  files.value = files.value.filter((file) => file.id !== id)
  emit('update:modelValue', files.value.map((file) => file.id))
}
</script>

<template>
  <div class="media-uploader">
    <div v-for="file in files" :key="file.id" class="media-tile">
      <img v-if="isImage(file)" :src="assetURL(file)" :alt="file.name" />
      <div v-else class="media-file">
        <n-icon :size="22"><DocumentOutline /></n-icon>
        <span class="media-name">{{ file.name }}</span>
        <span class="media-size">{{ sizeText(file.size) }}</span>
      </div>
      <button type="button" class="media-remove" title="移除" @click="remove(file.id)">
        <span aria-hidden="true">×</span>
      </button>
    </div>
    <button v-if="!full" type="button" class="media-add" :disabled="uploading" @click="pick">
      <n-spin v-if="uploading" :size="18" />
      <template v-else>
        <n-icon :size="20"><AddOutline /></n-icon>
        <span>上传</span>
      </template>
    </button>
    <input ref="input" type="file" :accept="accept" multiple hidden @change="onSelect" />
  </div>
</template>

<style scoped>
.media-uploader {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.media-tile {
  position: relative;
  width: 84px;
  height: 84px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  overflow: hidden;
}

.media-tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.media-file {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 8px;
  color: var(--text-muted);
}

.media-name {
  font-size: 11px;
  color: var(--text);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-size {
  font-size: 10px;
}

.media-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 50%;
  background: rgba(15, 23, 42, 0.62);
  color: #fff;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
}

.media-add {
  width: 84px;
  height: 84px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: var(--radius-sm);
  border: 1px dashed var(--border);
  background: var(--surface-soft);
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
}

.media-add:hover:not(:disabled) {
  border-color: var(--primary);
  color: var(--primary);
}

.media-add:disabled {
  cursor: default;
}
</style>

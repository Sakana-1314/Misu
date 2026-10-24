<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    src?: string | null
    name?: string | null
    appid?: string
    size?: number
  }>(),
  { src: '', name: '', appid: '', size: 40 },
)

const palette = ['#2563EB', '#3B82F6', '#1D4ED8', '#64748B', '#0EA5E9', '#475569', '#60A5FA', '#334155']

const color = computed(() => {
  const text = props.appid || props.name || 'wx'
  let hash = 0
  for (let i = 0; i < text.length; i += 1) hash = (hash * 31 + text.charCodeAt(i)) >>> 0
  return palette[hash % palette.length]
})

const fallback = computed(() => {
  const name = (props.name || '').trim()
  if (!name) return '小'
  return name.slice(0, 1)
})
</script>

<template>
  <n-avatar round :size="size" :src="src || undefined" :color="color" :style="{ fontSize: `${Math.max(12, size * 0.38)}px` }">
    {{ fallback }}
  </n-avatar>
</template>

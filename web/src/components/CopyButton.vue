<script setup lang="ts">
import { useMessage } from 'naive-ui'

const props = withDefaults(defineProps<{ text: string; label?: string }>(), {
  label: '复制',
})

const message = useMessage()

async function copy() {
  if (!props.text) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(props.text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = props.text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      textarea.remove()
    }
    message.success('已复制')
  } catch {
    message.error('复制失败')
  }
}
</script>

<template>
  <n-button quaternary type="primary" @click="copy">{{ label }}</n-button>
</template>

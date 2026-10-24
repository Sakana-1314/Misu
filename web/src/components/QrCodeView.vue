<script setup lang="ts">
import QRCode from 'qrcode'
import { ref, watch } from 'vue'

const props = withDefaults(defineProps<{ value: string; size?: number; alt?: string }>(), {
  size: 200,
  alt: '二维码',
})

const dataUrl = ref('')
const failed = ref(false)

async function render() {
  if (!props.value) {
    dataUrl.value = ''
    return
  }
  try {
    dataUrl.value = await QRCode.toDataURL(props.value, {
      width: props.size,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#111827', light: '#ffffff' },
    })
    failed.value = false
  } catch {
    failed.value = true
  }
}

watch(() => [props.value, props.size], render, { immediate: true })
</script>

<template>
  <div class="qr-frame">
    <img v-if="dataUrl && !failed" :src="dataUrl" :width="size" :height="size" :alt="alt" class="qr" />
    <div v-else class="qr-fallback" :style="{ width: `${size}px`, height: `${size}px` }">二维码生成失败</div>
  </div>
</template>

<style scoped>
.qr {
  display: block;
  border-radius: 6px;
}

.qr-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  font-size: 13px;
  background: #fff;
  border-radius: 6px;
}
</style>

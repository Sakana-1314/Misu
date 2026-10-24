<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { setErrorNotifier } from '@/api/client'

const message = useMessage()
setErrorNotifier((text: string) => message.error(text))
</script>

<template>
  <router-view />
</template>

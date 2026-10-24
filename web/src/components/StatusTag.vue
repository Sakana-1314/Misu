<script setup lang="ts">
import { computed } from 'vue'
import {
  accountStatusMeta,
  auditStatusMeta,
  authorizerStatusMeta,
  itemStatusMeta,
  jobStatusMeta,
  type NTagType,
} from '@/utils/constants'

const props = defineProps<{
  kind: 'job' | 'item' | 'audit' | 'authorizer' | 'account'
  value: string | number | null | undefined
}>()

const meta = computed<{ label: string; type: NTagType }>(() => {
  if (props.value === null || props.value === undefined) return { label: '—', type: 'default' }
  switch (props.kind) {
    case 'job':
      return jobStatusMeta[String(props.value)] || { label: String(props.value), type: 'default' }
    case 'item':
      return itemStatusMeta[String(props.value)] || { label: String(props.value), type: 'default' }
    case 'audit':
      return auditStatusMeta[Number(props.value)] || { label: `状态 ${props.value}`, type: 'default' }
    case 'authorizer':
      return authorizerStatusMeta[String(props.value)] || { label: String(props.value), type: 'default' }
    case 'account':
      return accountStatusMeta[Number(props.value)] || { label: `状态 ${props.value}`, type: 'default' }
    default:
      return { label: String(props.value), type: 'default' }
  }
})
</script>

<template>
  <n-tag size="small" :type="meta.type" :bordered="false">{{ meta.label }}</n-tag>
</template>

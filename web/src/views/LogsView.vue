<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { type DataTableColumns, type PaginationProps } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { OperationLog, Paged } from '@/types/api'
import { formatTime } from '@/utils/format'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import { operationActionLabels } from '@/utils/constants'

const items = ref<OperationLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

async function load(nextPage = page.value) {
  page.value = nextPage
  loading.value = true
  try {
    const data = await api.get<Paged<OperationLog>>('/logs', {
      params: { page: page.value, page_size: pageSize.value },
    })
    items.value = data.items || []
    total.value = data.total
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load(1)
})

const columns = computed<DataTableColumns<OperationLog>>(() => [
  { title: '时间', key: 'created_at', width: 180, render: (row) => formatTime(row.created_at) },
  {
    title: '操作',
    key: 'action',
    minWidth: 180,
    render: (row) => operationActionLabels[row.action] || row.action,
  },
  {
    title: '对象',
    key: 'target',
    minWidth: 220,
    render: (row) => h('span', { class: 'cell-muted' }, row.target || '—'),
  },
  { title: '操作人', key: 'actor', width: 120, render: (row) => h('span', { class: 'cell-muted' }, row.actor || '—') },
  { title: 'IP', key: 'ip', width: 140, render: (row) => h('span', { class: 'cell-muted mono' }, row.ip || '—') },
])

const pagination = computed<PaginationProps>(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  onUpdatePage: (value: number) => {
    void load(value)
  },
}))
</script>

<template>
  <div>
    <PageHeader title="操作日志" description="控制台中的每一次重要操作都会记录在这里，便于追溯。">
      <n-button :loading="loading" @click="load()">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
    </PageHeader>
    <div class="surface-card card-pad">
      <n-data-table :columns="columns" :data="items" :loading="loading" :pagination="pagination" :scroll-x="900">
        <template #empty><EmptyState compact title="暂无日志" /></template>
      </n-data-table>
    </div>
  </div>
</template>

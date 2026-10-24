<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, type DataTableColumns, type PaginationProps } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { OperationLog, Paged } from '@/types/api'
import { formatTime } from '@/utils/format'
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
    <div class="page-toolbar">
      <n-button :loading="loading" @click="load()">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
    </div>
    <div class="surface-card">
      <n-data-table :columns="columns" :data="items" :loading="loading" :pagination="pagination" size="small" :scroll-x="700" />
      <div v-if="!items.length && !loading" class="empty-state">暂无日志</div>
    </div>
  </div>
</template>

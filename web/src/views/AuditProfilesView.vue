<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { AddOutline, RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { AuditProfile, Authorizer, Paged } from '@/types/api'
import { formatTime } from '@/utils/format'
import AuditProfileModal from '@/components/AuditProfileModal.vue'

const message = useMessage()
const dialog = useDialog()
const profiles = ref<AuditProfile[]>([])
const apps = ref<Authorizer[]>([])
const loading = ref(false)
const modalShow = ref(false)
const editing = ref<AuditProfile | null>(null)

async function load() {
  loading.value = true
  try {
    const [profileResult, appResult] = await Promise.allSettled([
      api.get<{ items: AuditProfile[] }>('/audit-profiles'),
      api.get<Paged<Authorizer>>('/authorizers', { params: { page: 1, page_size: 100, status: 'active' }, silent: true }),
    ])
    if (profileResult.status === 'fulfilled') profiles.value = profileResult.value.items || []
    if (appResult.status === 'fulfilled') apps.value = appResult.value.items || []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

function createProfile() {
  editing.value = null
  modalShow.value = true
}

function editProfile(profile: AuditProfile) {
  editing.value = profile
  modalShow.value = true
}

function deleteProfile(profile: AuditProfile) {
  dialog.warning({
    title: '删除配置',
    content: `确认删除「${profile.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.delete(`/audit-profiles/${profile.id}`)
      message.success('已删除')
      await load()
    },
  })
}

const columns = computed<DataTableColumns<AuditProfile>>(() => [
  {
    title: '名称',
    key: 'name',
    minWidth: 220,
    render: (row) =>
      h('div', { style: 'display:flex;align-items:center;gap:8px' }, [
        h('span', { style: 'font-weight:600' }, row.name),
        row.is_default ? h(NTag, { size: 'small', type: 'primary', bordered: false }, { default: () => '默认' }) : null,
      ]),
  },
  { title: '审核项', key: 'item_list', width: 90, render: (row) => `${row.item_list?.length || 0} 个` },
  {
    title: '类目',
    key: 'categories',
    minWidth: 240,
    render: (row) => {
      const items = (row.item_list || []).slice(0, 2).map((item) => `${item.first_class}/${item.second_class}`)
      return items.length ? items.join('、') + ((row.item_list?.length || 0) > 2 ? '…' : '') : '—'
    },
  },
  { title: '更新时间', key: 'updated_at', width: 180, render: (row) => formatTime(row.updated_at) },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render: (row) =>
      h('div', { class: 'inline-actions' }, [
        h(NButton, { text: true, type: 'primary', onClick: () => editProfile(row) }, { default: () => '编辑' }),
        h(NButton, { text: true, onClick: () => deleteProfile(row) }, { default: () => '删除' }),
      ]),
  },
])
</script>

<template>
  <div>
    <div class="page-toolbar">
      <n-button :loading="loading" @click="load">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
      <n-button type="primary" @click="createProfile">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        新建配置
      </n-button>
    </div>

    <div class="surface-card">
      <n-data-table :columns="columns" :data="profiles" :loading="loading" :pagination="false" size="small" :scroll-x="900" />
      <div v-if="!profiles.length && !loading" class="empty-state">暂无审核配置</div>
    </div>

    <AuditProfileModal v-model:show="modalShow" :profile="editing" :apps="apps" @saved="load" />
  </div>
</template>

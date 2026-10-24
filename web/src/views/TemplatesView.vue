<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import { NButton, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { CodeDraft, CodeTemplate } from '@/types/api'
import { fromUnix } from '@/utils/format'

const message = useMessage()
const dialog = useDialog()
const tab = ref<'templates' | 'drafts'>('templates')
const templates = ref<CodeTemplate[]>([])
const drafts = ref<CodeDraft[]>([])
const loading = ref(false)
const draftsLoading = ref(false)
const syncing = ref(false)
const addingDraft = ref<number | null>(null)

async function loadTemplates() {
  loading.value = true
  try {
    const data = await api.get<{ items: CodeTemplate[] }>('/templates')
    templates.value = data.items || []
  } finally {
    loading.value = false
  }
}

async function loadDrafts() {
  draftsLoading.value = true
  try {
    const data = await api.get<{ items: CodeDraft[] }>('/templates/drafts')
    drafts.value = data.items || []
  } finally {
    draftsLoading.value = false
  }
}

async function syncTemplates() {
  syncing.value = true
  try {
    const data = await api.post<{ count: number }>('/templates/sync')
    message.success(`已同步 ${data.count} 个模板`)
    await loadTemplates()
  } finally {
    syncing.value = false
  }
}

function addDraft(draft: CodeDraft) {
  dialog.warning({
    title: '加入模板库',
    content: `确认加入草稿 #${draft.draft_id}？`,
    positiveText: '加入',
    negativeText: '取消',
    onPositiveClick: async () => {
      addingDraft.value = draft.draft_id
      try {
        await api.post('/templates/from-draft', { draft_id: draft.draft_id, template_type: 0 })
        message.success('已加入模板库')
        await loadTemplates()
        tab.value = 'templates'
      } finally {
        addingDraft.value = null
      }
    },
  })
}

function deleteTemplate(template: CodeTemplate) {
  dialog.warning({
    title: '删除模板',
    content: `确认删除模板 #${template.template_id}？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.delete(`/templates/${template.template_id}`)
      message.success('已删除')
      await loadTemplates()
    },
  })
}

const templateColumns = computed<DataTableColumns<CodeTemplate>>(() => [
  { title: '模板 ID', key: 'template_id', width: 110, render: (row) => `#${row.template_id}` },
  { title: '版本', key: 'user_version', width: 140, render: (row) => row.user_version || '—' },
  { title: '描述', key: 'user_desc', minWidth: 240, render: (row) => h('span', { class: 'cell-muted' }, row.user_desc || '—') },
  { title: '创建时间', key: 'create_time', width: 180, render: (row) => fromUnix(row.create_time) },
  {
    title: '操作',
    key: 'actions',
    width: 90,
    render: (row) => h(NButton, { text: true, onClick: () => deleteTemplate(row) }, { default: () => '删除' }),
  },
])

const draftColumns = computed<DataTableColumns<CodeDraft>>(() => [
  { title: '草稿 ID', key: 'draft_id', width: 110, render: (row) => `#${row.draft_id}` },
  { title: '版本', key: 'user_version', width: 140, render: (row) => row.user_version || '—' },
  { title: '描述', key: 'user_desc', minWidth: 240, render: (row) => h('span', { class: 'cell-muted' }, row.user_desc || '—') },
  { title: '上传时间', key: 'create_time', width: 180, render: (row) => fromUnix(row.create_time) },
  {
    title: '操作',
    key: 'actions',
    width: 130,
    render: (row) =>
      h(
        NButton,
        { text: true, type: 'primary', loading: addingDraft.value === row.draft_id, onClick: () => addDraft(row) },
        { default: () => '加入模板库' },
      ),
  },
])

watch(tab, (value) => {
  if (value === 'drafts' && drafts.value.length === 0) void loadDrafts()
})

onMounted(() => {
  void loadTemplates()
})
</script>

<template>
  <div>
    <div class="page-toolbar">
      <n-button :loading="draftsLoading" @click="loadDrafts">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新草稿
      </n-button>
      <n-button type="primary" :loading="syncing" @click="syncTemplates">同步模板</n-button>
    </div>

    <div class="surface-card tab-card">
      <n-tabs v-model:value="tab" type="line" animated>
        <n-tab-pane name="templates" tab="模板库">
          <n-data-table :columns="templateColumns" :data="templates" :loading="loading" :pagination="false" size="small" :scroll-x="800" />
          <div v-if="!templates.length && !loading" class="empty-state">暂无模板</div>
        </n-tab-pane>
        <n-tab-pane name="drafts" tab="草稿箱">
          <n-data-table :columns="draftColumns" :data="drafts" :loading="draftsLoading" :pagination="false" size="small" :scroll-x="800" />
          <div v-if="!drafts.length && !draftsLoading" class="empty-state">暂无草稿</div>
        </n-tab-pane>
      </n-tabs>
    </div>
  </div>
</template>

<style scoped>
.tab-card {
  padding: 8px 20px 20px;
}

@media (max-width: 720px) {
  .tab-card {
    padding: 6px 12px 16px;
  }
}
</style>

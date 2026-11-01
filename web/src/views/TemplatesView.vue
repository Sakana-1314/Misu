<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import { NButton, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { CodeSlashOutline, DocumentsOutline, SyncOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { CodeDraft, CodeTemplate } from '@/types/api'
import { fromUnix } from '@/utils/format'
import { useJobCreated } from '@/utils/actions'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import BatchActionModal from '@/components/BatchActionModal.vue'

const message = useMessage()
const dialog = useDialog()
const onJobCreated = useJobCreated()
const tab = ref<'templates' | 'drafts'>('templates')
const templates = ref<CodeTemplate[]>([])
const drafts = ref<CodeDraft[]>([])
const loading = ref(false)
const draftsLoading = ref(false)
const draftsLoaded = ref(false)
const syncing = ref(false)
const addingDraft = ref<number | null>(null)
const uploadShow = ref(false)
const uploadTemplateId = ref<number | null>(null)

async function loadTemplates() {
  loading.value = true
  try {
    const data = await api.get<{ items: CodeTemplate[] }>('/templates')
    templates.value = (data.items || []).sort((a, b) => b.create_time - a.create_time)
  } finally {
    loading.value = false
  }
}

async function loadDrafts() {
  draftsLoading.value = true
  try {
    const data = await api.get<{ items: CodeDraft[] }>('/templates/drafts')
    drafts.value = (data.items || []).sort((a, b) => b.create_time - a.create_time)
    draftsLoaded.value = true
  } finally {
    draftsLoading.value = false
  }
}

async function syncTemplates() {
  syncing.value = true
  try {
    const data = await api.post<{ count: number }>('/templates/sync')
    message.success(`已同步，共 ${data.count} 个模板`)
    await loadTemplates()
    if (draftsLoaded.value) await loadDrafts()
  } finally {
    syncing.value = false
  }
}

function addDraft(draft: CodeDraft) {
  addingDraft.value = draft.draft_id
  api
    .post('/templates/from-draft', { draft_id: draft.draft_id, template_type: 0 })
    .then(async () => {
      message.success(`${draft.user_version || '草稿'} 已加入模板库`)
      await loadTemplates()
      tab.value = 'templates'
    })
    .finally(() => {
      addingDraft.value = null
    })
}

function deleteTemplate(template: CodeTemplate) {
  dialog.warning({
    title: '删除模板',
    content: `确定从模板库中删除 ${template.user_version || `#${template.template_id}`}？已上传到小程序的代码不受影响。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.delete(`/templates/${template.template_id}`)
      message.success('已删除')
      await loadTemplates()
    },
  })
}

function uploadTemplate(template: CodeTemplate) {
  uploadTemplateId.value = template.id
  uploadShow.value = true
}

const latestId = computed(() => templates.value[0]?.id)

const templateColumns = computed<DataTableColumns<CodeTemplate>>(() => [
  {
    title: '版本',
    key: 'user_version',
    width: 160,
    render: (row) =>
      h('div', { class: 'version-cell' }, [
        h('b', row.user_version || '未命名'),
        row.id === latestId.value ? h('span', { class: 'latest' }, '最新') : null,
      ]),
  },
  { title: '说明', key: 'user_desc', minWidth: 240, render: (row) => h('span', { class: 'cell-muted' }, row.user_desc || '—') },
  { title: '来源', key: 'source_miniprogram', width: 160, render: (row) => h('span', { class: 'cell-muted' }, row.source_miniprogram || '—') },
  { title: '加入时间', key: 'create_time', width: 170, render: (row) => h('span', { class: 'cell-muted' }, fromUnix(row.create_time)) },
  {
    title: '',
    key: 'actions',
    width: 200,
    render: (row) =>
      h('div', { class: 'inline-actions row-actions' }, [
        h(NButton, { size: 'small', type: 'primary', secondary: true, onClick: () => uploadTemplate(row) }, { default: () => '上传到小程序' }),
        h(NButton, { size: 'small', quaternary: true, onClick: () => deleteTemplate(row) }, { default: () => '删除' }),
      ]),
  },
])

const draftColumns = computed<DataTableColumns<CodeDraft>>(() => [
  { title: '版本', key: 'user_version', width: 140, render: (row) => h('b', row.user_version || '未命名') },
  { title: '说明', key: 'user_desc', minWidth: 220, render: (row) => h('span', { class: 'cell-muted' }, row.user_desc || '—') },
  { title: '上传者', key: 'developer', width: 140, render: (row) => h('span', { class: 'cell-muted' }, row.developer || '—') },
  { title: '上传时间', key: 'create_time', width: 170, render: (row) => h('span', { class: 'cell-muted' }, fromUnix(row.create_time)) },
  {
    title: '',
    key: 'actions',
    width: 130,
    render: (row) =>
      h(
        NButton,
        { size: 'small', type: 'primary', secondary: true, loading: addingDraft.value === row.draft_id, onClick: () => addDraft(row) },
        { default: () => '加入模板库' },
      ),
  },
])

watch(tab, (value) => {
  if (value === 'drafts' && !draftsLoaded.value) void loadDrafts()
})

onMounted(() => {
  void loadTemplates()
})
</script>

<template>
  <div>
    <PageHeader title="代码模板" description="用开发者工具上传的代码会先进入草稿箱，加入模板库后就能批量上传到各个小程序。">
      <n-tooltip>
        <template #trigger>
          <n-button :loading="syncing" @click="syncTemplates">
            <template #icon><n-icon><SyncOutline /></n-icon></template>
            同步
          </n-button>
        </template>
        从微信开放平台拉取最新的模板库
      </n-tooltip>
    </PageHeader>

    <div class="flow surface-card">
      <div class="flow-step"><span>1</span>开发者工具上传代码</div>
      <div class="flow-line" />
      <div class="flow-step"><span>2</span>草稿箱中加入模板库</div>
      <div class="flow-line" />
      <div class="flow-step"><span>3</span>上传到小程序并提审</div>
    </div>

    <div class="surface-card tab-card">
      <n-tabs v-model:value="tab" type="line" animated>
        <n-tab-pane name="templates" :tab="`模板库 ${templates.length ? templates.length : ''}`">
          <n-data-table :columns="templateColumns" :data="templates" :loading="loading" :pagination="false" :scroll-x="900">
            <template #empty>
              <EmptyState
                compact
                :icon="CodeSlashOutline"
                title="模板库还是空的"
                description="先在「草稿箱」中把开发者工具上传的代码加入模板库。"
              >
                <n-button size="small" @click="tab = 'drafts'">去草稿箱</n-button>
              </EmptyState>
            </template>
          </n-data-table>
        </n-tab-pane>
        <n-tab-pane name="drafts" tab="草稿箱">
          <n-data-table :columns="draftColumns" :data="drafts" :loading="draftsLoading" :pagination="false" :scroll-x="800">
            <template #empty>
              <EmptyState
                compact
                :icon="DocumentsOutline"
                title="草稿箱是空的"
                description="在微信开发者工具中，用第三方平台的开发小程序点击「上传」，代码会出现在这里。"
              />
            </template>
          </n-data-table>
        </n-tab-pane>
      </n-tabs>
    </div>

    <BatchActionModal
      v-model:show="uploadShow"
      action="commit"
      :appids="[]"
      :initial-template-id="uploadTemplateId"
      @created="onJobCreated"
    />
  </div>
</template>

<style scoped>
.flow {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  padding: 12px 18px;
}

.flow-step {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  font-weight: 560;
  white-space: nowrap;
}

.flow-step span {
  width: 20px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 11px;
  font-weight: 700;
}

.flow-line {
  flex: 1;
  height: 1px;
  min-width: 16px;
  background: var(--border);
}

.tab-card {
  padding: 4px 20px 16px;
}

:deep(.version-cell) {
  display: flex;
  align-items: center;
  gap: 8px;
}

:deep(.latest) {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 11px;
  font-weight: 600;
}

:deep(.row-actions) {
  justify-content: flex-end;
}

@media (max-width: 760px) {
  .flow {
    flex-direction: column;
    align-items: flex-start;
  }

  .flow-line {
    display: none;
  }

  .tab-card {
    padding: 4px 12px 12px;
  }
}
</style>

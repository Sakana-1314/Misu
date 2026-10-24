<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NTag, useMessage, type DataTableColumns, type PaginationProps } from 'naive-ui'
import { AddOutline, RefreshOutline, SearchOutline, SyncOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { Authorizer, Job, Paged, SyncResult } from '@/types/api'
import { jobTypeOptions } from '@/utils/constants'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'
import BatchActionModal from '@/components/BatchActionModal.vue'
import AddAuthorizerModal from '@/components/AddAuthorizerModal.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const items = ref<Authorizer[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const status = ref<string | null>('active')
const loading = ref(false)
const syncing = ref(false)
const checkedKeys = ref<string[]>([])
const actionShow = ref(false)
const actionType = ref<string | null>(null)
const addShow = ref(false)
let searchTimer: number | undefined

async function load() {
  loading.value = true
  try {
    const data = await api.get<Paged<Authorizer>>('/authorizers', {
      params: {
        page: page.value,
        page_size: pageSize.value,
        keyword: keyword.value.trim() || undefined,
        status: status.value || undefined,
      },
    })
    items.value = data.items || []
    total.value = data.total
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
  if (route.query.add === '1') {
    addShow.value = true
    router.replace({ name: 'authorizers' })
  }
})

onBeforeUnmount(() => {
  if (searchTimer) window.clearTimeout(searchTimer)
})

function onKeywordInput() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    page.value = 1
    void load()
  }, 350)
}

function onFilterChange() {
  page.value = 1
  void load()
}

async function syncAuthorizers() {
  syncing.value = true
  try {
    const result = await api.post<SyncResult>('/authorizers/sync', null, { timeout: 300000 })
    message.success(`已同步 ${result.total} 个小程序`)
    checkedKeys.value = []
    await load()
  } finally {
    syncing.value = false
  }
}

function hasCodePermission(row: Authorizer): boolean {
  return (row.func_info || []).some((item) => item.funcscope_category?.id === 18)
}

const missingPermissionCount = computed(
  () => items.value.filter((row) => row.status === 'active' && !hasCodePermission(row)).length,
)

const columns = computed<DataTableColumns<Authorizer>>(() => [
  { type: 'selection', fixed: 'left' },
  {
    title: '小程序',
    key: 'nick_name',
    minWidth: 260,
    render: (row) =>
      h('div', { class: 'app-cell' }, [
        h(AppAvatar, { src: row.head_img, name: row.nick_name, appid: row.appid, size: 38 }),
        h('div', { class: 'app-cell-text' }, [
          h('div', { class: 'app-name' }, row.nick_name || '未命名小程序'),
          h('div', { class: 'app-id' }, row.appid),
        ]),
      ]),
  },
  { title: '主体', key: 'principal_name', minWidth: 180, render: (row) => row.principal_name || '—' },
  { title: '状态', key: 'status', width: 120, render: (row) => h(StatusTag, { kind: 'authorizer', value: row.status }) },
  {
    title: '代码权限',
    key: 'func_info',
    width: 120,
    render: (row) => {
      if (row.status !== 'active') return h(NTag, { size: 'small', bordered: false }, { default: () => '—' })
      return hasCodePermission(row)
        ? h(NTag, { size: 'small', bordered: false, type: 'primary' }, { default: () => '已授权' })
        : h(NTag, { size: 'small', bordered: false, type: 'warning' }, { default: () => '缺少权限' })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 116,
    fixed: 'right',
    render: (row) =>
      h(
        NButton,
        { onClick: () => router.push(`/authorizers/${row.appid}`) }, { default: () => '详情' },
      ),
  },
])

const pagination = computed<PaginationProps>(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  onUpdatePage: (value: number) => {
    page.value = value
    void load()
  },
  onUpdatePageSize: (value: number) => {
    pageSize.value = value
    page.value = 1
    void load()
  },
}))

const moreOptions = jobTypeOptions
  .filter((item) => !['commit', 'submit_audit'].includes(item.value))
  .map((item) => ({ label: item.label, key: item.value }))

function openAction(action: string) {
  if (checkedKeys.value.length === 0) {
    message.warning('请先选择小程序')
    return
  }
  actionType.value = action
  actionShow.value = true
}

function onJobCreated(job: Job) {
  checkedKeys.value = []
  message.success(`已提交 ${job.total} 个小程序的任务`)
  window.setTimeout(() => void load(), 1500)
}

const statusOptions = [
  { label: '已授权', value: 'active' },
  { label: '已取消授权', value: 'unauthorized' },
]

const showEmpty = computed(() => !loading.value && items.value.length === 0 && !keyword.value && status.value === 'active')
</script>

<template>
  <div>
    <div class="page-toolbar">
      <div class="left-actions">
        <n-input
          v-model:value="keyword"
          clearable
          placeholder="搜索小程序"
          style="width: 240px"
          @input="onKeywordInput"
          @clear="onFilterChange"
        >
          <template #prefix><n-icon><SearchOutline /></n-icon></template>
        </n-input>
        <n-select
          v-model:value="status"
          :options="statusOptions"
          clearable
          placeholder="状态"
          style="width: 140px"
          @update:value="onFilterChange"
        />
      </div>
      <n-button :loading="loading" @click="load">
        <template #icon><n-icon><RefreshOutline /></n-icon></template>
        刷新
      </n-button>
      <n-button :loading="syncing" @click="syncAuthorizers">
        <template #icon><n-icon><SyncOutline /></n-icon></template>
        同步
      </n-button>
      <n-button type="primary" @click="addShow = true">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        新增
      </n-button>
    </div>

    <div class="stack-12">
      <n-alert v-if="missingPermissionCount > 0" type="warning" :bordered="false">
        部分小程序缺少代码管理权限，批量操作可能失败。
      </n-alert>

      <transition name="slide-up">
        <div v-if="checkedKeys.length > 0" class="batch-bar">
          <div class="batch-info">已选 {{ checkedKeys.length }} 个</div>
          <div class="batch-actions">
            <n-button type="primary" @click="openAction('commit')">上传</n-button>
            <n-button type="primary" @click="openAction('submit_audit')">提审</n-button>
            <n-dropdown trigger="click" :options="moreOptions" @select="(key: string | number) => openAction(String(key))">
              <n-button>更多</n-button>
            </n-dropdown>
            <n-button @click="checkedKeys = []">清空</n-button>
          </div>
        </div>
      </transition>

      <div v-if="showEmpty" class="surface-card empty-state">暂无小程序</div>

      <n-data-table
        v-else
        :columns="columns"
        :data="items"
        :loading="loading"
        :pagination="pagination"
        :row-key="(row: Authorizer) => row.appid"
        :checked-row-keys="checkedKeys"
        :scroll-x="900"
        size="small"
        @update:checked-row-keys="(keys: (string | number)[]) => (checkedKeys = keys as string[])"
      />
    </div>

    <AddAuthorizerModal v-model:show="addShow" @synced="load" />
    <BatchActionModal
      v-model:show="actionShow"
      :action="actionType"
      :appids="checkedKeys"
      :apps="items"
      @created="onJobCreated"
    />
  </div>
</template>

<style scoped>
.batch-bar {
  position: sticky;
  top: 76px;
  z-index: 12;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--primary-soft);
  flex-wrap: wrap;
}

.batch-info {
  font-size: 13px;
  font-weight: 560;
  color: var(--primary);
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .page-toolbar {
    justify-content: flex-start;
  }

  .page-toolbar .left-actions {
    width: 100%;
  }

  .page-toolbar .left-actions :deep(.n-input) {
    width: 100% !important;
  }

  .batch-bar {
    top: 68px;
  }
}
</style>

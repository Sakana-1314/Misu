<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NIcon, NTag, NTooltip, useMessage, type DataTableColumns, type PaginationProps } from 'naive-ui'
import { AddOutline, AppsOutline, ChevronDownOutline, ChevronForwardOutline, SearchOutline, SyncOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { Authorizer, Paged, SyncResult } from '@/types/api'
import { actionMenu, useJobCreated } from '@/utils/actions'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import AppAvatar from '@/components/AppAvatar.vue'
import StatusTag from '@/components/StatusTag.vue'
import BatchActionModal from '@/components/BatchActionModal.vue'
import AddAuthorizerModal from '@/components/AddAuthorizerModal.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const onJobCreated = useJobCreated()

const items = ref<Authorizer[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const status = ref<string>('active')
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
        status: status.value === 'all' ? undefined : status.value,
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
  checkedKeys.value = []
  void load()
}

async function syncAuthorizers() {
  syncing.value = true
  try {
    const result = await api.post<SyncResult>('/authorizers/sync', null, { timeout: 300000 })
    message.success(result.added > 0 ? `同步完成，新增 ${result.added} 个小程序` : `同步完成，共 ${result.total} 个小程序`)
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
  { type: 'selection', fixed: 'left', disabled: (row) => row.status !== 'active' },
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
  { title: '主体', key: 'principal_name', minWidth: 180, render: (row) => h('span', { class: 'cell-muted' }, row.principal_name || '—') },
  { title: '状态', key: 'status', width: 110, render: (row) => h(StatusTag, { kind: 'authorizer', value: row.status }) },
  {
    title: '代码管理',
    key: 'func_info',
    width: 130,
    render: (row) => {
      if (row.status !== 'active') return h('span', { class: 'cell-muted' }, '—')
      if (hasCodePermission(row)) return h(NTag, { size: 'small', bordered: false, type: 'primary' }, { default: () => '可用' })
      return h(
        NTooltip,
        { style: { maxWidth: '260px' } },
        {
          trigger: () => h(NTag, { size: 'small', bordered: false, type: 'warning' }, { default: () => '未授权' }),
          default: () => '授权时没有勾选「开发管理与数据分析」，无法上传代码和提审。请让管理员重新授权。',
        },
      )
    },
  },
  {
    title: '',
    key: 'actions',
    width: 92,
    fixed: 'right',
    render: (row) =>
      h(
        NButton,
        { size: 'small', quaternary: true, type: 'primary', iconPlacement: 'right', onClick: (event: MouseEvent) => {
          event.stopPropagation()
          void router.push(`/authorizers/${row.appid}`)
        } },
        { default: () => '管理', icon: () => h(NIcon, null, { default: () => h(ChevronForwardOutline) }) },
      ),
  },
])

const rowProps = (row: Authorizer) => ({
  style: 'cursor: pointer',
  onClick: (event: MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.n-checkbox, .n-data-table-td--selection')) return
    void router.push(`/authorizers/${row.appid}`)
  },
})

const pagination = computed<PaginationProps | false>(() =>
  total.value <= pageSize.value && page.value === 1
    ? false
    : {
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
      },
)

const moreOptions = actionMenu(['commit', 'submit_audit', 'release'])

function openAction(action: string) {
  if (checkedKeys.value.length === 0) {
    message.warning('请先勾选小程序')
    return
  }
  actionType.value = action
  actionShow.value = true
}

function handleCreated(job: Parameters<typeof onJobCreated>[0]) {
  checkedKeys.value = []
  onJobCreated(job)
}

const isEmptyLibrary = computed(() => !loading.value && items.value.length === 0 && !keyword.value && status.value === 'active')
</script>

<template>
  <div>
    <PageHeader title="小程序" description="已接入的小程序都在这里。勾选后可以批量上传代码、提交审核和发布。">
      <n-tooltip>
        <template #trigger>
          <n-button :loading="syncing" @click="syncAuthorizers">
            <template #icon><n-icon><SyncOutline /></n-icon></template>
            同步列表
          </n-button>
        </template>
        从微信开放平台拉取最新的授权状态
      </n-tooltip>
      <n-button type="primary" @click="addShow = true">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        接入小程序
      </n-button>
    </PageHeader>

    <div v-if="isEmptyLibrary" class="surface-card">
      <EmptyState
        :icon="AppsOutline"
        title="还没有接入小程序"
        description="接入后即可在这里统一上传代码、提交审核和发布。只需要小程序管理员扫码确认授权。"
      >
        <n-button type="primary" @click="addShow = true">接入第一个小程序</n-button>
        <n-button :loading="syncing" @click="syncAuthorizers">已授权过？同步列表</n-button>
      </EmptyState>
    </div>

    <div v-else class="surface-card list-card">
      <div class="list-toolbar">
        <n-radio-group v-model:value="status" size="small" @update:value="onFilterChange">
          <n-radio-button value="active">已接入</n-radio-button>
          <n-radio-button value="unauthorized">已取消授权</n-radio-button>
          <n-radio-button value="all">全部</n-radio-button>
        </n-radio-group>
        <n-input
          v-model:value="keyword"
          clearable
          placeholder="搜索名称或 AppID"
          class="search"
          @input="onKeywordInput"
          @clear="onFilterChange"
        >
          <template #prefix><n-icon><SearchOutline /></n-icon></template>
        </n-input>
      </div>

      <n-alert v-if="missingPermissionCount > 0" type="warning" :bordered="false" class="perm-alert">
        有 {{ missingPermissionCount }} 个小程序未授权代码管理，批量上传和提审时会被跳过。
      </n-alert>

      <n-data-table
        :columns="columns"
        :data="items"
        :loading="loading"
        :pagination="pagination"
        :row-key="(row: Authorizer) => row.appid"
        :row-props="rowProps"
        :checked-row-keys="checkedKeys"
        :scroll-x="880"
        @update:checked-row-keys="(keys: (string | number)[]) => (checkedKeys = keys as string[])"
      >
        <template #empty>
          <EmptyState compact title="没有找到小程序" description="换个关键词或筛选条件试试。" />
        </template>
      </n-data-table>
    </div>

    <transition name="slide-up">
      <div v-if="checkedKeys.length > 0" class="batch-bar">
        <div class="batch-info">
          已选 <b>{{ checkedKeys.length }}</b> 个
          <n-button text size="small" class="batch-clear" @click="checkedKeys = []">取消选择</n-button>
        </div>
        <div class="batch-actions">
          <n-button size="small" @click="openAction('commit')">上传代码</n-button>
          <n-button size="small" @click="openAction('submit_audit')">提交审核</n-button>
          <n-button size="small" type="primary" @click="openAction('release')">发布上线</n-button>
          <n-dropdown trigger="click" :options="moreOptions" placement="top-end" @select="(key: string | number) => openAction(String(key))">
            <n-button size="small" icon-placement="right">
              更多操作
              <template #icon><n-icon><ChevronDownOutline /></n-icon></template>
            </n-button>
          </n-dropdown>
        </div>
      </div>
    </transition>

    <AddAuthorizerModal v-model:show="addShow" @synced="load" />
    <BatchActionModal v-model:show="actionShow" :action="actionType" :appids="checkedKeys" :apps="items" @created="handleCreated" />
  </div>
</template>

<style scoped>
.list-card {
  padding: 14px 16px 6px;
}

.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.search {
  width: 260px;
}

.perm-alert {
  margin-bottom: 12px;
}

.batch-bar {
  position: fixed;
  left: 50%;
  bottom: 24px;
  z-index: 30;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 10px 12px 10px 18px;
  border-radius: 14px;
  background: #172033;
  color: #fff;
  box-shadow: 0 18px 40px -12px rgba(15, 23, 42, 0.45);
}

.batch-info {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  white-space: nowrap;
}

.batch-clear {
  color: rgba(255, 255, 255, 0.6);
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.batch-actions :deep(.n-button:not(.n-button--primary-type)) {
  --n-color: rgba(255, 255, 255, 0.08) !important;
  --n-color-hover: rgba(255, 255, 255, 0.16) !important;
  --n-color-pressed: rgba(255, 255, 255, 0.2) !important;
  --n-color-focus: rgba(255, 255, 255, 0.16) !important;
  --n-text-color: #fff !important;
  --n-text-color-hover: #fff !important;
  --n-text-color-pressed: #fff !important;
  --n-text-color-focus: #fff !important;
  --n-border: 1px solid rgba(255, 255, 255, 0.16) !important;
  --n-border-hover: 1px solid rgba(255, 255, 255, 0.3) !important;
  --n-border-pressed: 1px solid rgba(255, 255, 255, 0.3) !important;
  --n-border-focus: 1px solid rgba(255, 255, 255, 0.3) !important;
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translate(-50%, 12px);
}

@media (max-width: 760px) {
  .search {
    width: 100%;
  }

  .batch-bar {
    left: 12px;
    right: 12px;
    transform: none;
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .batch-actions {
    flex-wrap: wrap;
  }
}
</style>

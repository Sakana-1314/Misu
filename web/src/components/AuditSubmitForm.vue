<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { AddOutline, CloseOutline, RocketOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { AuditItem, Authorizer, Paged, SettingCategoriesResponse } from '@/types/api'
import MediaUploader from '@/components/MediaUploader.vue'

interface AuditItemDraft extends AuditItem {
  categoryKey: string | null
}

const props = withDefaults(
  defineProps<{
    apps?: Authorizer[]
    defaultAppid?: string | null
  }>(),
  {
    apps: () => [],
    defaultAppid: null,
  },
)

const appList = ref<Authorizer[]>([])
const loadingApps = ref(false)
const referenceAppid = ref<string | null>(props.defaultAppid)
const categories = ref<SettingCategoriesResponse | null>(null)
const pages = ref<string[]>([])
const loadingRef = ref(false)
const referenceError = ref('')
const items = ref<AuditItemDraft[]>([emptyItem()])
const versionDesc = ref('')
const autoRelease = ref(false)
const hasUGC = ref(false)
const ugc = reactive({
  scene: [] as number[],
  otherSceneDesc: '',
  method: [] as number[],
  hasAuditTeam: 0,
  auditDesc: '',
})
const previewPics = ref<number[]>([])
const previewVideos = ref<number[]>([])
const feedbackInfo = ref('')
const feedbackAssets = ref<number[]>([])

const sceneOptions = [
  { label: '用户资料（头像、昵称等）', value: 1 },
  { label: '图片', value: 2 },
  { label: '视频', value: 3 },
  { label: '文字（评论、帖子等）', value: 4 },
  { label: '其他', value: 5 },
]

const methodOptions = [
  { label: '使用微信内容安全接口', value: 1 },
  { label: '使用其他内容审核服务', value: 2 },
  { label: '人工审核', value: 3 },
  { label: '暂未审核', value: 4 },
]

function emptyItem(): AuditItemDraft {
  return {
    categoryKey: null,
    address: '',
    tag: '',
    first_class: '',
    second_class: '',
    third_class: '',
    first_id: 0,
    second_id: 0,
    third_id: 0,
    title: '',
  }
}

const multiApp = computed(() => props.apps.length > 1)

const appOptions = computed(() => {
  const seen = new Map<string, Authorizer>()
  for (const app of [...props.apps, ...appList.value]) {
    if (app.status === 'active' && !seen.has(app.appid)) seen.set(app.appid, app)
  }
  return Array.from(seen.values()).map((app) => ({ label: app.nick_name || app.appid, value: app.appid }))
})

async function loadApps() {
  loadingApps.value = true
  try {
    const data = await api.get<Paged<Authorizer>>('/authorizers', {
      params: { page: 1, page_size: 100, status: 'active' },
      silent: true,
    })
    appList.value = data.items || []
  } finally {
    loadingApps.value = false
  }
}

const pageOptions = computed(() => pages.value.map((page) => ({ label: page, value: page })))
const categoryOptions = computed(() =>
  (categories.value?.categories || []).map((category) => ({
    label: `${category.first_name} › ${category.second_name}${
      category.audit_status === 3 ? '' : category.audit_status === 1 ? '（类目审核中）' : '（类目未通过）'
    }`,
    value: `${category.first}-${category.second}`,
    disabled: category.audit_status !== 3 && category.audit_status !== 1,
  })),
)

const showOtherScene = computed(() => ugc.scene.includes(5))

function applyCategory(item: AuditItemDraft, value: string | null) {
  item.categoryKey = value
  const selected = (categories.value?.categories || []).find((category) => `${category.first}-${category.second}` === value)
  item.first_class = selected?.first_name || ''
  item.second_class = selected?.second_name || ''
  item.first_id = selected?.first || 0
  item.second_id = selected?.second || 0
}

// Fill the first audit item with the most likely answers so the common case
// needs no typing at all.
function applyDefaults() {
  const first = items.value[0]
  if (!first) return
  if (!first.categoryKey) {
    const approved = (categories.value?.categories || []).find((category) => category.audit_status === 3)
    if (approved) applyCategory(first, `${approved.first}-${approved.second}`)
  }
  if (!first.address && pages.value.length > 0) first.address = pages.value[0]
}

async function loadReference(appid: string | null) {
  categories.value = null
  pages.value = []
  referenceError.value = ''
  if (!appid) return
  loadingRef.value = true
  try {
    const [categoryResult, pageResult] = await Promise.allSettled([
      api.get<SettingCategoriesResponse>(`/apps/${appid}/categories`, { silent: true }),
      api.get<{ items: string[] }>(`/apps/${appid}/pages`, { silent: true }),
    ])
    if (categoryResult.status === 'fulfilled') categories.value = categoryResult.value
    if (pageResult.status === 'fulfilled') pages.value = pageResult.value.items || []
    if (categoryResult.status === 'rejected') {
      referenceError.value = '没能读取到已配置的类目，请确认小程序已在后台设置服务类目。'
    } else if (!categories.value?.categories?.length) {
      referenceError.value = '该小程序还没有配置服务类目，请先到小程序后台「设置 → 服务类目」中添加。'
    }
    applyDefaults()
  } finally {
    loadingRef.value = false
  }
}

watch(
  () => props.defaultAppid,
  (appid) => {
    referenceAppid.value = appid
  },
)

onMounted(() => {
  if (multiApp.value) void loadApps()
})

watch(
  referenceAppid,
  (appid) => {
    for (const item of items.value) applyCategory(item, null)
    void loadReference(appid)
  },
  { immediate: true },
)

function addItem() {
  if (items.value.length >= 5) return
  items.value.push(emptyItem())
}

function removeItem(index: number) {
  if (items.value.length <= 1) return
  items.value.splice(index, 1)
}

function buildPayload(): { payload: Record<string, unknown> } | { error: string } {
  const itemList: AuditItem[] = []
  for (let index = 0; index < items.value.length; index += 1) {
    const item = items.value[index]
    const label = items.value.length > 1 ? `第 ${index + 1} 个页面` : '审核页面'
    if (!item.first_id || !item.second_id) return { error: `${label}请选择服务类目` }
    if (!item.address?.trim()) return { error: `${label}请选择页面路径` }
    itemList.push({
      address: item.address.trim(),
      tag: (item.tag || '').trim(),
      first_class: item.first_class,
      second_class: item.second_class,
      third_class: '',
      first_id: item.first_id,
      second_id: item.second_id,
      third_id: 0,
      title: (item.title || '').trim(),
    })
  }
  let ugcDeclare: Record<string, unknown> | null = null
  if (hasUGC.value) {
    if (ugc.scene.length === 0) return { error: '请选择用户可以发布哪些内容' }
    if (ugc.method.length === 0) return { error: '请选择如何审核用户发布的内容' }
    ugcDeclare = {
      scene: ugc.scene,
      method: ugc.method,
      other_scene_desc: showOtherScene.value ? ugc.otherSceneDesc.trim() : '',
      has_audit_team: ugc.hasAuditTeam,
      audit_desc: ugc.auditDesc.trim(),
    }
  }
  return {
    payload: {
      item_list: itemList,
      version_desc: versionDesc.value.trim(),
      feedback_info: feedbackInfo.value.trim(),
      feedback_assets: feedbackAssets.value,
      preview_pics: previewPics.value,
      preview_videos: previewVideos.value,
      ugc_declare: ugcDeclare,
      auto_release: autoRelease.value,
    },
  }
}

defineExpose({ buildPayload })
</script>

<template>
  <div class="audit-form">
    <div v-if="multiApp" class="field">
      <div class="field-label">参照小程序</div>
      <n-select
        v-model:value="referenceAppid"
        :options="appOptions"
        :loading="loadingApps"
        filterable
        placeholder="选择一个小程序读取类目和页面"
      />
      <div class="hint field-hint">批量提审时，所有小程序使用同一份审核信息。这里选一个来读取可选的类目和页面。</div>
    </div>

    <n-alert v-if="referenceError" type="warning" :bordered="false" class="ref-alert">{{ referenceError }}</n-alert>

    <div class="field">
      <div class="field-label">版本说明 <span class="optional">给审核人员看，选填</span></div>
      <n-input v-model:value="versionDesc" maxlength="200" placeholder="例如：新增书单分享功能，修复已知问题" />
    </div>

    <div class="field">
      <div class="field-label">
        送审页面
        <span class="optional">审核人员会重点查看这些页面</span>
      </div>
      <n-spin :show="loadingRef">
        <div class="item-list">
          <div v-for="(item, index) in items" :key="index" class="item-card">
            <div class="item-grid">
              <n-select
                :value="item.categoryKey"
                :options="categoryOptions"
                filterable
                placeholder="服务类目"
                @update:value="(value: string | null) => applyCategory(item, value)"
              />
              <n-select
                :value="item.address || null"
                :options="pageOptions"
                @update:value="(value: string | null) => (item.address = value || '')"
                filterable
                tag
                clearable
                placeholder="页面路径，可直接输入"
              />
              <n-button
                v-if="items.length > 1"
                quaternary
                circle
                class="icon-button"
                title="移除"
                @click="removeItem(index)"
              >
                <template #icon><n-icon><CloseOutline /></n-icon></template>
              </n-button>
            </div>
            <n-collapse class="item-more" arrow-placement="left">
              <n-collapse-item title="补充页面标题和标签" :name="`more-${index}`">
                <div class="grid-2">
                  <n-input v-model:value="item.title" maxlength="32" placeholder="页面标题，选填" />
                  <n-input v-model:value="item.tag" placeholder="标签，用空格分隔，选填" />
                </div>
              </n-collapse-item>
            </n-collapse>
          </div>
        </div>
      </n-spin>
      <n-button v-if="items.length < 5" text type="primary" class="add-item" @click="addItem">
        <template #icon><n-icon><AddOutline /></n-icon></template>
        再添加一个页面
      </n-button>
    </div>

    <div class="field">
      <div class="field-label">用户能在小程序里发布内容吗？</div>
      <n-radio-group v-model:value="hasUGC">
        <n-radio :value="false">不能</n-radio>
        <n-radio :value="true">能，例如评论、上传图片</n-radio>
      </n-radio-group>
      <div v-if="hasUGC" class="ugc-box">
        <div class="sub-label">可以发布哪些内容</div>
        <n-checkbox-group v-model:value="ugc.scene">
          <div class="check-grid">
            <n-checkbox v-for="option in sceneOptions" :key="option.value" :value="option.value" :label="option.label" />
          </div>
        </n-checkbox-group>
        <n-input v-if="showOtherScene" v-model:value="ugc.otherSceneDesc" maxlength="256" placeholder="说明其他内容类型" class="mt-8" />
        <div class="sub-label">如何审核这些内容</div>
        <n-checkbox-group v-model:value="ugc.method">
          <div class="check-grid">
            <n-checkbox v-for="option in methodOptions" :key="option.value" :value="option.value" :label="option.label" />
          </div>
        </n-checkbox-group>
        <div class="team-row">
          <n-switch v-model:value="ugc.hasAuditTeam" :checked-value="1" :unchecked-value="0" size="small" />
          <span>有专门的内容审核团队</span>
        </div>
        <n-input v-model:value="ugc.auditDesc" maxlength="256" placeholder="简单说明审核机制，选填" class="mt-8" />
      </div>
    </div>

    <n-collapse class="extras" arrow-placement="left">
      <n-collapse-item title="补充材料（选填）" name="materials">
        <div class="grid-2">
          <div>
            <div class="sub-label">功能截图</div>
            <MediaUploader v-model="previewPics" accept="image/*" :max="10" />
          </div>
          <div>
            <div class="sub-label">操作录屏</div>
            <MediaUploader v-model="previewVideos" accept="video/*" :max="5" />
          </div>
        </div>
      </n-collapse-item>
      <n-collapse-item title="上次被驳回？给审核人员留言" name="feedback">
        <n-input
          v-model:value="feedbackInfo"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          maxlength="200"
          placeholder="说明已经按要求做了哪些修改"
        />
        <div class="sub-label">相关截图</div>
        <MediaUploader v-model="feedbackAssets" accept="image/*" :max="5" />
      </n-collapse-item>
    </n-collapse>

    <div class="auto-release" :class="{ on: autoRelease }">
      <n-icon class="auto-icon"><RocketOutline /></n-icon>
      <div class="auto-text">
        <div class="auto-title">审核通过后自动发布</div>
        <div class="hint">开启后无需再手动点击发布，适合已经充分测试过的版本。</div>
      </div>
      <n-switch v-model:value="autoRelease" />
    </div>
  </div>
</template>

<style scoped>
.audit-form {
  width: 100%;
}

.field {
  margin-bottom: 18px;
}

.field-label {
  margin-bottom: 7px;
  font-size: 13px;
  font-weight: 600;
}

.optional {
  margin-left: 6px;
  color: var(--text-faint);
  font-size: 12px;
  font-weight: 400;
}

.field-hint {
  margin-top: 6px;
}

.ref-alert {
  margin-bottom: 16px;
}

.item-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-card {
  padding: 10px 12px 2px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.item-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.item-grid:not(:has(.icon-button)) {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}

.item-more {
  margin-top: 4px;
}

.item-more :deep(.n-collapse-item__header-main) {
  font-size: 12px;
  color: var(--text-muted);
}

.add-item {
  margin-top: 10px;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 14px;
}

.ugc-box {
  margin-top: 10px;
  padding: 4px 14px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.sub-label {
  margin: 12px 0 7px;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
}

.check-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 12px;
}

.team-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  font-size: 13px;
}

.mt-8 {
  margin-top: 8px;
}

.extras {
  margin-bottom: 16px;
}

.auto-release {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  transition: all 0.15s ease;
}

.auto-release.on {
  border-color: var(--primary);
  background: var(--primary-faint);
}

.auto-icon {
  font-size: 20px;
  color: var(--primary);
}

.auto-text {
  flex: 1;
}

.auto-title {
  font-size: 13.5px;
  font-weight: 620;
}

@media (max-width: 720px) {
  .grid-2,
  .check-grid,
  .item-grid {
    grid-template-columns: 1fr;
  }
}
</style>

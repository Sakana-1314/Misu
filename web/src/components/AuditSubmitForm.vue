<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { AddOutline, CloseOutline } from '@vicons/ionicons5'
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

const message = useMessage()

const referenceAppid = ref<string | null>(props.defaultAppid)
const categories = ref<SettingCategoriesResponse | null>(null)
const pages = ref<string[]>([])
const loadingRef = ref(false)
const items = ref<AuditItemDraft[]>([emptyItem()])
const versionDesc = ref('')
const autoRelease = ref(false)
const ugcEnabled = ref(false)
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
  { label: '不涉及用户生成内容', value: 0 },
  { label: '用户资料', value: 1 },
  { label: '图片', value: 2 },
  { label: '视频', value: 3 },
  { label: '文本', value: 4 },
  { label: '其他', value: 5 },
]

const methodOptions = [
  { label: '平台内容安全 API', value: 1 },
  { label: '其他内容审核产品', value: 2 },
  { label: '人工审核把关', value: 3 },
  { label: '未做审核把关', value: 4 },
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

const appOptions = computed(() => {
  const seen = new Map<string, Authorizer>()
  for (const app of props.apps) {
    if (app.status === 'active' && !seen.has(app.appid)) seen.set(app.appid, app)
  }
  return Array.from(seen.values()).map((app) => ({
    label: `${app.nick_name || app.appid}（${app.appid}）`,
    value: app.appid,
  }))
})

const pageOptions = computed(() => pages.value.map((page) => ({ label: page, value: page })))
const categoryOptions = computed(
  () =>
    (categories.value?.categories || []).map((category) => ({
      label: `${category.first_name} / ${category.second_name}${
        category.audit_status === 3 ? '' : category.audit_status === 1 ? '（审核中）' : '（未通过）'
      }`,
      value: `${category.first}-${category.second}`,
    })),
)

const showOtherScene = computed(() => ugc.scene.includes(5))
const noUGC = computed(() => ugc.scene.includes(0))

function applyCategory(item: AuditItemDraft, value: string | null) {
  item.categoryKey = value
  if (!value) {
    item.first_class = ''
    item.second_class = ''
    item.first_id = 0
    item.second_id = 0
    return
  }
  const selected = (categories.value?.categories || []).find((category) => `${category.first}-${category.second}` === value)
  if (!selected) return
  item.first_class = selected.first_name
  item.second_class = selected.second_name
  item.first_id = selected.first
  item.second_id = selected.second
}

async function loadReference(appid: string | null) {
  categories.value = null
  pages.value = []
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
      message.warning('未能读取该小程序已配置的类目')
    }
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

watch(
  referenceAppid,
  (appid) => {
    for (const item of items.value) item.categoryKey = null
    void loadReference(appid)
  },
  { immediate: true },
)

watch(
  () => ugc.scene.slice(),
  (scene) => {
    if (scene.includes(0) && scene.length > 1) {
      ugc.scene = [0]
    }
    if (!scene.includes(5)) ugc.otherSceneDesc = ''
    if (scene.includes(0)) {
      ugc.method = []
      ugc.auditDesc = ''
      ugc.hasAuditTeam = 0
    }
  },
)

function addItem() {
  if (items.value.length >= 5) {
    message.warning('审核项最多 5 个')
    return
  }
  items.value.push(emptyItem())
}

function removeItem(index: number) {
  if (items.value.length <= 1) return
  items.value.splice(index, 1)
}

function buildPayload(): { payload: Record<string, unknown> } | { error: string } {
  if (items.value.length < 1) return { error: '请至少添加一个审核项' }
  const itemList: AuditItem[] = []
  for (let index = 0; index < items.value.length; index += 1) {
    const item = items.value[index]
    if (!item.first_id || !item.second_id) return { error: `第 ${index + 1} 个审核项请选择类目` }
    if (!item.address?.trim()) return { error: `第 ${index + 1} 个审核项请选择页面路径` }
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
  if (ugcEnabled.value) {
    if (ugc.scene.length === 0) return { error: 'UGC 声明请选择涉及场景' }
    if (!noUGC.value && ugc.method.length === 0) return { error: 'UGC 声明请选择审核方式' }
    ugcDeclare = {
      scene: ugc.scene,
      method: noUGC.value ? [] : ugc.method,
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
    <n-form label-placement="top" :show-feedback="true">
      <div class="row-2">
        <n-form-item label="参考小程序">
          <n-select
            v-model:value="referenceAppid"
            :options="appOptions"
            :loading="loadingRef"
            filterable
            placeholder="选择一个小程序读取类目和页面"
          />
        </n-form-item>
        <n-form-item label="版本说明">
          <n-input v-model:value="versionDesc" maxlength="200" placeholder="例如 修复已知问题" />
        </n-form-item>
      </div>

      <div class="section-head">
        <span class="section-title">审核项</span>
        <n-button :disabled="items.length >= 5" @click="addItem">
          <template #icon><n-icon><AddOutline /></n-icon></template>
          添加
        </n-button>
      </div>

      <div v-for="(item, index) in items" :key="index" class="item-card">
        <div class="item-head">
          <span>审核项 {{ index + 1 }}</span>
          <n-button circle class="icon-button" :disabled="items.length <= 1" @click="removeItem(index)">
            <template #icon><n-icon><CloseOutline /></n-icon></template>
          </n-button>
        </div>
        <div class="row-2">
          <n-form-item label="类目">
            <n-select
              :value="item.categoryKey"
              :options="categoryOptions"
              :loading="loadingRef"
              filterable
              placeholder="选择已配置类目"
              @update:value="(value: string | null) => applyCategory(item, value)"
            />
          </n-form-item>
          <n-form-item label="页面">
            <n-select
              v-model:value="item.address"
              :options="pageOptions"
              :loading="loadingRef"
              filterable
              tag
              clearable
              placeholder="选择或输入页面路径"
            />
          </n-form-item>
        </div>
        <div class="row-2">
          <n-form-item label="页面标题">
            <n-input v-model:value="item.title" maxlength="32" placeholder="选填" />
          </n-form-item>
          <n-form-item label="审核标签">
            <n-input v-model:value="item.tag" placeholder="选填，用空格分隔" />
          </n-form-item>
        </div>
      </div>

      <n-divider style="margin: 6px 0 16px" />

      <div class="section-head">
        <span class="section-title">预览素材</span>
      </div>
      <div class="row-2">
        <n-form-item label="截图">
          <MediaUploader v-model="previewPics" accept="image/*" :max="10" />
        </n-form-item>
        <n-form-item label="录屏">
          <MediaUploader v-model="previewVideos" accept="video/*" :max="5" />
        </n-form-item>
      </div>

      <div class="section-head">
        <span class="section-title">UGC 声明</span>
        <n-switch v-model:value="ugcEnabled" />
      </div>
      <template v-if="ugcEnabled">
        <n-form-item label="涉及场景">
          <n-checkbox-group v-model:value="ugc.scene">
            <n-space>
              <n-checkbox v-for="option in sceneOptions" :key="option.value" :value="option.value" :label="option.label" />
            </n-space>
          </n-checkbox-group>
        </n-form-item>
        <n-form-item v-if="showOtherScene" label="其他场景说明">
          <n-input v-model:value="ugc.otherSceneDesc" maxlength="256" placeholder="说明其他用户生成内容场景" />
        </n-form-item>
        <template v-if="!noUGC">
          <n-form-item label="审核方式">
            <n-checkbox-group v-model:value="ugc.method">
              <n-space>
                <n-checkbox v-for="option in methodOptions" :key="option.value" :value="option.value" :label="option.label" />
              </n-space>
            </n-checkbox-group>
          </n-form-item>
          <div class="row-2">
            <n-form-item label="设有审核团队">
              <n-switch v-model:value="ugc.hasAuditTeam" :checked-value="1" :unchecked-value="0" />
            </n-form-item>
            <n-form-item label="审核机制说明">
              <n-input v-model:value="ugc.auditDesc" maxlength="256" placeholder="说明当前 UGC 审核机制" />
            </n-form-item>
          </div>
        </template>
      </template>

      <div class="section-head">
        <span class="section-title">驳回反馈</span>
      </div>
      <n-form-item label="反馈说明">
        <n-input
          v-model:value="feedbackInfo"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          maxlength="200"
          placeholder="仅上一次审核被驳回时有效"
        />
      </n-form-item>
      <n-form-item label="反馈截图">
        <MediaUploader v-model="feedbackAssets" accept="image/*" :max="5" />
      </n-form-item>

      <n-form-item label="审核通过后自动发布">
        <n-switch v-model:value="autoRelease" />
      </n-form-item>
    </n-form>
  </div>
</template>

<style scoped>
.audit-form {
  width: 100%;
}

.row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 14px 0 10px;
}

.section-title {
  font-size: 14px;
  font-weight: 650;
}

.item-card {
  padding: 10px 12px 0;
  margin-bottom: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 600;
}

@media (max-width: 720px) {
  .row-2 {
    grid-template-columns: 1fr;
  }
}
</style>

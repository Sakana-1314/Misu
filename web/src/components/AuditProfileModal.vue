<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '@/api/client'
import type { AuditItem, AuditProfile, Authorizer, SettingCategory, SettingCategoriesResponse } from '@/types/api'
import { prettyJSON } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    show: boolean
    profile?: AuditProfile | null
    apps?: Authorizer[]
  }>(),
  {
    profile: null,
    apps: () => [],
  },
)

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'saved'): void
}>()

const message = useMessage()
const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value),
})

function emptyItem(): AuditItem {
  return {
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

const form = reactive({
  name: '',
  version_desc: '',
  feedback_info: '',
  feedback_stuff: '',
  is_default: false,
  items: [emptyItem()] as AuditItem[],
  ugcEnabled: false,
  ugcText: '{\n  "scene": [0],\n  "method": [3],\n  "has_audit_team": 0,\n  "audit_desc": ""\n}',
  previewPics: '',
  previewVideos: '',
})

const referenceAppid = ref<string | null>(null)
const categories = ref<SettingCategory[]>([])
const pages = ref<string[]>([])
const loadingHelper = ref(false)
const saving = ref(false)

const appOptions = computed(() =>
  props.apps.filter((app) => app.status === 'active').map((app) => ({ label: `${app.nick_name || app.appid} (${app.appid})`, value: app.appid })),
)

const categoryOptions = computed(() =>
  categories.value.map((category) => ({
    label: `${category.first_name} / ${category.second_name}${category.audit_status === 3 ? '' : category.audit_status === 1 ? '（审核中）' : '（未通过）'}`,
    value: `${category.first}|${category.second}`,
  })),
)

const pageOptions = computed(() => pages.value.map((page) => ({ label: page, value: page })))

function categoryKey(item: AuditItem): string | null {
  if (!item.first_id || !item.second_id) return null
  return `${item.first_id}|${item.second_id}`
}

function applyCategory(item: AuditItem, value: string | null) {
  if (!value) return
  const found = categories.value.find((category) => `${category.first}|${category.second}` === value)
  if (!found) return
  item.first_class = found.first_name
  item.second_class = found.second_name
  item.first_id = found.first
  item.second_id = found.second
  item.third_class = ''
  item.third_id = 0
}

async function loadHelper(appid: string | null) {
  categories.value = []
  pages.value = []
  if (!appid) return
  loadingHelper.value = true
  try {
    const [categoryResult, pageResult] = await Promise.allSettled([
      api.get<SettingCategoriesResponse>(`/apps/${appid}/categories`, { silent: true }),
      api.get<{ items: string[] }>(`/apps/${appid}/pages`, { silent: true }),
    ])
    if (categoryResult.status === 'fulfilled') categories.value = categoryResult.value.categories || []
    if (pageResult.status === 'fulfilled') pages.value = pageResult.value.items || []
  } finally {
    loadingHelper.value = false
  }
}

function addItem() {
  if (form.items.length >= 5) {
    message.warning('审核项最多 5 个')
    return
  }
  form.items.push(emptyItem())
}

function removeItem(index: number) {
  if (form.items.length <= 1) return
  form.items.splice(index, 1)
}

function reset() {
  const profile = props.profile
  form.name = profile?.name || ''
  form.version_desc = profile?.version_desc || ''
  form.feedback_info = profile?.feedback_info || ''
  form.feedback_stuff = profile?.feedback_stuff || ''
  form.is_default = profile?.is_default || false
  form.items = profile?.item_list?.length
    ? profile.item_list.map((item) => ({
        address: item.address || '',
        tag: item.tag || '',
        first_class: item.first_class || '',
        second_class: item.second_class || '',
        third_class: item.third_class || '',
        first_id: item.first_id || 0,
        second_id: item.second_id || 0,
        third_id: item.third_id || 0,
        title: item.title || '',
      }))
    : [emptyItem()]
  const ugc = profile?.ugc_declare
  form.ugcEnabled = !!ugc && typeof ugc === 'object'
  form.ugcText = form.ugcEnabled ? prettyJSON(ugc) : '{\n  "scene": [0],\n  "method": [3],\n  "has_audit_team": 0,\n  "audit_desc": ""\n}'
  const preview = profile?.preview_info
  form.previewPics = Array.isArray(preview?.pic_id_list) ? preview.pic_id_list.join(', ') : ''
  form.previewVideos = Array.isArray(preview?.video_id_list) ? preview.video_id_list.join(', ') : ''
  referenceAppid.value = props.apps.find((app) => app.status === 'active')?.appid || null
  void loadHelper(referenceAppid.value)
}

watch(
  () => props.show,
  (show) => {
    if (show) reset()
  },
  { immediate: true },
)

watch(referenceAppid, (appid) => {
  if (props.show) void loadHelper(appid)
})

function parseIDs(text: string): string[] {
  return text
    .split(/[,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

async function save() {
  if (!form.name.trim()) {
    message.warning('请填写配置名称')
    return
  }
  if (form.items.length < 1 || form.items.length > 5) {
    message.warning('审核项数量必须在 1-5 之间')
    return
  }
  for (let i = 0; i < form.items.length; i += 1) {
    const item = form.items[i]
    if (!item.first_class.trim() || !item.first_id) {
      message.warning(`第 ${i + 1} 个审核项缺少一级类目`)
      return
    }
    if (!item.second_class.trim() || !item.second_id) {
      message.warning(`第 ${i + 1} 个审核项缺少二级类目`)
      return
    }
  }
  let ugc: any = null
  if (form.ugcEnabled) {
    try {
      ugc = JSON.parse(form.ugcText)
    } catch {
      message.error('UGC 声明不是合法的 JSON')
      return
    }
  }
  const pics = parseIDs(form.previewPics)
  const videos = parseIDs(form.previewVideos)
  const preview = pics.length || videos.length ? { pic_id_list: pics, video_id_list: videos } : null
  const payload = {
    name: form.name.trim(),
    item_list: form.items,
    version_desc: form.version_desc.trim(),
    feedback_info: form.feedback_info.trim(),
    feedback_stuff: form.feedback_stuff.trim(),
    ugc_declare: ugc,
    preview_info: preview,
    is_default: form.is_default,
  }
  saving.value = true
  try {
    if (props.profile?.id) {
      await api.put(`/audit-profiles/${props.profile.id}`, payload)
    } else {
      await api.post('/audit-profiles', payload)
    }
    message.success('审核配置已保存')
    visible.value = false
    emit('saved')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    :title="profile ? '编辑审核配置' : '新建审核配置'"
    :style="{ width: 'min(900px, 96vw)' }"
    :mask-closable="!saving"
    :closable="!saving"
  >
    <div class="modal-body">
      <n-form label-placement="top">
        <div class="top-grid">
          <n-form-item label="配置名称">
            <n-input v-model:value="form.name" maxlength="64" placeholder="例如 默认提审配置" />
          </n-form-item>
          <n-form-item label="设为默认配置">
            <n-switch v-model:value="form.is_default" />
          </n-form-item>
        </div>
        <n-form-item label="版本说明">
          <n-input v-model:value="form.version_desc" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="提交审核时的版本功能说明" />
        </n-form-item>

        <div class="section-head">
          <div>
            <div class="section-title">审核项</div>
          </div>
          <n-button secondary type="primary" @click="addItem">添加审核项</n-button>
        </div>

        <div class="helper-bar">
          <div class="helper-item">
            <span class="helper-label">参考小程序</span>
            <n-select
              v-model:value="referenceAppid"
              :options="appOptions"
              :loading="loadingHelper"
              filterable
              clearable
              placeholder="选择已授权小程序以填充类目/页面"
              style="min-width: 280px"
            />
          </div>
          <n-button quaternary :loading="loadingHelper" @click="loadHelper(referenceAppid)">刷新候选</n-button>
        </div>

        <n-card v-for="(item, index) in form.items" :key="index" size="small" class="item-card" :bordered="true">
          <template #header>
            <div class="item-head">
              <span>审核项 {{ index + 1 }}</span>
              <n-button text :disabled="form.items.length <= 1" @click="removeItem(index)">移除</n-button>
            </div>
          </template>
          <div class="grid-2">
            <n-form-item label="选择已配置类目">
              <n-select
                :value="categoryKey(item)"
                :options="categoryOptions"
                filterable
                clearable
                placeholder="选择后自动填充一级/二级类目"
                @update:value="(value: string | null) => applyCategory(item, value)"
              />
            </n-form-item>
            <n-form-item label="页面路径">
              <n-select
                v-model:value="item.address"
                :options="pageOptions"
                filterable
                tag
                clearable
                placeholder="选择或输入页面路径"
              />
            </n-form-item>
          </div>
          <div class="grid-4">
            <n-form-item label="一级类目名称">
              <n-input v-model:value="item.first_class" placeholder="文娱" />
            </n-form-item>
            <n-form-item label="一级类目 ID">
              <n-input-number v-model:value="item.first_id" :min="0" :show-button="false" style="width: 100%" />
            </n-form-item>
            <n-form-item label="二级类目名称">
              <n-input v-model:value="item.second_class" placeholder="资讯" />
            </n-form-item>
            <n-form-item label="二级类目 ID">
              <n-input-number v-model:value="item.second_id" :min="0" :show-button="false" style="width: 100%" />
            </n-form-item>
          </div>
          <div class="grid-4">
            <n-form-item label="三级类目名称（选填）">
              <n-input v-model:value="item.third_class" placeholder="选填" />
            </n-form-item>
            <n-form-item label="三级类目 ID（选填）">
              <n-input-number v-model:value="item.third_id" :min="0" :show-button="false" style="width: 100%" />
            </n-form-item>
            <n-form-item label="审核标签">
              <n-input v-model:value="item.tag" placeholder="用空格分隔，最多 10 个" />
            </n-form-item>
            <n-form-item label="页面标题">
              <n-input v-model:value="item.title" maxlength="32" placeholder="首页" />
            </n-form-item>
          </div>
        </n-card>

        <n-divider style="margin: 8px 0 18px" />

        <n-form-item label="涉及 UGC 内容（按需开启）">
          <n-switch v-model:value="form.ugcEnabled" />
        </n-form-item>
        <n-form-item v-if="form.ugcEnabled" label="UGC 信息安全声明（JSON）">
          <n-input v-model:value="form.ugcText" type="textarea" :autosize="{ minRows: 6, maxRows: 12 }" />
        </n-form-item>

        <div class="grid-2">
          <n-form-item label="预览图片 media_id（逗号分隔）">
            <n-input v-model:value="form.previewPics" placeholder="选填，media_id 有效期 3 天" />
          </n-form-item>
          <n-form-item label="预览视频 media_id（逗号分隔）">
            <n-input v-model:value="form.previewVideos" placeholder="选填，media_id 有效期 3 天" />
          </n-form-item>
        </div>
        <div class="grid-2">
          <n-form-item label="驳回反馈内容（选填）">
            <n-input v-model:value="form.feedback_info" placeholder="仅上一次审核被驳回时有效" />
          </n-form-item>
          <n-form-item label="反馈素材 media_id（竖线分隔，选填）">
            <n-input v-model:value="form.feedback_stuff" placeholder="media_id1|media_id2" />
          </n-form-item>
        </div>
      </n-form>
    </div>

    <template #footer>
      <div class="modal-footer">
        <n-button :disabled="saving" @click="visible = false">取消</n-button>
        <n-button type="primary" :loading="saving" @click="save">保存配置</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.modal-body {
  max-height: 70vh;
  overflow-y: auto;
  padding-right: 4px;
}

.top-grid {
  display: grid;
  grid-template-columns: 1fr 180px;
  gap: 16px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.section-title {
  font-size: 15px;
  font-weight: 650;
}



.helper-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px dashed var(--border);
  background: var(--surface-soft);
  margin-bottom: 14px;
}

.helper-item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.helper-label {
  font-size: 13px;
  font-weight: 560;
}



.item-card {
  margin-bottom: 14px;
}

.item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 620;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}

.grid-4 {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0 12px;
}



.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 860px) {
  .grid-4 {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 640px) {
  .top-grid,
  .grid-2,
  .grid-4 {
    grid-template-columns: 1fr;
  }
}
</style>

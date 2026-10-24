<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { api } from '@/api/client'
import type { Authorizer, CodeTemplate, DomainResponse, Job } from '@/types/api'
import { domainActions, domainFields, jobTypeOptions } from '@/utils/constants'
import { parseLines } from '@/utils/format'
import AuditSubmitForm from '@/components/AuditSubmitForm.vue'

const props = withDefaults(
  defineProps<{
    show: boolean
    action: string | null
    appids: string[]
    apps?: Authorizer[]
    initialDomains?: DomainResponse | null
    initialRollbackVersion?: string
  }>(),
  {
    apps: () => [],
    initialDomains: null,
    initialRollbackVersion: '',
  },
)

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'created', job: Job): void
}>()

const message = useMessage()
const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value),
})

const submitting = ref(false)
const loadingOptions = ref(false)
const templates = ref<CodeTemplate[]>([])
const confirmed = ref(false)
const auditForm = ref<InstanceType<typeof AuditSubmitForm> | null>(null)

const commitForm = reactive({
  templateId: null as number | null,
  userVersion: '',
  userDesc: '',
  extJson: '{\n  "appid": "{{appid}}"\n}',
})
const autoRelease = ref(false)
const rollbackVersion = ref('')
const grayPercentage = ref(10)
const supportDebuger = ref(false)
const supportExperiencer = ref(false)
const domainAction = ref<'add' | 'set' | 'delete'>('add')
const domainTexts = reactive<Record<string, string>>({
  requestdomain: '',
  wsrequestdomain: '',
  uploaddomain: '',
  downloaddomain: '',
  udpdomain: '',
  tcpdomain: '',
})
const wechatid = ref('')
const userstr = ref('')
const visitAction = ref<'open' | 'close'>('open')
const supportVersion = ref('')

const actionMeta = computed(() => jobTypeOptions.find((item) => item.value === props.action) || null)

const selectedAppNames = computed(() => {
  const map = new Map(props.apps.map((item) => [item.appid, item.nick_name || item.appid]))
  return props.appids.map((appid) => map.get(appid) || appid)
})

const summary = computed(() => {
  const names = selectedAppNames.value
  if (names.length <= 3) return names.join('、')
  return `${names.slice(0, 3).join('、')} 等 ${names.length} 个`
})

const templateOptions = computed(() =>
  templates.value.map((template) => ({
    label: `${template.user_version || '未命名版本'}${template.user_desc ? ` · ${template.user_desc}` : ''}`,
    value: template.id,
  })),
)

const selectedTemplate = computed(() => templates.value.find((item) => item.id === commitForm.templateId) || null)

const requiresConfirm = computed(() => {
  if (!props.action) return false
  if (['release', 'undo_audit', 'rollback', 'gray_release', 'cancel_gray_release', 'unbind_tester', 'modify_domain'].includes(props.action)) {
    return true
  }
  if (props.action === 'set_visit_status' && visitAction.value === 'close') return true
  if (props.action === 'modify_domain' && domainAction.value === 'set') return true
  return false
})



function resetForms() {
  confirmed.value = false
  submitting.value = false
  commitForm.templateId = null
  commitForm.userVersion = ''
  commitForm.userDesc = ''
  commitForm.extJson = '{\n  "appid": "{{appid}}"\n}'
  autoRelease.value = false
  rollbackVersion.value = props.initialRollbackVersion || ''
  grayPercentage.value = 10
  supportDebuger.value = false
  supportExperiencer.value = false
  domainAction.value = 'add'
  wechatid.value = ''
  userstr.value = ''
  visitAction.value = 'open'
  supportVersion.value = ''
  const initial = props.initialDomains as any
  for (const field of domainFields) {
    const values = initial?.[field.key] as string[] | null | undefined
    domainTexts[field.key] = (values || []).join('\n')
  }
}

async function init() {
  resetForms()
  if (!props.show || !props.action) return
  if (props.action === 'commit') {
    loadingOptions.value = true
    try {
      const data = await api.get<{ items: CodeTemplate[] }>('/templates')
      templates.value = data.items || []
      if (templates.value.length > 0) commitForm.templateId = templates.value[0].id
    } finally {
      loadingOptions.value = false
    }
  }
}

watch(
  () => [props.show, props.action, props.appids.length] as const,
  ([show]) => {
    if (show) void init()
  },
  { immediate: true },
)

function isJSON(text: string): boolean {
  try {
    JSON.parse(text)
    return true
  } catch {
    return false
  }
}

function renderedExtJSON(text: string): string {
  return text
    .replaceAll('{{appid}}', 'wx0000000000000000')
    .replaceAll('{{authorizer_appid}}', 'wx0000000000000000')
    .replaceAll('{{nick_name}}', '示例小程序')
    .replaceAll('{{user_version}}', commitForm.userVersion || '1.0.0')
    .replaceAll('{{now}}', '20260101120000')
}

function buildPayload(): any {
  switch (props.action) {
    case 'commit': {
      if (!selectedTemplate.value) throw new Error('请选择代码模板')
      if (!commitForm.userVersion.trim()) throw new Error('请填写版本号')
      const ext = commitForm.extJson.trim() || '{}'
      if (!isJSON(renderedExtJSON(ext))) throw new Error('ext_json 不是合法的 JSON')
      return {
        template_id: selectedTemplate.value.template_id,
        user_version: commitForm.userVersion.trim(),
        user_desc: commitForm.userDesc.trim(),
        ext_json: ext,
      }
    }
    case 'submit_audit': {
      const result = auditForm.value?.buildPayload()
      if (!result || 'error' in result) throw new Error(result?.error || '请填写提审信息')
      return result.payload
    }
    case 'rollback':
      return { app_version: rollbackVersion.value.trim() }
    case 'gray_release':
      if (grayPercentage.value === 0 && !supportDebuger.value && !supportExperiencer.value) {
        throw new Error('灰度比例为 0 时，至少选择项目成员或体验成员优先')
      }
      return {
        gray_percentage: grayPercentage.value,
        support_debuger_first: supportDebuger.value,
        support_experiencer_first: supportExperiencer.value,
      }
    case 'modify_domain': {
      const payload: Record<string, any> = { action: domainAction.value }
      let total = 0
      for (const field of domainFields) {
        const list = parseLines(domainTexts[field.key] || '')
        payload[field.key] = list
        total += list.length
      }
      if (total === 0) throw new Error('请至少填写一个域名')
      return payload
    }
    case 'bind_tester':
      if (!wechatid.value.trim()) throw new Error('请填写体验者微信号')
      return { wechatid: wechatid.value.trim() }
    case 'unbind_tester':
      if (!userstr.value.trim()) throw new Error('请填写体验者 userstr')
      return { userstr: userstr.value.trim() }
    case 'set_visit_status':
      return { action: visitAction.value }
    case 'set_support_version':
      if (!supportVersion.value.trim()) throw new Error('请填写基础库版本号')
      return { version: supportVersion.value.trim() }
    default:
      return {}
  }
}

async function submit() {
  if (!props.action || props.appids.length === 0) return
  if (requiresConfirm.value && !confirmed.value) {
    message.warning('请先确认风险提示')
    return
  }
  let payload: any
  try {
    payload = buildPayload()
  } catch (error) {
    message.error((error as Error).message)
    return
  }
  submitting.value = true
  try {
    const job = await api.post<Job>('/jobs', { type: props.action, appids: props.appids, payload })
    visible.value = false
    message.success('批量任务已创建')
    emit('created', job)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    :title="actionMeta?.label || '批量操作'"
    :style="{ width: 'min(720px, 94vw)' }"
    :mask-closable="!submitting"
    :closable="!submitting"
  >
    <div class="modal-body">
      <div class="app-summary">
        <n-tag type="primary" :bordered="false">{{ appids.length }} 个小程序</n-tag>
        <span>{{ summary }}</span>
      </div>

      <n-form label-placement="top" :show-feedback="true">
        <template v-if="action === 'commit'">
          <n-form-item label="代码模板">
            <n-select
              v-model:value="commitForm.templateId"
              :options="templateOptions"
              :loading="loadingOptions"
              filterable
              placeholder="请选择代码模板"
            />
          </n-form-item>
          <div class="form-grid">
            <n-form-item label="版本号">
              <n-input v-model:value="commitForm.userVersion" maxlength="64" placeholder="例如 1.2.0" />
            </n-form-item>
            <n-form-item label="版本描述">
              <n-input v-model:value="commitForm.userDesc" maxlength="120" placeholder="例如 修复已知问题" />
            </n-form-item>
          </div>
          <n-form-item label="ext_json">
            <n-input
              v-model:value="commitForm.extJson"
              type="textarea"
              :autosize="{ minRows: 6, maxRows: 12 }"
              placeholder='{"appid": "{{appid}}"}'
            />
          </n-form-item>
        </template>

        <template v-else-if="action === 'submit_audit'">
          <AuditSubmitForm ref="auditForm" :apps="apps" :default-appid="appids[0] || null" />
        </template>

        <template v-else-if="action === 'rollback'">
          <n-form-item label="目标版本（留空表示上一版本）">
            <n-input v-model:value="rollbackVersion" placeholder="例如 1.2.0，留空回退到上一线上版本" />
          </n-form-item>
        </template>

        <template v-else-if="action === 'gray_release'">
          <n-form-item label="灰度比例">
            <n-slider v-model:value="grayPercentage" :min="0" :max="100" :step="1" />
          </n-form-item>
          <div class="gray-value">{{ grayPercentage }}%</div>
          <div class="switch-row">
            <n-switch v-model:value="supportDebuger" />
            <span>项目成员优先灰度</span>
            <n-switch v-model:value="supportExperiencer" />
            <span>体验成员优先灰度</span>
          </div>
        </template>

        <template v-else-if="action === 'modify_domain'">
          <n-form-item label="操作方式">
            <n-select v-model:value="domainAction" :options="domainActions.map((item) => ({ label: item.label, value: item.value }))" />
          </n-form-item>
          <n-alert v-if="domainAction === 'set'" type="warning" :bordered="false" style="margin-bottom: 14px">
            覆盖会清空未填写的类型。
          </n-alert>
          <div class="domain-grid">
            <n-form-item v-for="field in domainFields" :key="field.key" :label="field.label">
              <n-input
                v-model:value="domainTexts[field.key]"
                type="textarea"
                :autosize="{ minRows: 2, maxRows: 5 }"
                :placeholder="field.placeholder"
              />
            </n-form-item>
          </div>
        </template>

        <template v-else-if="action === 'bind_tester'">
          <n-form-item label="体验者微信号">
            <n-input v-model:value="wechatid" placeholder="输入体验者的微信号" />
          </n-form-item>
        </template>

        <template v-else-if="action === 'unbind_tester'">
          <n-form-item label="体验者 userstr">
            <n-input v-model:value="userstr" placeholder="从对应小程序的体验者列表中获取" />
          </n-form-item>
        </template>

        <template v-else-if="action === 'set_visit_status'">
          <n-form-item label="服务状态">
            <n-radio-group v-model:value="visitAction">
              <n-radio-button value="open">开启服务</n-radio-button>
              <n-radio-button value="close">暂停服务</n-radio-button>
            </n-radio-group>
          </n-form-item>
        </template>

        <template v-else-if="action === 'set_support_version'">
          <n-form-item label="最低基础库版本">
            <n-input v-model:value="supportVersion" placeholder="例如 3.0.0" />
          </n-form-item>
        </template>

        <div v-if="requiresConfirm" class="confirm-box">
          <n-checkbox v-model:checked="confirmed">我已知晓风险，确认执行</n-checkbox>
        </div>
      </n-form>
    </div>

    <template #footer>
      <div class="modal-footer">
        <n-button :disabled="submitting" @click="visible = false">取消</n-button>
        <n-button
          type="primary"
          :loading="submitting"
          :disabled="appids.length === 0 || (requiresConfirm && !confirmed)"
          @click="submit"
        >
          提交
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.modal-body {
  max-height: 68vh;
  overflow-y: auto;
  padding-right: 4px;
}

.app-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
}

.summary-label {
  color: var(--text);
  font-weight: 560;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}

.domain-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}



.profile-title {
  font-weight: 620;
  font-size: 14px;
}

.profile-meta {
  color: var(--text-muted);
  font-size: 12px;
  margin-top: 4px;
}

.profile-items {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}



.gray-value {
  text-align: center;
  font-size: 22px;
  font-weight: 650;
  color: var(--primary);
  margin-top: -8px;
  margin-bottom: 12px;
}

.switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  font-size: 13px;
  margin-bottom: 16px;
}

.confirm-box {
  margin-top: 8px;
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 640px) {
  .form-grid,
  .domain-grid {
    grid-template-columns: 1fr;
  }
}
</style>

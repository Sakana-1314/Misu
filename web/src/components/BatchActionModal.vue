<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { CheckmarkCircle, CodeSlashOutline, WarningOutline } from '@vicons/ionicons5'
import { api } from '@/api/client'
import type { Authorizer, CodeTemplate, DomainResponse, Job, Paged } from '@/types/api'
import { domainActions, domainFields, jobTypeOptions } from '@/utils/constants'
import { fromUnix, parseLines } from '@/utils/format'
import AuditSubmitForm from '@/components/AuditSubmitForm.vue'
import AppAvatar from '@/components/AppAvatar.vue'

const props = withDefaults(
  defineProps<{
    show: boolean
    action: string | null
    appids: string[]
    apps?: Authorizer[]
    initialDomains?: DomainResponse | null
    initialRollbackVersion?: string
    initialTemplateId?: number | null
  }>(),
  {
    apps: () => [],
    initialDomains: null,
    initialRollbackVersion: '',
    initialTemplateId: null,
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

// When opened without a selection (e.g. from the template library) the
// operator picks the target mini programs here.
const pickApps = computed(() => props.appids.length === 0)
const pickedAppids = ref<string[]>([])
const allApps = ref<Authorizer[]>([])

const commitForm = reactive({
  templateId: null as number | null,
  userVersion: '',
  userDesc: '',
  extJson: '{\n  "appid": "{{appid}}"\n}',
})
const rollbackVersion = ref('')
const grayPercentage = ref(10)
const supportDebuger = ref(true)
const supportExperiencer = ref(true)
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

const targetAppids = computed(() => (pickApps.value ? pickedAppids.value : props.appids))
const appMap = computed(() => new Map([...allApps.value, ...props.apps].map((item) => [item.appid, item])))
const targetApps = computed(() => targetAppids.value.map((appid) => appMap.value.get(appid) || ({ appid, nick_name: appid } as Authorizer)))
const appPickerOptions = computed(() =>
  allApps.value
    .filter((app) => app.status === 'active')
    .map((app) => ({ label: app.nick_name || app.appid, value: app.appid })),
)

const sortedTemplates = computed(() => [...templates.value].sort((a, b) => b.create_time - a.create_time))
const selectedTemplate = computed(() => templates.value.find((item) => item.id === commitForm.templateId) || null)

const confirmText: Record<string, string> = {
  release: '我确认要把新版本发布给所有用户',
  undo_audit: '我知道撤回次数有限，确认撤回',
  rollback: '我确认要把线上版本回退',
  gray_release: '我确认要开始分阶段发布',
  cancel_gray_release: '我确认停止分阶段发布',
  modify_domain: '我确认修改服务器域名',
  unbind_tester: '我确认移除该体验成员',
  set_visit_status: '我确认暂停服务，用户将无法打开小程序',
}

const requiresConfirm = computed(() => {
  if (!props.action) return false
  if (props.action === 'set_visit_status') return visitAction.value === 'close'
  return Boolean(actionMeta.value?.dangerous)
})

const submitLabel = computed(() => {
  switch (props.action) {
    case 'commit':
      return '开始上传'
    case 'submit_audit':
      return '提交审核'
    case 'release':
      return '发布上线'
    case 'preflight':
      return '开始检查'
    default:
      return '确认执行'
  }
})

function pickTemplate(template: CodeTemplate) {
  commitForm.templateId = template.id
  commitForm.userVersion = template.user_version || ''
  commitForm.userDesc = template.user_desc || ''
}

function resetForms() {
  confirmed.value = false
  submitting.value = false
  pickedAppids.value = []
  commitForm.templateId = null
  commitForm.userVersion = ''
  commitForm.userDesc = ''
  commitForm.extJson = '{\n  "appid": "{{appid}}"\n}'
  rollbackVersion.value = props.initialRollbackVersion || ''
  grayPercentage.value = 10
  supportDebuger.value = true
  supportExperiencer.value = true
  domainAction.value = 'add'
  wechatid.value = ''
  userstr.value = ''
  visitAction.value = 'open'
  supportVersion.value = ''
  for (const field of domainFields) domainTexts[field.key] = ''
}

async function init() {
  resetForms()
  if (!props.show || !props.action) return
  if (pickApps.value && allApps.value.length === 0) {
    api
      .get<Paged<Authorizer>>('/authorizers', { params: { page: 1, page_size: 100, status: 'active' }, silent: true })
      .then((data) => (allApps.value = data.items || []))
      .catch(() => undefined)
  }
  if (props.action === 'commit') {
    loadingOptions.value = true
    try {
      const data = await api.get<{ items: CodeTemplate[] }>('/templates')
      templates.value = data.items || []
      const preset = templates.value.find((item) => item.id === props.initialTemplateId)
      const first = preset || sortedTemplates.value[0]
      if (first) pickTemplate(first)
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

// Replacing starts from the current list so the operator edits instead of retyping.
watch(domainAction, (value) => {
  const initial = props.initialDomains as Record<string, string[] | null | undefined> | null
  if (value !== 'set' || !initial) return
  if (domainFields.some((field) => domainTexts[field.key].trim())) return
  for (const field of domainFields) domainTexts[field.key] = (initial[field.key] || []).join('\n')
})

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

const extJsonValid = computed(() => isJSON(renderedExtJSON(commitForm.extJson.trim() || '{}')))

function buildPayload(): any {
  switch (props.action) {
    case 'commit': {
      if (!selectedTemplate.value) throw new Error('请选择要上传的代码版本')
      if (!commitForm.userVersion.trim()) throw new Error('请填写版本号')
      if (!extJsonValid.value) throw new Error('高级设置中的配置不是合法的 JSON')
      return {
        template_id: selectedTemplate.value.template_id,
        user_version: commitForm.userVersion.trim(),
        user_desc: commitForm.userDesc.trim(),
        ext_json: commitForm.extJson.trim() || '{}',
      }
    }
    case 'submit_audit': {
      const result = auditForm.value?.buildPayload()
      if (!result || 'error' in result) throw new Error(result?.error || '请填写审核信息')
      return result.payload
    }
    case 'rollback':
      return { app_version: rollbackVersion.value.trim() }
    case 'gray_release':
      if (grayPercentage.value === 0 && !supportDebuger.value && !supportExperiencer.value) {
        throw new Error('比例为 0 时，至少让项目成员或体验成员先用上')
      }
      return {
        gray_percentage: grayPercentage.value,
        support_debuger_first: supportDebuger.value,
        support_experiencer_first: supportExperiencer.value,
      }
    case 'modify_domain': {
      const payload: Record<string, any> = { action: domainAction.value }
      let count = 0
      for (const field of domainFields) {
        const list = parseLines(domainTexts[field.key] || '')
        payload[field.key] = list
        count += list.length
      }
      if (count === 0) throw new Error('请至少填写一个域名')
      return payload
    }
    case 'bind_tester':
      if (!wechatid.value.trim()) throw new Error('请填写微信号')
      return { wechatid: wechatid.value.trim() }
    case 'unbind_tester':
      if (!userstr.value.trim()) throw new Error('缺少体验成员标识')
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
  if (!props.action) return
  if (targetAppids.value.length === 0) {
    message.warning('请选择小程序')
    return
  }
  if (requiresConfirm.value && !confirmed.value) {
    message.warning('请先勾选确认')
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
    const job = await api.post<Job>('/jobs', { type: props.action, appids: targetAppids.value, payload })
    visible.value = false
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
    :style="{ width: action === 'submit_audit' ? 'min(760px, 96vw)' : 'min(620px, 96vw)' }"
    :mask-closable="!submitting"
    :closable="!submitting"
    :segmented="{ content: true, footer: 'soft' }"
  >
    <template #header>
      <div class="modal-title">{{ actionMeta?.label || '批量操作' }}</div>
      <div class="modal-desc">{{ actionMeta?.description }}</div>
    </template>

    <div class="modal-body">
      <!-- Targets -->
      <div v-if="pickApps" class="field">
        <div class="field-label">选择小程序</div>
        <n-select
          v-model:value="pickedAppids"
          multiple
          filterable
          max-tag-count="responsive"
          :options="appPickerOptions"
          placeholder="选择要上传的小程序"
        />
      </div>
      <div v-else class="targets">
        <div class="avatars">
          <AppAvatar
            v-for="app in targetApps.slice(0, 5)"
            :key="app.appid"
            :src="app.head_img"
            :name="app.nick_name"
            :appid="app.appid"
            :size="26"
            class="stack-avatar"
          />
        </div>
        <span class="targets-text">
          <template v-if="targetApps.length === 1">{{ targetApps[0].nick_name || targetApps[0].appid }}</template>
          <template v-else>
            {{ targetApps.slice(0, 2).map((app) => app.nick_name || app.appid).join('、') }} 等
            <b>{{ targetApps.length }}</b> 个小程序
          </template>
        </span>
      </div>

      <!-- Upload code -->
      <template v-if="action === 'commit'">
        <div class="field">
          <div class="field-label">选择代码版本</div>
          <n-spin :show="loadingOptions">
            <div v-if="sortedTemplates.length" class="template-list">
              <button
                v-for="template in sortedTemplates"
                :key="template.id"
                type="button"
                class="template-option"
                :class="{ selected: template.id === commitForm.templateId }"
                @click="pickTemplate(template)"
              >
                <n-icon class="template-check" :size="18">
                  <CheckmarkCircle v-if="template.id === commitForm.templateId" />
                  <CodeSlashOutline v-else />
                </n-icon>
                <div class="template-text">
                  <div class="template-version">{{ template.user_version || '未命名版本' }}</div>
                  <div class="template-desc">{{ template.user_desc || '无描述' }}</div>
                </div>
                <span class="template-time">{{ fromUnix(template.create_time).slice(0, 10) }}</span>
              </button>
            </div>
            <div v-else-if="!loadingOptions" class="empty-box">
              模板库还是空的。请先在「代码模板」中把开发者工具上传的草稿加入模板库。
            </div>
          </n-spin>
        </div>
        <div class="grid-2">
          <div class="field">
            <div class="field-label">版本号</div>
            <n-input v-model:value="commitForm.userVersion" maxlength="64" placeholder="例如 1.2.0" />
          </div>
          <div class="field">
            <div class="field-label">版本说明</div>
            <n-input v-model:value="commitForm.userDesc" maxlength="120" placeholder="例如 修复已知问题" />
          </div>
        </div>
        <n-collapse class="advanced" arrow-placement="left">
          <n-collapse-item name="ext" title="高级设置">
            <div class="hint" style="margin-bottom: 8px">
              第三方平台自定义配置（ext.json）。可使用变量
              <code v-pre>{{appid}}</code>、<code v-pre>{{nick_name}}</code>、<code v-pre>{{user_version}}</code>，
              上传时会替换成每个小程序自己的值。不确定时保持默认即可。
            </div>
            <n-input
              v-model:value="commitForm.extJson"
              type="textarea"
              class="mono"
              :status="extJsonValid ? undefined : 'error'"
              :autosize="{ minRows: 4, maxRows: 10 }"
            />
            <div v-if="!extJsonValid" class="field-error">JSON 格式不正确</div>
          </n-collapse-item>
        </n-collapse>
      </template>

      <!-- Submit for review -->
      <template v-else-if="action === 'submit_audit'">
        <AuditSubmitForm ref="auditForm" :apps="targetApps" :default-appid="targetAppids[0] || null" />
      </template>

      <template v-else-if="action === 'release'">
        <div class="info-box">
          发布的是<b>已通过审核</b>的版本。发布后所有用户打开小程序即会使用新版本；若尚未通过审核，该小程序会被跳过。
        </div>
      </template>

      <template v-else-if="action === 'undo_audit'">
        <div class="info-box">撤回后版本回到体验版状态，可修改后重新提交。微信限制每个小程序每天只能撤回 1 次、每月 10 次。</div>
      </template>

      <template v-else-if="action === 'cancel_gray_release'">
        <div class="info-box">停止后，尚未更新的用户将继续使用当前线上版本。</div>
      </template>

      <template v-else-if="action === 'preflight'">
        <div class="info-box">
          将逐个检查：审核状态、已配置类目、服务器域名、隐私接口声明等，结果会显示在任务中心。检查不会修改任何设置。
        </div>
      </template>

      <template v-else-if="action === 'rollback'">
        <div class="field">
          <div class="field-label">回退到</div>
          <n-input v-model:value="rollbackVersion" placeholder="留空则回退到上一个线上版本" />
          <div class="hint field-hint">只能回退到最近发布过的版本。可在小程序详情页的「历史版本」中查看。</div>
        </div>
      </template>

      <template v-else-if="action === 'gray_release'">
        <div class="field">
          <div class="field-label">先让多少用户用上新版本</div>
          <div class="gray-row">
            <n-slider v-model:value="grayPercentage" :min="0" :max="100" :step="1" :marks="{ 1: '1%', 10: '10%', 50: '50%', 100: '100%' }" />
            <span class="gray-value">{{ grayPercentage }}%</span>
          </div>
        </div>
        <div class="check-list">
          <n-checkbox v-model:checked="supportDebuger">项目成员优先使用新版本</n-checkbox>
          <n-checkbox v-model:checked="supportExperiencer">体验成员优先使用新版本</n-checkbox>
        </div>
      </template>

      <template v-else-if="action === 'modify_domain'">
        <div class="field">
          <div class="field-label">修改方式</div>
          <n-radio-group v-model:value="domainAction">
            <n-radio-button v-for="item in domainActions" :key="item.value" :value="item.value">{{ item.label }}</n-radio-button>
          </n-radio-group>
          <div class="hint field-hint" :class="{ warn: domainAction === 'set' }">
            {{ domainActions.find((item) => item.value === domainAction)?.hint }}
          </div>
        </div>
        <div class="grid-2">
          <div v-for="field in domainFields" :key="field.key" class="field">
            <div class="field-label">
              {{ field.label }} <span class="api-name">{{ field.api }}</span>
            </div>
            <n-input
              v-model:value="domainTexts[field.key]"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 5 }"
              :placeholder="field.placeholder"
            />
          </div>
        </div>
        <div class="hint">每行一个域名。微信限制每个小程序每月最多修改 50 次。</div>
      </template>

      <template v-else-if="action === 'bind_tester'">
        <div class="field">
          <div class="field-label">微信号</div>
          <n-input v-model:value="wechatid" placeholder="对方的微信号（不是昵称）" />
          <div class="hint field-hint">添加后对方即可扫描体验版二维码打开小程序。</div>
        </div>
      </template>

      <template v-else-if="action === 'unbind_tester'">
        <div class="field">
          <div class="field-label">体验成员标识</div>
          <n-input v-model:value="userstr" placeholder="userstr" />
        </div>
      </template>

      <template v-else-if="action === 'set_visit_status'">
        <div class="choice-grid">
          <button type="button" class="choice" :class="{ selected: visitAction === 'open' }" @click="visitAction = 'open'">
            <div class="choice-title">正常服务</div>
            <div class="choice-desc">用户可以正常打开和使用小程序</div>
          </button>
          <button type="button" class="choice danger" :class="{ selected: visitAction === 'close' }" @click="visitAction = 'close'">
            <div class="choice-title">暂停服务</div>
            <div class="choice-desc">用户打开时会看到暂停提示</div>
          </button>
        </div>
      </template>

      <template v-else-if="action === 'set_support_version'">
        <div class="field">
          <div class="field-label">最低基础库版本</div>
          <n-input v-model:value="supportVersion" placeholder="例如 3.0.0" />
          <div class="hint field-hint">微信版本过低（基础库低于该版本）的用户打开时会被提示升级微信。</div>
        </div>
      </template>

      <div v-if="requiresConfirm" class="confirm-box">
        <n-icon class="confirm-icon"><WarningOutline /></n-icon>
        <n-checkbox v-model:checked="confirmed">{{ confirmText[action || ''] || '我已了解影响，确认执行' }}</n-checkbox>
      </div>
    </div>

    <template #footer>
      <div class="modal-footer">
        <span class="hint">任务会在后台执行，可在任务中心查看进度</span>
        <div class="footer-actions">
          <n-button :disabled="submitting" @click="visible = false">取消</n-button>
          <n-button
            type="primary"
            :loading="submitting"
            :disabled="targetAppids.length === 0 || (requiresConfirm && !confirmed)"
            @click="submit"
          >
            {{ submitLabel }}
          </n-button>
        </div>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.modal-title {
  font-size: 16px;
  font-weight: 680;
}

.modal-desc {
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 12.5px;
  font-weight: 400;
}

.modal-body {
  max-height: 66vh;
  overflow-y: auto;
  margin-right: -8px;
  padding-right: 8px;
}

.targets {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  padding: 9px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  font-size: 13px;
}

.avatars {
  display: flex;
}

.stack-avatar {
  box-shadow: 0 0 0 2px var(--surface-soft);
}

.stack-avatar + .stack-avatar {
  margin-left: -8px;
}

.targets-text b {
  color: var(--primary);
}

.field {
  margin-bottom: 16px;
}

.field-label {
  margin-bottom: 7px;
  font-size: 13px;
  font-weight: 600;
}

.field-hint {
  margin-top: 6px;
}

.field-hint.warn {
  color: var(--warning);
}

.field-error {
  margin-top: 4px;
  color: var(--danger);
  font-size: 12px;
}

.api-name {
  margin-left: 4px;
  color: var(--text-faint);
  font-weight: 400;
  font-size: 11px;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 14px;
}

.template-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 232px;
  overflow-y: auto;
}

.template-option {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.template-option:hover {
  border-color: #b9cbf7;
}

.template-option.selected {
  border-color: var(--primary);
  background: var(--primary-faint);
}

.template-check {
  color: var(--text-faint);
}

.selected .template-check {
  color: var(--primary);
}

.template-text {
  flex: 1;
  min-width: 0;
}

.template-version {
  font-size: 13.5px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}

.template-desc {
  margin-top: 1px;
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.template-time {
  color: var(--text-faint);
  font-size: 11.5px;
}

.empty-box,
.info-box {
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.7;
}

.info-box b {
  color: var(--text);
}

.advanced {
  margin-top: -4px;
}

.advanced code {
  padding: 0 4px;
  border-radius: 4px;
  background: var(--surface-soft);
  font-size: 11.5px;
}

.gray-row {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 4px 4px 16px;
}

.gray-value {
  min-width: 52px;
  text-align: right;
  font-size: 22px;
  font-weight: 700;
  color: var(--primary);
  font-variant-numeric: tabular-nums;
}

.check-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.choice-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.choice {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.choice.selected {
  border-color: var(--primary);
  background: var(--primary-faint);
}

.choice.danger.selected {
  border-color: var(--danger);
  background: var(--danger-soft);
}

.choice-title {
  font-weight: 650;
  font-size: 14px;
}

.choice-desc {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
}

.confirm-box {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 16px;
  padding: 11px 14px;
  border-radius: var(--radius-md);
  background: var(--warning-soft);
}

.confirm-icon {
  color: var(--warning);
  font-size: 18px;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.footer-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

@media (max-width: 640px) {
  .grid-2,
  .choice-grid {
    grid-template-columns: 1fr;
  }

  .modal-footer .hint {
    display: none;
  }
}
</style>

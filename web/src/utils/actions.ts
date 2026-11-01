import { h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, useNotification, type DropdownOption } from 'naive-ui'
import type { Job } from '@/types/api'
import { jobGroupLabels, jobTypeOptions, type JobTypeOption } from '@/utils/constants'

function renderOption(option: JobTypeOption) {
  return () =>
    h('div', { class: 'action-option' }, [
      h('div', { class: 'action-option-label' }, option.label),
      h('div', { class: 'action-option-desc' }, option.description),
    ])
}

// actionMenu groups the job types into a dropdown, each entry with a one-line
// explanation so the menu reads as tasks rather than API names.
export function actionMenu(exclude: string[] = []): DropdownOption[] {
  const groups: DropdownOption[] = []
  for (const group of Object.keys(jobGroupLabels) as JobTypeOption['group'][]) {
    const children = jobTypeOptions
      .filter((item) => item.group === group && !item.hidden && !exclude.includes(item.value))
      .map((item) => ({ key: item.value, label: renderOption(item) }))
    if (children.length === 0) continue
    if (groups.length > 0) groups.push({ type: 'divider', key: `divider-${group}` })
    groups.push({ type: 'group', key: group, label: jobGroupLabels[group], children })
  }
  return groups
}

// useJobCreated tells the operator a background task started and offers a
// shortcut to its progress.
export function useJobCreated() {
  const notification = useNotification()
  const router = useRouter()
  return (job: Job) => {
    const n = notification.success({
      title: '任务已开始',
      content: `${job.title || '批量任务'}，正在后台执行。`,
      duration: 6000,
      action: () =>
        h(
          NButton,
          {
            text: true,
            type: 'primary',
            onClick: () => {
              n.destroy()
              void router.push({ name: 'jobs', query: { id: String(job.id) } })
            },
          },
          { default: () => '查看进度' },
        ),
    })
  }
}

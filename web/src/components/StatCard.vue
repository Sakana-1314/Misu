<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string | number
    tone?: 'primary' | 'slate'
  }>(),
  { tone: 'primary' },
)

const toneClass: Record<string, string> = {
  primary: 'tone-primary',
  slate: 'tone-slate',
}
</script>

<template>
  <div class="stat-card surface-card">
    <div class="top">
      <div class="icon" :class="toneClass[tone]">
        <slot name="icon" />
      </div>
      <span class="label">{{ label }}</span>
    </div>
    <div class="metric-value">{{ value }}</div>
  </div>
</template>

<style scoped>
.stat-card {
  padding: 13px 15px;
  min-height: 90px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.top {
  display: flex;
  align-items: center;
  gap: 10px;
}

.icon {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
}

.tone-primary {
  background: rgba(37, 99, 235, 0.1);
  color: var(--primary);
}

.tone-slate {
  background: rgba(100, 116, 139, 0.1);
  color: #64748b;
}

.label {
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 560;
}

.metric-value {
  margin-top: 10px;
}
</style>

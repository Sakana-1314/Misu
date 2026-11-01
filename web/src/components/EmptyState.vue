<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon?: Component; title: string; description?: string; compact?: boolean }>()
</script>

<template>
  <div class="empty" :class="{ compact }">
    <div v-if="icon" class="empty-icon">
      <n-icon :size="compact ? 20 : 26"><component :is="icon" /></n-icon>
    </div>
    <div class="empty-title">{{ title }}</div>
    <div v-if="description" class="empty-desc">{{ description }}</div>
    <div v-if="$slots.default" class="empty-actions"><slot /></div>
  </div>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 44px 20px;
}

.empty.compact {
  padding: 24px 16px;
}

.empty-icon {
  width: 52px;
  height: 52px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 14px;
  border-radius: 16px;
  background: var(--primary-soft);
  color: var(--primary);
}

.compact .empty-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  margin-bottom: 10px;
}

.empty-title {
  font-size: 14px;
  font-weight: 620;
}

.empty-desc {
  max-width: 420px;
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.7;
}

.empty-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  flex-wrap: wrap;
  justify-content: center;
}
</style>

<script setup lang="ts">
import { computed } from 'vue'
import type { DayUsage } from '../domain/types'
import { formatKwh } from '../domain/format'

const props = defineProps<{
  days: DayUsage[]
  average: number
}>()

const maxValue = computed(() =>
  Math.max(0.1, ...props.days.map((day) => day.kwh), props.average),
)

const averagePct = computed(() => Math.min(94, (props.average / maxValue.value) * 100))

function height(day: DayUsage): string {
  return `${Math.max(2, (day.kwh / maxValue.value) * 100)}%`
}

function label(day: DayUsage): string {
  return `${day.label}, ${formatKwh(day.kwh)} kWh${day.isToday ? ' (hari ini)' : ''}`
}
</script>

<template>
  <div class="usage-chart">
    <div class="usage-plot">
      <div
        v-if="average > 0"
        class="usage-average"
        :style="{ bottom: `${averagePct}%` }"
        aria-hidden="true"
      >
        <span>rata-rata</span>
      </div>

      <div class="usage-bars">
        <div
          v-for="day in days"
          :key="day.key"
          class="usage-bar"
          :class="{ 'usage-bar--today': day.isToday }"
          :title="label(day)"
          :aria-label="label(day)"
          role="img"
        >
          <span class="usage-bar-fill" :style="{ height: height(day) }" />
        </div>
      </div>
    </div>

    <div class="usage-labels" aria-hidden="true">
      <span
        v-for="day in days"
        :key="day.key"
        class="usage-label"
        :class="{ 'usage-label--today': day.isToday }"
      >
        {{ day.isToday ? 'Hari ini' : day.label }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DayUsage } from '../domain/types'
import { formatKwh } from '../domain/format'
import { nearestIndex } from '../domain/chart.ts'

const props = defineProps<{
  days: DayUsage[]
  average: number
}>()

const maxValue = computed(() =>
  Math.max(0.1, ...props.days.map((day) => day.kwh), props.average),
)

const averagePct = computed(() => Math.min(94, (props.average / maxValue.value) * 100))

const active = ref<number | null>(null)
const activeDay = computed(() => (active.value == null ? null : (props.days[active.value] ?? null)))

function onPointer(event: PointerEvent) {
  const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
  if (box.width <= 0 || props.days.length === 0) return
  const centers = props.days.map((_, index) => (index + 0.5) / props.days.length)
  active.value = nearestIndex((event.clientX - box.left) / box.width, centers)
}

function height(day: DayUsage): string {
  return `${Math.max(2, (day.kwh / maxValue.value) * 100)}%`
}

function label(day: DayUsage): string {
  return `${day.label}, ${formatKwh(day.kwh)} kWh${day.isToday ? ' (hari ini)' : ''}`
}
</script>

<template>
  <div class="usage-chart">
    <div
      class="usage-plot"
      @pointermove="onPointer"
      @pointerdown="onPointer"
      @pointerleave="active = null"
    >
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
          v-for="(day, index) in days"
          :key="day.key"
          class="usage-bar"
          :class="{ 'usage-bar--today': day.isToday, 'usage-bar--active': index === active }"
          :title="label(day)"
          :aria-label="label(day)"
          tabindex="0"
          role="img"
          @focus="active = index"
        >
          <span class="usage-bar-fill" :style="{ height: height(day) }" />
        </div>
      </div>

      <div
        v-if="activeDay && active != null"
        class="usage-tip"
        :style="{ left: `${((active + 0.5) / days.length) * 100}%` }"
        aria-hidden="true"
      >
        <span>{{ activeDay.label }}</span>
        <strong>{{ formatKwh(activeDay.kwh) }} kWh</strong>
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

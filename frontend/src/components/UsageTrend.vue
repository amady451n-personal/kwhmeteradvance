<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatKwh } from '../domain/format'
import { nearestIndex } from '../domain/chart.ts'

export interface TrendPoint {
  key: string
  label: string
  kwh: number
  ms: number
}

const props = withDefaults(
  defineProps<{
    points: TrendPoint[]
    label?: string
    unit?: string
  }>(),
  { label: 'Tren pemakaian', unit: 'kWh/hari' },
)

const W = 320
const H = 112
const PAD = 8

const active = ref<number | null>(null)

const maxValue = computed(() => Math.max(0.1, ...props.points.map((point) => point.kwh)))
const firstMs = computed(() => props.points[0]?.ms ?? 0)
const spanMs = computed(() => (props.points[props.points.length - 1]?.ms ?? 0) - firstMs.value)

function x(index: number): number {
  if (props.points.length < 2) return W / 2
  if (spanMs.value <= 0) return PAD + (index / (props.points.length - 1)) * (W - PAD * 2)
  return PAD + ((props.points[index].ms - firstMs.value) / spanMs.value) * (W - PAD * 2)
}

function y(value: number): number {
  return H - PAD - (value / maxValue.value) * (H - PAD * 2)
}

const line = computed(() =>
  props.points.map((point, index) => `${x(index).toFixed(1)},${y(point.kwh).toFixed(1)}`).join(' '),
)

const last = computed(() => props.points[props.points.length - 1])

const activePoint = computed(() =>
  active.value == null ? null : (props.points[active.value] ?? null),
)

const xRatios = computed(() => props.points.map((_, index) => x(index) / W))

function onPointer(event: PointerEvent) {
  const box = (event.currentTarget as HTMLElement).getBoundingClientRect()
  if (box.width <= 0) return
  active.value = nearestIndex((event.clientX - box.left) / box.width, xRatios.value)
}

const tipStyle = computed(() => {
  if (active.value == null) return {}
  const xPct = Math.min(86, Math.max(14, (x(active.value) / W) * 100))
  return {
    left: `${xPct}%`,
    top: `${(y(props.points[active.value].kwh) / H) * 100}%`,
  }
})

function toggle(index: number) {
  active.value = active.value === index ? null : index
}
</script>

<template>
  <div class="trend-chart">
    <div
      class="trend-plot"
      @pointermove="onPointer"
      @pointerdown="onPointer"
      @pointerleave="active = null"
    >
      <svg
        :viewBox="`0 0 ${W} ${H}`"
        preserveAspectRatio="none"
        role="img"
        :aria-label="`${label}: ${points.map((p) => `${p.label} ${formatKwh(p.kwh)} ${unit}`).join(', ')}`"
      >
        <line class="trend-base" :x1="PAD" :y1="H - PAD" :x2="W - PAD" :y2="H - PAD" vector-effect="non-scaling-stroke" />
        <polyline class="trend-line" :points="line" vector-effect="non-scaling-stroke" />
      </svg>

      <span
        v-for="(point, index) in points"
        :key="point.key"
        class="trend-dot"
        :class="{ 'trend-dot--last': index === points.length - 1, 'trend-dot--active': index === active }"
        :style="{ left: `${(x(index) / W) * 100}%`, top: `${(y(point.kwh) / H) * 100}%` }"
        :title="`${point.label}: ${formatKwh(point.kwh)} ${unit}`"
        :aria-label="`${point.label}: ${formatKwh(point.kwh)} ${unit}`"
        tabindex="0"
        @focus="active = index"
        @click="toggle(index)"
      />

      <div v-if="activePoint" class="trend-tip" :style="tipStyle" aria-hidden="true">
        <span>{{ activePoint.label }}</span>
        <strong>{{ formatKwh(activePoint.kwh) }} {{ unit }}</strong>
      </div>
    </div>

    <div class="trend-foot">
      <span>{{ points[0]?.label }}</span>
      <strong class="tabular">
        {{ last ? `${formatKwh(last.kwh)} ${unit}` : '-' }}
      </strong>
      <span>{{ last?.label }}</span>
    </div>
  </div>
</template>
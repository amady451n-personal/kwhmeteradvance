<script setup lang="ts">
import { computed } from 'vue'
import { formatKwh } from '../domain/format'

export interface TrendPoint {
  key: string
  label: string
  kwh: number
}

const props = defineProps<{ points: TrendPoint[] }>()

const W = 320
const H = 112
const PAD = 8

const maxValue = computed(() => Math.max(0.1, ...props.points.map((point) => point.kwh)))

function x(index: number): number {
  if (props.points.length < 2) return W / 2
  return PAD + (index / (props.points.length - 1)) * (W - PAD * 2)
}

function y(value: number): number {
  return H - PAD - (value / maxValue.value) * (H - PAD * 2)
}

const line = computed(() =>
  props.points.map((point, index) => `${x(index).toFixed(1)},${y(point.kwh).toFixed(1)}`).join(' '),
)

const last = computed(() => props.points[props.points.length - 1])
</script>

<template>
  <div class="trend-chart">
    <div class="trend-plot">
      <svg
        :viewBox="`0 0 ${W} ${H}`"
        preserveAspectRatio="none"
        role="img"
        :aria-label="`Tren pemakaian: ${points.map((p) => `${p.label} ${formatKwh(p.kwh)} kWh per hari`).join(', ')}`"
      >
        <line class="trend-base" :x1="PAD" :y1="H - PAD" :x2="W - PAD" :y2="H - PAD" vector-effect="non-scaling-stroke" />
        <polyline class="trend-line" :points="line" vector-effect="non-scaling-stroke" />
      </svg>

      <span
        v-for="(point, index) in points"
        :key="point.key"
        class="trend-dot"
        :class="{ 'trend-dot--last': index === points.length - 1 }"
        :style="{ left: `${(x(index) / W) * 100}%`, top: `${(y(point.kwh) / H) * 100}%` }"
        :title="`${point.label}: ${formatKwh(point.kwh)} kWh/hari`"
        aria-hidden="true"
      />
    </div>

    <div class="trend-foot">
      <span>{{ points[0]?.label }}</span>
      <strong class="tabular">
        {{ last ? `${formatKwh(last.kwh)} kWh/hari` : '-' }}
      </strong>
      <span>{{ last?.label }}</span>
    </div>
  </div>
</template>
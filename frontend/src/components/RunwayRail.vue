<script setup lang="ts">
import { computed } from 'vue'
import { DAY_MS } from '../domain/energy'
import { formatDays, formatDate, timeOnly } from '../domain/format'

const props = defineProps<{
  nowMs: number
  targetMs: number
  depletionMs: number | null
  daysLeft: number | null
}>()

const endMs = computed(() => {
  if (props.depletionMs == null) return props.targetMs + 2 * DAY_MS
  return Math.max(props.targetMs, props.depletionMs) + 2 * DAY_MS
})

const span = computed(() => Math.max(1, endMs.value - props.nowMs))

function percent(ms: number): number {
  return Math.min(98, Math.max(2, ((ms - props.nowMs) / span.value) * 100))
}

const targetPct = computed(() => percent(props.targetMs))
const depletionPct = computed(() =>
  props.depletionMs == null ? null : Math.min(100, Math.max(0, ((props.depletionMs - props.nowMs) / span.value) * 100)),
)
const isLate = computed(() => props.depletionMs != null && props.depletionMs < props.targetMs)

const targetLabel = computed(
  () => `${formatDate(props.targetMs)}, ${timeOnly(props.targetMs)}`,
)
const bufferLabel = computed(() =>
  props.depletionMs == null ? 'Belum bisa dihitung' : formatDate(props.depletionMs),
)
const accessibleLabel = computed(() => {
  if (props.daysLeft == null || props.depletionMs == null) {
    return `Belum cukup data untuk memperkirakan waktu mencapai buffer. Target isi berikutnya ${targetLabel.value}.`
  }
  return `Perkiraan buffer dalam ${formatDays(props.daysLeft)}, sekitar ${bufferLabel.value}. Target isi berikutnya ${targetLabel.value}.`
})
</script>

<template>
  <div class="runway" role="img" :aria-label="accessibleLabel">
    <div class="runway-track" aria-hidden="true">
      <div
        v-if="depletionPct != null"
        class="runway-fill"
        :class="{ 'runway-fill--late': isLate }"
        :style="{ width: `${depletionPct}%` }"
      />
      <span class="runway-now" />
      <span class="runway-target" :style="{ left: `${targetPct}%` }">
        <span class="runway-tick runway-tick--target" />
        <span class="runway-dot runway-dot--target" />
      </span>
      <span
        v-if="depletionPct != null"
        class="runway-buffer"
        :class="{ 'runway-buffer--late': isLate }"
        :style="{ left: `${depletionPct}%` }"
      >
        <span class="runway-tick" />
        <span class="runway-dot" />
      </span>
    </div>

    <div class="runway-legend" aria-hidden="true">
      <div class="runway-item">
        <span class="runway-key runway-key--now" />
        <span class="runway-item-label">Sekarang</span>
        <strong>{{ formatDate(nowMs) }}</strong>
      </div>
      <div class="runway-item">
        <span class="runway-key runway-key--target" />
        <span class="runway-item-label">Target isi</span>
        <strong>{{ targetLabel }}</strong>
      </div>
      <div class="runway-item">
        <span class="runway-key runway-key--buffer" :class="{ 'runway-key--late': isLate }" />
        <span class="runway-item-label">Perkiraan buffer</span>
        <strong>{{ bufferLabel }}</strong>
      </div>
    </div>
  </div>
</template>

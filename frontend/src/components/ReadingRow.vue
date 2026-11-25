<script setup lang="ts">
import { computed } from 'vue'
import type { ReadingView } from '../domain/types'
import {
  formatKwh,
  formatRupiah,
  formatDate,
  timeOnly,
} from '../domain/format'

const props = defineProps<{
  view: ReadingView
  compact?: boolean
  actions?: boolean
}>()

const emit = defineEmits<{ edit: []; delete: [] }>()

const comparisonLabel = computed(() => {
  if (props.view.comparison === 'hemat') return 'Hemat'
  if (props.view.comparison === 'boros') return 'Boros'
  return 'Stabil'
})

const comparisonColor = computed(() => {
  if (props.view.comparison === 'hemat') return 'success'
  if (props.view.comparison === 'boros') return 'error'
  return 'secondary'
})

const comparisonDiff = computed(() => {
  if (props.view.comparisonDiff == null) return ''
  return `${formatKwh(Math.abs(props.view.comparisonDiff))} kWh/hari`
})
</script>

<template>
  <div
    class="reading-row"
    :class="{ 'reading-row--compact': compact, 'reading-row--actions': actions }"
  >
    <div class="reading-date">
      <span class="reading-day">{{ formatDate(view.timestamp) }}</span>
      <span class="reading-time tabular">{{ timeOnly(view.timestamp) }}</span>
    </div>

    <div class="reading-main">
      <div class="reading-value tabular">
        {{ formatKwh(view.reading) }} <span>kWh</span>
      </div>
      <div v-if="view.notes" class="reading-note">{{ view.notes }}</div>
      <div class="reading-badges">
        <v-chip
          v-if="view.isTopup"
          size="x-small"
          color="accent"
          variant="flat"
        >
          Isi {{ formatKwh(view.rechargeAmount) }} kWh
        </v-chip>
        <v-chip
          v-else-if="view.comparison"
          size="x-small"
          :color="comparisonColor"
          variant="tonal"
        >
          {{ comparisonLabel }}
        </v-chip>
        <span v-if="view.comparison && comparisonDiff" class="reading-diff tabular">
          {{ comparisonDiff }}
        </span>
      </div>
    </div>

    <div class="reading-metrics">
      <div v-if="view.delta != null" class="reading-metric">
        <span class="metric-label">Pemakaian</span>
        <span class="metric-value tabular">{{ formatKwh(view.delta) }} kWh</span>
      </div>
      <div v-if="view.rate != null" class="reading-metric">
        <span class="metric-label">Laju</span>
        <span class="metric-value tabular">{{ formatKwh(view.rate) }} kWh/hari</span>
      </div>
      <div v-if="view.isTopup" class="reading-metric">
        <span class="metric-label">Biaya</span>
        <span class="metric-value tabular">{{ formatRupiah(view.cost) }}</span>
      </div>
    </div>

    <div v-if="actions" class="reading-actions">
      <v-btn
        icon="mdi-pencil-outline"
        variant="text"
        size="small"
        :aria-label="`Ubah catatan ${formatDate(view.timestamp)}`"
        @click="emit('edit')"
      />
      <v-btn
        icon="mdi-delete-outline"
        variant="text"
        size="small"
        :disabled="view.locked"
        :aria-label="`Hapus catatan ${formatDate(view.timestamp)}`"
        @click="emit('delete')"
      />
    </div>
  </div>
</template>

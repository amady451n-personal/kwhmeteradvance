<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAppStore } from '../stores/app'
import AddReadingDialog from '../components/AddReadingDialog.vue'
import ReadingRow from '../components/ReadingRow.vue'
import { dayKey, toMs } from '../domain/energy'
import type { Reading } from '../domain/types'
import { formatDateTime, formatKwh } from '../domain/format'
import { useFocusReturn } from '../composables/useFocusReturn'

const store = useAppStore()
const addOpen = ref(false)
const from = ref('')
const to = ref('')
const visibleCount = ref(20)
const editing = ref<Reading | null>(null)
const deleteTarget = ref<Reading | null>(null)
const editOpen = computed({
  get: () => editing.value != null,
  set: (value: boolean) => {
    if (!value) editing.value = null
  },
})

useFocusReturn(() => deleteTarget.value != null || editOpen.value)

const filtered = computed(() => {
  return store.views.filter((view) => {
    const key = dayKey(toMs(view.timestamp))
    if (from.value && key < from.value) return false
    if (to.value && key > to.value) return false
    return true
  })
})

const visible = computed(() => filtered.value.slice(0, visibleCount.value))
const hasMore = computed(() => visibleCount.value < filtered.value.length)
const filterActive = computed(() => Boolean(from.value || to.value))
const countLabel = computed(() => {
  if (filtered.value.length === 0) return 'Tidak ada catatan'
  return `${filtered.value.length} catatan`
})

watch([from, to], () => {
  visibleCount.value = 20
})

function clearFilter() {
  from.value = ''
  to.value = ''
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  const id = deleteTarget.value.id
  deleteTarget.value = null
  await store.deleteReading(id).catch(() => {})
}
</script>

<template>
  <div class="page readings-page">
    <header class="page-header">
      <div>
        <h1>Riwayat</h1>
        <p>Semua catatan meter, terbaru di atas.</p>
      </div>
      <v-btn v-if="!store.offline" color="primary" prepend-icon="mdi-plus" @click="addOpen = true">
        Catat meter
      </v-btn>
    </header>

    <v-alert v-if="store.error" type="error" density="compact" class="form-alert">
      {{ store.error }}
    </v-alert>

    <section
      v-if="store.pendingReadings.length"
      class="surface-card pending-readings"
      aria-label="Catatan meter tertunda"
    >
      <p>Catatan meter tertunda dari versi sebelumnya. Input meter sedang dimatikan; hapus catatan ini untuk membersihkan antrean.</p>
      <div v-for="item in store.pendingReadings" :key="item.id" class="pending-reading-row">
        <span class="tabular">
          {{ formatDateTime(item.input.timestamp) }} · {{ formatKwh(item.input.reading ?? 0) }} kWh
        </span>
        <v-btn
          variant="text"
          size="small"
          :aria-label="`Hapus catatan tertunda ${formatKwh(item.input.reading ?? 0)} kWh`"
          @click="store.discardReading(item.id)"
        >
          Hapus
        </v-btn>
      </div>
    </section>

    <section class="surface-card filter-card" aria-label="Filter tanggal">
      <div class="filter-fields">
        <v-text-field
          v-model="from"
          type="date"
          label="Dari"
          density="compact"
          clearable
          hide-details
        />
        <v-text-field
          v-model="to"
          type="date"
          label="Sampai"
          density="compact"
          clearable
          hide-details
        />
      </div>
      <div class="filter-meta">
        <span>{{ countLabel }}</span>
        <v-btn
          v-if="filterActive"
          variant="text"
          size="small"
          @click="clearFilter"
        >
          Hapus filter
        </v-btn>
      </div>
    </section>

    <section v-if="visible.length" class="reading-list" aria-label="Daftar catatan">
      <ReadingRow
        v-for="view in visible"
        :key="view.id"
        :view="view"
        actions
        @edit="editing = view"
        @delete="deleteTarget = view"
      />
    </section>

    <section v-else class="surface-card empty-state">
      <v-icon icon="mdi-clipboard-text-clock-outline" size="32" />
      <h2>{{ filterActive ? 'Tidak ada catatan di rentang ini' : 'Belum ada catatan' }}</h2>
      <p v-if="filterActive">
        Ubah rentang tanggal atau hapus filter untuk melihat semua catatan.
      </p>
      <p v-else>
        Mulai dengan mencatat angka meter hari ini. Catatan pertama menjadi dasar perkiraan.
      </p>
      <v-btn v-if="filterActive" variant="tonal" @click="clearFilter">Hapus filter</v-btn>
      <v-btn v-if="!store.offline" color="primary" prepend-icon="mdi-plus" @click="addOpen = true">
        Catat meter
      </v-btn>
    </section>

    <div v-if="hasMore" class="load-more">
      <v-btn variant="tonal" @click="visibleCount += 20">
        Muat {{ Math.min(20, filtered.length - visibleCount) }} catatan lagi
      </v-btn>
    </div>

    <AddReadingDialog v-model="addOpen" />
    <AddReadingDialog v-model="editOpen" :reading="editing" />

    <v-dialog
      :model-value="deleteTarget != null"
      max-width="420"
      aria-labelledby="delete-reading-title"
      @update:model-value="deleteTarget = null"
    >
      <v-card class="dialog-card">
        <div class="dialog-head">
          <div>
            <h2 id="delete-reading-title">Hapus catatan?</h2>
            <p>Catatan yang terkait token tidak bisa dihapus.</p>
          </div>
        </div>
        <v-card-text>
          <p v-if="deleteTarget" class="confirm-copy">
            {{ formatKwh(deleteTarget.reading) }} kWh,
            {{ formatDateTime(deleteTarget.timestamp) }}.
          </p>
        </v-card-text>
        <v-card-actions class="dialog-actions">
          <v-btn variant="text" @click="deleteTarget = null">Batal</v-btn>
          <v-btn color="error" variant="flat" @click="confirmDelete">Hapus</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
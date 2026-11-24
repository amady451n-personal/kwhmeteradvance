<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAppStore } from '../stores/app'
import { apiClient } from '../api'
import type { UsageBucket } from '../api'
import ReadingRow from '../components/ReadingRow.vue'
import { dayKey, toMs } from '../domain/energy'
import {
  formatDateTime,
  formatDays,
  formatKwh,
  formatRupiah,
  formatShortDate,
} from '../domain/format'

const store = useAppStore()
const todayKey = dayKey(Date.now())
const from = ref(`${todayKey.slice(0, 8)}01`)
const to = ref(todayKey)

const filtered = computed(() =>
  store.views.filter((view) => {
    const key = dayKey(toMs(view.timestamp))
    if (from.value && key < from.value) return false
    if (to.value && key > to.value) return false
    return true
  }),
)

const visible = computed(() => filtered.value.slice(0, 12))
const remaining = computed(() => Math.max(0, filtered.value.length - visible.value.length))
const price = computed(() => store.summary.effectivePrice)

function printReport() {
  window.print()
}

const pdfError = ref('')
const group = ref<'day' | 'week' | 'month'>('week')
const buckets = ref<UsageBucket[]>([])
const bucketsError = ref('')

async function loadBuckets() {
  bucketsError.value = ''
  try {
    const page = await apiClient().getUsage({
      from: from.value || todayKey,
      to: to.value || todayKey,
      group: group.value,
    })
    buckets.value = page.data
  } catch (caught) {
    buckets.value = []
    bucketsError.value = caught instanceof Error ? caught.message : 'Gagal memuat pemakaian.'
  }
}

watch([from, to, group], loadBuckets, { immediate: true })

async function downloadPdf() {
  pdfError.value = ''
  try {
    const { blob, filename } = await apiClient().getReport(from.value, to.value)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename ?? `meteran-${from.value}-${to.value}.pdf`
    link.click()
    URL.revokeObjectURL(url)
  } catch (caught) {
    pdfError.value = caught instanceof Error ? caught.message : 'Gagal mengunduh PDF.'
  }
}
</script>

<template>
  <div class="page reports-page">
    <header class="page-header print-hide">
      <div>
        <h1>Laporan</h1>
        <p>Ringkasan untuk arsip dan pengecekan pemakaian.</p>
      </div>
    </header>

    <section class="surface-card filter-card print-hide" aria-label="Rentang laporan">
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
      <p class="filter-note">
        {{ filtered.length }} catatan dalam rentang. Ringkasan di bawah memakai data terbaru.
      </p>
    </section>

    <article class="surface-card report-card print-area">
      <header class="report-head">
        <div>
          <p class="eyebrow">meteran</p>
          <h2>Ringkasan meteran</h2>
          <p>Dibuat {{ formatDateTime(store.nowMs) }}</p>
        </div>
        <div class="report-range">
          <span>Rentang catatan</span>
          <strong>{{ from || 'Awal' }} sampai {{ to || 'Sekarang' }}</strong>
        </div>
      </header>

      <dl class="report-facts">
        <div>
          <dt>Sisa terakhir</dt>
          <dd class="tabular">
            {{ store.summary.latest ? `${formatKwh(store.summary.latest.reading)} kWh` : '-' }}
          </dd>
        </div>
        <div>
          <dt>Buffer</dt>
          <dd class="tabular">{{ formatKwh(store.summary.buffer, 0) }} kWh</dd>
        </div>
        <div>
          <dt>Rata-rata {{ store.summary.avgWindow }} hari</dt>
          <dd class="tabular">{{ formatKwh(store.summary.avgKwh) }} kWh/hari</dd>
        </div>
        <div>
          <dt>Perkiraan buffer</dt>
          <dd>
            {{ store.summary.depletionMs ? formatShortDate(store.summary.depletionMs) : 'Belum bisa dihitung' }}
          </dd>
        </div>
        <div>
          <dt>Harga efektif</dt>
          <dd class="tabular">{{ price ? `${formatRupiah(price)}/kWh` : 'Belum ada data' }}</dd>
        </div>
        <div>
          <dt>Stok token</dt>
          <dd class="tabular">{{ formatKwh(store.summary.tokenStock) }} kWh</dd>
        </div>
        <div>
          <dt>Pemakaian minggu ini</dt>
          <dd class="tabular">{{ formatKwh(store.summary.weekKwh) }} kWh</dd>
        </div>
        <div>
          <dt>Pemakaian bulan ini</dt>
          <dd class="tabular">{{ formatKwh(store.summary.monthKwh) }} kWh</dd>
        </div>
        <div v-if="store.summary.tariffComparison.diffPercent != null">
          <dt>Selisih tarif resmi</dt>
          <dd class="tabular">
            {{ store.summary.tariffComparison.diffPercent > 0 ? '+' : '' }}{{
              store.summary.tariffComparison.diffPercent.toFixed(1)
            }}%
          </dd>
        </div>
        <div>
          <dt>Sisa waktu</dt>
          <dd>{{ formatDays(store.summary.daysLeft) }}</dd>
        </div>
      </dl>

      <section class="report-usage print-hide">
        <div class="card-head">
          <h3>Pemakaian per {{ group === 'day' ? 'hari' : group === 'week' ? 'minggu' : 'bulan' }}</h3>
          <v-btn-toggle v-model="group" mandatory divided density="compact" variant="outlined">
            <v-btn value="day">Hari</v-btn>
            <v-btn value="week">Minggu</v-btn>
            <v-btn value="month">Bulan</v-btn>
          </v-btn-toggle>
        </div>
        <p v-if="bucketsError" class="form-error">{{ bucketsError }}</p>
        <table v-else-if="buckets.length" class="usage-table">
          <thead>
            <tr>
              <th scope="col">Periode</th>
              <th scope="col">kWh</th>
              <th scope="col">Biaya</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="bucket in buckets" :key="bucket.key">
              <td>
                {{ bucket.label }}
                <span v-if="bucket.estimated" class="estimate-note">sebagian perkiraan</span>
              </td>
              <td class="tabular">{{ formatKwh(bucket.kwh) }}</td>
              <td class="tabular">{{ formatRupiah(bucket.cost) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="empty-inline">Belum ada pemakaian pada rentang ini.</p>
      </section>

      <section class="report-readings">
        <h3>Catatan dalam rentang</h3>
        <div v-if="visible.length" class="reading-list">
          <ReadingRow v-for="view in visible" :key="view.id" :view="view" compact />
        </div>
        <p v-else class="empty-inline">Tidak ada catatan dalam rentang ini.</p>
        <p v-if="remaining > 0" class="report-more">
          {{ remaining }} catatan lain tidak dicetak. Persempit rentang untuk laporan yang lebih pendek.
        </p>
      </section>

      <footer class="report-foot">
        <span>Sumber: {{ store.sourceLabel }}.</span>
        <span>Angka mengikuti catatan terakhir yang tersedia.</span>
      </footer>
    </article>

    <div class="report-actions print-hide">
      <v-btn color="primary" prepend-icon="mdi-printer-outline" @click="printReport">
        Cetak ringkasan
      </v-btn>
      <div class="report-pdf">
        <v-btn
          variant="outlined"
          prepend-icon="mdi-file-pdf-box"
          :disabled="store.mode === 'preview'"
          @click="downloadPdf"
        >
          PDF server
        </v-btn>
        <p v-if="pdfError" class="form-error">{{ pdfError }}</p>
        <p v-else-if="store.mode === 'preview'">Tersedia setelah backend terpasang.</p>
        <p v-else>Ringkasan resmi dari server dalam rentang ini.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.usage-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 12px;
}

.usage-table th,
.usage-table td {
  text-align: left;
  padding: 8px 4px;
  border-bottom: 1px solid rgba(var(--v-border-color), 0.16);
}

.usage-table th:not(:first-child),
.usage-table td:not(:first-child) {
  text-align: right;
}

.estimate-note {
  display: block;
  font-size: 0.75rem;
  opacity: 0.7;
}
</style>

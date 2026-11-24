<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useAppStore } from '../stores/app'
import AddReadingDialog from '../components/AddReadingDialog.vue'
import ReadingRow from '../components/ReadingRow.vue'
import RunwayRail from '../components/RunwayRail.vue'
import UsageBars from '../components/UsageBars.vue'
import {
  formatDays,
  formatDateTime,
  formatKwh,
  formatRelative,
  formatRupiah,
  formatShortDate,
} from '../domain/format'

const store = useAppStore()
const { smAndDown } = useDisplay()
const addOpen = ref(false)

const summary = computed(() => store.summary)
const latest = computed(() => store.summary.latest)
const recent = computed(() => store.views.slice(0, 5))
const aboveBuffer = computed(() => store.summary.availableKwh >= store.summary.buffer)
const latestRelative = computed(() =>
  latest.value ? formatRelative(latest.value.timestamp, store.nowMs) : '',
)
const price = computed(() => store.summary.effectivePrice)
const tariff = computed(() => store.summary.tariffComparison)
</script>

<template>
  <div class="page dashboard">
    <h1 class="sr-only">Beranda</h1>

    <v-alert
      v-if="!store.settings.previewDismissed && store.mode === 'preview'"
      type="info"
      density="compact"
      closable
      icon="mdi-database-eye-outline"
      class="preview-alert"
      @click:close="store.dismissPreview"
    >
      <div class="preview-copy">
        <strong>Mode pratinjau</strong>
        <span>
          {{ store.sourceLabel }}. Perubahan disimpan di perangkat ini, belum ke server.
        </span>
      </div>
    </v-alert>

    <v-alert
      v-if="store.offline"
      type="warning"
      density="compact"
      class="form-alert"
    >
      Tidak ada koneksi ke server. Data yang tampil dari perhitungan perangkat.
    </v-alert>

    <v-alert
      v-for="alert in summary.alerts"
      :key="alert.code"
      :type="alert.severity === 'critical' ? 'error' : alert.severity === 'warning' ? 'warning' : 'info'"
      density="compact"
      class="form-alert"
    >
      {{ alert.message }}
    </v-alert>

    <section class="balance-card">
      <div class="balance-top">
        <div class="balance-copy">
          <p class="eyebrow">Sisa token</p>
          <div class="balance-value tabular">
            {{ latest ? formatKwh(latest.reading) : '-' }}
            <span class="balance-unit">kWh</span>
          </div>
          <p v-if="latest" class="balance-meta">
            Terakhir dicatat {{ formatDateTime(latest.timestamp) }}
            <span class="meta-sep" aria-hidden="true">·</span>
            {{ latestRelative }}
          </p>
          <p v-else class="balance-meta">Belum ada catatan meter.</p>

          <div class="balance-status">
            <v-chip
              :color="aboveBuffer ? 'success' : 'error'"
              variant="tonal"
              size="small"
            >
              {{ aboveBuffer ? 'Di atas buffer' : 'Di bawah buffer' }}
            </v-chip>
            <span class="buffer-note">
              Buffer {{ formatKwh(summary.buffer, 0) }} kWh
            </span>
          </div>
        </div>

        <div class="balance-side">
          <div class="balance-metric">
            <span>Perkiraan buffer</span>
            <strong class="tabular">
              {{ summary.depletionMs ? formatShortDate(summary.depletionMs) : 'Belum bisa' }}
            </strong>
          </div>
          <div class="balance-metric">
            <span>Sisa waktu</span>
            <strong>{{ formatDays(summary.daysLeft) }}</strong>
          </div>
          <v-btn
            v-if="!smAndDown"
            color="primary"
            size="large"
            prepend-icon="mdi-plus"
            @click="addOpen = true"
          >
            Catat meter
          </v-btn>
        </div>
      </div>

      <RunwayRail
        :now-ms="store.nowMs"
        :target-ms="summary.targetMs"
        :depletion-ms="summary.depletionMs"
        :days-left="summary.daysLeft"
      />
    </section>

    <v-alert
      v-if="summary.recommendation"
      type="warning"
      density="compact"
      class="recommendation-alert"
    >
      Sampai target isi berikutnya, perlu sekitar
      <strong>{{ formatKwh(summary.needKwh) }} kWh</strong>.
      Nominal terdekat {{ formatRupiah(summary.recommendation.rupiah) }}.
    </v-alert>

    <div class="dashboard-grid">
      <section class="surface-card usage-card">
        <div class="card-head">
          <div>
            <h2>Pemakaian 7 hari</h2>
            <p>kWh per hari, termasuk hari ini.</p>
          </div>
          <v-btn variant="text" to="/riwayat" append-icon="mdi-arrow-right">
            Riwayat
          </v-btn>
        </div>

        <UsageBars :days="summary.chart" :average="summary.avgKwh" />

        <div class="usage-foot">
          <div class="usage-foot-item">
            <span>Rata-rata {{ summary.avgWindow }} hari</span>
            <strong class="tabular">{{ formatKwh(summary.avgKwh) }} kWh/hari</strong>
          </div>
          <div class="usage-foot-item">
            <span>Hari ini</span>
            <strong class="tabular">{{ formatKwh(summary.todayKwh) }} kWh</strong>
          </div>
          <div class="usage-foot-item">
            <span>Minggu ini</span>
            <strong class="tabular">{{ formatKwh(summary.weekKwh) }} kWh</strong>
          </div>
        </div>
      </section>

      <section class="surface-card facts-card">
        <div class="card-head">
          <div>
            <h2>Ringkasan</h2>
            <p>Angka dari catatan yang ada.</p>
          </div>
        </div>

        <dl class="facts">
          <div class="fact fact--lead">
            <dt>Harga efektif</dt>
            <dd class="tabular">
              {{ price ? formatRupiah(price) : 'Belum ada data' }}
              <span v-if="price" class="fact-unit">/kWh</span>
            </dd>
            <p>Tertimbang dari isi token yang punya biaya.</p>
          </div>

          <div class="fact">
            <dt>Stok token</dt>
            <dd class="tabular">{{ formatKwh(summary.tokenStock) }} kWh</dd>
            <v-btn variant="text" size="small" to="/token" class="fact-link">
              {{ summary.tokenStock > 0 ? 'Lihat token' : 'Tambah token' }}
            </v-btn>
          </div>

          <div class="fact">
            <dt>Pemakaian minggu ini</dt>
            <dd class="tabular">{{ formatKwh(summary.weekKwh) }} kWh</dd>
            <p>{{ summary.weekCost == null ? 'Harga belum diketahui.' : formatRupiah(summary.weekCost) }}</p>
          </div>

          <div class="fact">
            <dt>Pemakaian bulan ini</dt>
            <dd class="tabular">{{ formatKwh(summary.monthKwh) }} kWh</dd>
            <p v-if="(summary.monthCost ?? 0) > 0">
              {{ formatRupiah(summary.monthCost) }} untuk isi token bulan ini.
            </p>
            <p v-else>Belum ada biaya isi token bulan ini.</p>
          </div>

          <div v-if="tariff.diffPercent != null" class="fact">
            <dt>Dibanding tarif resmi</dt>
            <dd class="tabular">
              {{ tariff.diffPercent > 0 ? '+' : '' }}{{ tariff.diffPercent.toFixed(1) }}%
            </dd>
            <p>
              {{ formatRupiah(tariff.purchasePricePerKwh) }} vs
              {{ formatRupiah(tariff.officialTariffPerKwh) }} per kWh.
            </p>
          </div>
        </dl>
      </section>
    </div>

    <section class="surface-card recent-card">
      <div class="card-head">
        <div>
          <h2>Catatan terakhir</h2>
          <p>Lima catatan terbaru.</p>
        </div>
        <v-btn variant="text" to="/riwayat" append-icon="mdi-arrow-right">
          Lihat semua
        </v-btn>
      </div>

      <div v-if="recent.length" class="reading-list">
        <ReadingRow v-for="view in recent" :key="view.id" :view="view" compact />
      </div>
      <p v-else class="empty-inline">
        Belum ada catatan meter. Mulai dengan mencatat angka meter hari ini.
      </p>
    </section>

    <v-btn
      v-if="smAndDown"
      color="primary"
      size="large"
      prepend-icon="mdi-plus"
      class="mobile-fab"
      @click="addOpen = true"
    >
      Catat
    </v-btn>

    <AddReadingDialog v-model="addOpen" />
  </div>
</template>

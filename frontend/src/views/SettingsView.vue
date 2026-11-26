<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '../stores/app'
import { apiClient } from '../api'
import type { ImportPreview } from '../api'
import { formatKwh, formatRupiah } from '../domain/format'

const store = useAppStore()
const denomInput = ref('')
const denomError = ref('')
const importFile = ref<File[]>([])
const importPreview = ref<ImportPreview | null>(null)
const importError = ref('')
const importBusy = ref(false)
const importResult = ref('')
const pushError = ref('')
const pushDone = ref(false)
const pushBusy = ref(false)

function base64ToBytes(value: string): Uint8Array {
  const trimmed = value.trim()
  if (!trimmed) {
    throw new Error('Kunci push belum diatur di server (METERAN_VAPID_PUBLIC).')
  }
  let raw: string
  try {
    const padding = '='.repeat((4 - (trimmed.length % 4)) % 4)
    raw = atob((trimmed + padding).replace(/-/g, '+').replace(/_/g, '/'))
  } catch {
    throw new Error('Kunci push server tidak bisa dibaca. Periksa METERAN_VAPID_PUBLIC.')
  }
  const bytes = Uint8Array.from(raw, (char) => char.charCodeAt(0))
  if (bytes.length !== 65 || bytes[0] !== 0x04) {
    throw new Error('Kunci push server tidak valid. Periksa METERAN_VAPID_PUBLIC.')
  }
  return bytes
}

async function enablePush() {
  pushError.value = ''
  pushBusy.value = true
  try {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      throw new Error('Perangkat ini tidak mendukung notifikasi push.')
    }
    const permission = await Notification.requestPermission()
    if (permission !== 'granted') throw new Error('Izin notifikasi ditolak.')

    const registration = await navigator.serviceWorker.ready
    const key = await apiClient().getPushKey()
    const existing = await registration.pushManager.getSubscription()
    const subscription =
      existing ??
      (await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: base64ToBytes(key) as BufferSource,
      }))
    const json = subscription.toJSON()
    await apiClient().subscribePush({
      endpoint: subscription.endpoint,
      keys: { p256dh: json.keys?.p256dh ?? '', auth: json.keys?.auth ?? '' },
    })
    pushDone.value = true
  } catch (caught) {
    pushError.value = caught instanceof Error ? caught.message : 'Gagal mengaktifkan notifikasi.'
  } finally {
    pushBusy.value = false
  }
}

async function runImportPreview() {
  const file = importFile.value[0]
  if (!file) return
  importBusy.value = true
  importError.value = ''
  importResult.value = ''
  try {
    importPreview.value = await apiClient().importPreview(file)
  } catch (caught) {
    importPreview.value = null
    importError.value = caught instanceof Error ? caught.message : 'Gagal membaca berkas.'
  } finally {
    importBusy.value = false
  }
}

async function commitImport() {
  if (!importPreview.value) return
  importBusy.value = true
  importError.value = ''
  try {
    const result = await apiClient().importCommit(importPreview.value.previewId, true)
    importResult.value = `${result.imported.rows} catatan diimpor, ${
      result.verification.rowsMatch ? 'jumlah cocok' : 'jumlah belum cocok'
    }.`
    importPreview.value = null
    importFile.value = []
    await store.init()
  } catch (caught) {
    importError.value = caught instanceof Error ? caught.message : 'Gagal mengimpor.'
  } finally {
    importBusy.value = false
  }
}

const buffer = computed({
  get: () => store.settings.buffer,
  set: (value: number | string) => {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      void store.updateSettings({ buffer: Math.max(0, Math.min(1000, parsed)) })
    }
  },
})

const avgWindow = computed({
  get: () => store.settings.avgWindow,
  set: (value: number) => {
    if ([7, 14, 30].includes(Number(value))) {
      void store.updateSettings({ avgWindow: Number(value) as 7 | 14 | 30 })
    }
  },
})

const targetHour = computed({
  get: () => store.settings.targetHour,
  set: (value: number) => {
    void store.updateSettings({ targetHour: Number(value) })
  },
})

const tariff = computed({
  get: () => store.settings.tariffPerKwh,
  set: (value: number | string) => {
    const parsed = Number(String(value).replace(/[^\d.,]/g, '').replace(/\./g, '').replace(',', '.'))
    void store.updateSettings({ tariffPerKwh: Number.isFinite(parsed) ? parsed : null })
  },
})

const targetOptions = [
  { title: '05:00 WIB', value: 5 },
  { title: '06:00 WIB', value: 6 },
  { title: '07:00 WIB', value: 7 },
]

function addDenomination() {
  denomError.value = ''
  const value = Number(denomInput.value.replace(/[^\d]/g, ''))
  if (!Number.isFinite(value) || value < 1000 || value > 1_000_000) {
    denomError.value = 'Masukkan nominal antara Rp1.000 dan Rp1.000.000.'
    return
  }
  if (store.settings.denominations.includes(value)) {
    denomError.value = 'Nominal itu sudah ada.'
    return
  }
  void store.updateSettings({
    denominations: [...store.settings.denominations, value].sort((a, b) => a - b),
  })
  denomInput.value = ''
}

function removeDenomination(value: number) {
  denomError.value = ''
  if (store.settings.denominations.length <= 1) {
    denomError.value = 'Sisakan minimal satu nominal.'
    return
  }
  void store.updateSettings({
    denominations: store.settings.denominations.filter((item) => item !== value),
  })
}

</script>

<template>
  <div class="page settings-page">
    <header class="page-header">
      <div>
        <h1>Pengaturan</h1>
        <p>Perubahan langsung dipakai setelah disimpan.</p>
      </div>
    </header>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Perhitungan</h2>
          <p>Dipakai untuk perkiraan sisa dan saran isi token.</p>
        </div>
      </div>

      <div class="settings-fields">
        <v-text-field
          v-model.number="buffer"
          type="number"
          min="0"
          max="1000"
          step="0.5"
          label="Buffer"
          suffix="kWh"
          hint="Batas aman sebelum dianggap perlu isi token."
        />

        <div class="setting-block">
          <span class="setting-label">Jendela rata-rata</span>
          <v-btn-toggle v-model="avgWindow" mandatory divided variant="outlined" color="primary">
            <v-btn :value="7">7 hari</v-btn>
            <v-btn :value="14">14 hari</v-btn>
            <v-btn :value="30">30 hari</v-btn>
          </v-btn-toggle>
          <p class="setting-hint">
            Rata-rata {{ avgWindow }} hari dipakai untuk perkiraan buffer.
          </p>
        </div>

        <v-select
          v-model="targetHour"
          :items="targetOptions"
          label="Target isi berikutnya"
          hint="Tanggal 1 pada jam ini, mengikuti WIB."
        />

        <v-text-field
          v-model.number="tariff"
          type="number"
          min="0"
          step="0.01"
          label="Tarif resmi"
          prefix="Rp"
          suffix="/kWh"
          hint="Hanya untuk pembanding. Tidak dipakai menghitung sisa."
        />
      </div>
    </section>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Nominal token</h2>
          <p>Dipakai untuk memilih nominal terdekat saat stok menipis.</p>
        </div>
      </div>

      <div class="denom-list">
        <v-chip
          v-for="value in store.settings.denominations"
          :key="value"
          closable
          variant="tonal"
          color="primary"
          class="denom-chip"
          @click:close="removeDenomination(value)"
        >
          {{ formatRupiah(value) }}
        </v-chip>
      </div>

      <div class="denom-add">
        <v-text-field
          v-model="denomInput"
          label="Nominal baru"
          prefix="Rp"
          inputmode="numeric"
          density="compact"
          hide-details
          class="denom-field"
          @keyup.enter="addDenomination"
        />
        <v-btn variant="tonal" @click="addDenomination">Tambah</v-btn>
      </div>
      <p v-if="denomError" class="form-error">{{ denomError }}</p>
    </section>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Impor data lama</h2>
          <p>Sekali saja, dari ekspor aplikasi basic. Bergantung koneksi server.</p>
        </div>
      </div>

      <div class="denom-add">
        <v-file-input
          v-model="importFile"
          label="Berkas ekspor"
          accept=".csv,.json,.db,.sqlite"
          density="compact"
          hide-details
          class="denom-field"
        />
        <v-btn variant="tonal" :loading="importBusy" @click="runImportPreview">
          Periksa
        </v-btn>
      </div>

      <div v-if="importPreview" class="info-row">
        <div>
          <strong>
            {{ importPreview.groups.valid.length }} siap,
            {{ importPreview.groups.needsConfirmation.length }} perlu konfirmasi,
            {{ importPreview.groups.rejected.length }} ditolak
          </strong>
          <p>
            {{ importPreview.source.rows }} baris terbaca,
            {{ importPreview.source.otherMeterRows }} baris meter lain diabaikan.
          </p>
        </div>
        <v-btn color="primary" variant="flat" :loading="importBusy" @click="commitImport">
          Impor
        </v-btn>
      </div>

      <p v-if="importResult" class="setting-hint">{{ importResult }}</p>
      <p v-if="importError" class="form-error">{{ importError }}</p>
    </section>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Pengingat</h2>
          <p>Notifikasi push saat buffer mendekat atau catatan terlambat.</p>
        </div>
      </div>
      <div class="info-row">
        <div>
          <strong>{{ pushDone ? 'Notifikasi aktif' : 'Aktifkan notifikasi' }}</strong>
          <p>Butuh HTTPS dan izin browser. Di iOS, pasang aplikasi ke Home Screen dulu.</p>
        </div>
        <v-btn
          variant="tonal"
          :loading="pushBusy"
          :disabled="store.mode === 'preview' || pushDone"
          @click="enablePush"
        >
          {{ store.mode === 'preview' ? 'Perlu server' : 'Aktifkan' }}
        </v-btn>
      </div>
      <p v-if="pushError" class="form-error">{{ pushError }}</p>
    </section>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Tampilan</h2>
          <p>Warna mengikuti preferensi perangkat.</p>
        </div>
      </div>
      <div class="info-row">
        <div>
          <strong>Tema mengikuti perangkat</strong>
          <p>Terang atau gelap ditentukan pengaturan sistem. Tidak ada tema tetap.</p>
        </div>
        <v-icon icon="mdi-theme-light-dark" size="22" />
      </div>
    </section>

    <section class="surface-card settings-card">
      <div class="card-head">
        <div>
          <h2>Data dan akses</h2>
          <p>Sumber data dan kesiapan fitur keamanan.</p>
        </div>
      </div>

      <div class="info-row">
        <div>
          <strong>{{ store.sourceLabel }}</strong>
          <p>Perubahan tersimpan sesuai mode koneksi yang aktif.</p>
        </div>
      </div>

      <div class="info-row">
        <div>
          <strong>Kunci aplikasi</strong>
          <p>Sesi PIN aktif saat backend terhubung. Mode pratinjau tanpa PIN.</p>
        </div>
        <div class="d-flex align-center ga-2">
          <v-chip color="secondary" variant="tonal" size="small">
            {{ store.mode === 'preview' ? 'Pratinjau' : 'Aktif' }}
          </v-chip>
          <v-btn
            v-if="store.mode === 'server'"
            variant="tonal"
            size="small"
            @click="store.logout()"
          >
            Keluar
          </v-btn>
        </div>
      </div>

      <div class="info-row">
        <div>
          <strong>Rata-rata saat ini</strong>
          <p>{{ formatKwh(store.summary.avgKwh) }} kWh/hari dari {{ store.summary.avgWindow }} hari terakhir.</p>
        </div>
      </div>
    </section>
  </div>
</template>

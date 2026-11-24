<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '../stores/app'
import AddTokenDialog from '../components/AddTokenDialog.vue'
import type { TokenRecord } from '../domain/types'
import { formatDateTime, formatKwh, formatRupiah, formatShortDate } from '../domain/format'
import { useFocusReturn } from '../composables/useFocusReturn'

const store = useAppStore()
const addOpen = ref(false)
const removeTarget = ref<TokenRecord | null>(null)
const cancelTarget = ref<TokenRecord | null>(null)
const usedTarget = ref<TokenRecord | null>(null)
const usedReading = ref('')
const usedError = ref('')
const usedSaving = ref(false)
const revealed = ref<{ id: string; code: string } | null>(null)
const busy = ref('')

useFocusReturn(
  () =>
    removeTarget.value != null ||
    cancelTarget.value != null ||
    usedTarget.value != null ||
    revealed.value != null,
)

const tokens = computed(() =>
  [...store.tokens].sort(
    (a, b) => new Date(b.purchasedAt).getTime() - new Date(a.purchasedAt).getTime(),
  ),
)

const stockCount = computed(
  () => store.tokens.filter((token) => token.status === 'unused').length,
)

const targetLabel = computed(() => formatShortDate(store.summary.targetMs))

const statusLabel: Record<TokenRecord['status'], string> = {
  unused: 'Belum dipakai',
  used: 'Terpakai',
  locked: 'Terkunci',
}

const statusColor: Record<TokenRecord['status'], string> = {
  unused: 'success',
  used: 'secondary',
  locked: 'warning',
}

async function confirmRemove() {
  if (!removeTarget.value) return
  const id = removeTarget.value.id
  removeTarget.value = null
  await store.removeToken(id).catch(() => {})
}

function openUsed(token: TokenRecord) {
  usedTarget.value = token
  usedReading.value = String(
    Number(((store.latest?.reading ?? 0) + token.kwh).toFixed(2)),
  )
  usedError.value = ''
}

async function confirmUsed() {
  if (!usedTarget.value) return
  const value = Number(usedReading.value.replace(',', '.'))
  if (!Number.isFinite(value) || value < 0) {
    usedError.value = 'Angka sisa harus 0 atau lebih.'
    return
  }
  usedSaving.value = true
  try {
    await store.markTokenUsed(usedTarget.value.id, value)
    usedTarget.value = null
  } catch (caught) {
    usedError.value = caught instanceof Error ? caught.message : 'Gagal menandai token.'
  } finally {
    usedSaving.value = false
  }
}

async function cancel() {
  if (!cancelTarget.value) return
  const id = cancelTarget.value.id
  cancelTarget.value = null
  busy.value = id
  try {
    await store.cancelToken(id)
  } finally {
    busy.value = ''
  }
}

async function reveal(token: TokenRecord) {
  busy.value = token.id
  try {
    const code = await store.revealTokenCode(token.id)
    revealed.value = { id: token.id, code }
  } catch {
    revealed.value = null
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <div class="page tokens-page">
    <header class="page-header">
      <div>
        <h1>Token</h1>
        <p>Stok token yang belum dimasukkan ke meter.</p>
      </div>
      <v-btn color="primary" prepend-icon="mdi-plus" @click="addOpen = true">
        Tambah token
      </v-btn>
    </header>

    <section class="token-summary">
      <div>
        <p class="eyebrow">Stok token</p>
        <div class="token-stock tabular">
          {{ formatKwh(store.summary.tokenStock) }}
          <span>kWh</span>
        </div>
        <p class="token-summary-meta">
          {{ stockCount }} token belum dipakai. Stok ikut dihitung di perkiraan sisa.
        </p>
      </div>
      <div class="token-target">
        <span>Target isi berikutnya</span>
        <strong>{{ targetLabel }}</strong>
        <span class="tabular">
          {{ store.summary.needKwh > 0 ? `${formatKwh(store.summary.needKwh)} kWh` : 'Cukup sampai target' }}
        </span>
      </div>
    </section>

    <v-alert
      v-if="store.summary.recommendation && store.summary.recommendation.status !== 'price_unknown'"
      type="warning"
      density="compact"
      class="recommendation-alert"
    >
      Saran nominal terdekat:
      <strong>{{ formatRupiah(store.summary.recommendation.rupiah) }}</strong>
      untuk sekitar {{ formatKwh(store.summary.recommendation.kwh) }} kWh.
      <span v-if="store.summary.recommendation.status === 'largest_only'">
        Nominal terbesar yang tersedia.
      </span>
    </v-alert>
    <p v-else class="target-ok">
      <v-icon icon="mdi-check-circle-outline" size="18" />
      Belum perlu isi token sampai {{ targetLabel }} berdasarkan rata-rata
      {{ store.summary.avgWindow }} hari.
    </p>

    <v-alert v-if="store.error" type="error" density="compact" class="form-alert">
      {{ store.error }}
    </v-alert>

    <section v-if="tokens.length" class="token-list" aria-label="Daftar token">
      <article
        v-for="token in tokens"
        :key="token.id"
        class="surface-card token-row"
      >
        <div class="token-main">
          <div class="token-amount tabular">
            {{ formatKwh(token.kwh) }} <span>kWh</span>
          </div>
          <div class="token-meta">
            <span>{{ formatDateTime(token.purchasedAt) }}</span>
            <span>{{ formatRupiah(token.cost) }}</span>
            <span>{{ token.type === 'bulk' ? 'Isi besar' : 'Tambahan' }}</span>
            <span v-if="token.denomination != null">{{ formatRupiah(token.denomination) }}</span>
          </div>
          <p v-if="token.notes" class="token-note">{{ token.notes }}</p>
          <p v-if="token.denominationMismatch" class="token-note">
            Biaya di luar toleransi nominal. Periksa lagi.
          </p>
        </div>

        <div class="token-side">
          <v-chip :color="statusColor[token.status]" variant="tonal" size="small">
            {{ statusLabel[token.status] }}
          </v-chip>

          <v-btn
            v-if="token.hasCode"
            variant="text"
            size="small"
            :loading="busy === token.id"
            @click="reveal(token)"
          >
            Lihat kode
          </v-btn>

          <v-btn
            v-if="token.status === 'unused'"
            variant="text"
            size="small"
            :loading="busy === token.id"
            @click="openUsed(token)"
          >
            Tandai terpakai
          </v-btn>

          <v-btn
            v-if="token.status === 'used'"
            variant="text"
            size="small"
            :loading="busy === token.id"
            @click="cancelTarget = token"
          >
            Batalkan
          </v-btn>

          <v-btn
            v-if="token.canDelete"
            icon="mdi-delete-outline"
            variant="text"
            size="small"
            :aria-label="`Hapus token ${formatKwh(token.kwh)} kWh`"
            @click="removeTarget = token"
          />
        </div>
      </article>
    </section>

    <section v-else class="surface-card empty-state">
      <v-icon icon="mdi-ticket-confirmation-outline" size="32" />
      <h2>Belum ada token tersimpan</h2>
      <p>
        Catat token yang belum dimasukkan ke meter. Stok ini dipakai untuk menghitung
        perkiraan sisa dan saran isi berikutnya.
      </p>
      <v-btn color="primary" prepend-icon="mdi-plus" @click="addOpen = true">
        Tambah token
      </v-btn>
    </section>

    <AddTokenDialog v-model="addOpen" />

    <v-dialog
      :model-value="removeTarget != null"
      max-width="420"
      aria-labelledby="remove-token-title"
      @update:model-value="removeTarget = null"
    >
      <v-card class="dialog-card">
        <div class="dialog-head">
          <div>
            <h2 id="remove-token-title">Hapus token?</h2>
            <p>Hanya token yang belum ditautkan ke catatan meter yang bisa dihapus.</p>
          </div>
        </div>
        <v-card-text>
          <p v-if="removeTarget" class="confirm-copy">
            {{ formatKwh(removeTarget.kwh) }} kWh,
            {{ formatRupiah(removeTarget.cost) }},
            {{ formatDateTime(removeTarget.purchasedAt) }}.
          </p>
        </v-card-text>
        <v-card-actions class="dialog-actions">
          <v-btn variant="text" @click="removeTarget = null">Batal</v-btn>
          <v-btn color="error" variant="flat" @click="confirmRemove">Hapus</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="usedTarget != null"
      max-width="480"
      aria-labelledby="used-token-title"
      @update:model-value="usedTarget = null"
    >
      <v-card class="dialog-card">
        <div class="dialog-head">
          <div>
            <h2 id="used-token-title">Tandai token terpakai</h2>
            <p>Masukkan sisa kWh pada meter setelah token dimasukkan.</p>
          </div>
        </div>
        <v-card-text>
          <p v-if="usedTarget" class="confirm-copy">
            Token {{ formatKwh(usedTarget.kwh) }} kWh. Catatan terakhir
            {{ store.latest ? `${formatKwh(store.latest.reading)} kWh` : 'belum ada' }}.
          </p>
          <v-form @submit.prevent="confirmUsed">
            <v-text-field
              v-model="usedReading"
              label="Sisa kWh setelah token"
              suffix="kWh"
              inputmode="decimal"
              autofocus
              class="field"
              hint="Isi bawaan = catatan terakhir + kWh token."
              persistent-hint
            />
            <v-alert v-if="usedError" type="error" density="compact" class="form-alert">
              {{ usedError }}
            </v-alert>
            <div class="dialog-actions">
              <v-btn variant="text" @click="usedTarget = null">Batal</v-btn>
              <v-btn type="submit" color="primary" variant="flat" :loading="usedSaving">
                Tandai terpakai
              </v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="cancelTarget != null"
      max-width="420"
      aria-labelledby="cancel-token-title"
      @update:model-value="cancelTarget = null"
    >
      <v-card class="dialog-card">
        <div class="dialog-head">
          <div>
            <h2 id="cancel-token-title">Batalkan pemakaian token?</h2>
            <p>
              Token kembali ke belum dipakai. Catatan meter yang tertaut ikut dibatalkan.
            </p>
          </div>
        </div>
        <v-card-text>
          <p v-if="cancelTarget" class="confirm-copy">
            {{ formatKwh(cancelTarget.kwh) }} kWh,
            {{ formatRupiah(cancelTarget.cost) }},
            {{ formatDateTime(cancelTarget.purchasedAt) }}.
          </p>
        </v-card-text>
        <v-card-actions class="dialog-actions">
          <v-btn variant="text" @click="cancelTarget = null">Batal</v-btn>
          <v-btn color="primary" variant="flat" @click="cancel">Batalkan</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="revealed != null"
      max-width="420"
      aria-labelledby="reveal-token-title"
      @update:model-value="revealed = null"
    >
      <v-card class="dialog-card">
        <div class="dialog-head">
          <div>
            <h2 id="reveal-token-title">Kode token</h2>
            <p>Ditampilkan sekali setelah dibuka. Jangan bagikan ke orang lain.</p>
          </div>
        </div>
        <v-card-text>
          <p class="tabular reveal-code">{{ revealed?.code }}</p>
        </v-card-text>
        <v-card-actions class="dialog-actions">
          <v-btn variant="text" @click="revealed = null">Tutup</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.reveal-code {
  font-size: 1.25rem;
  letter-spacing: 0.08em;
  word-break: break-all;
}
</style>
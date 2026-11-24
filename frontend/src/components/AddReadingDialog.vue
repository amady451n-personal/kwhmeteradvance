<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useAppStore } from '../stores/app'
import { ApiError } from '../api'
import type { Reading } from '../domain/types'
import { fromWibInput, formatKwh, toWibInput } from '../domain/format'
import { useFocusReturn } from '../composables/useFocusReturn'

const props = defineProps<{ modelValue: boolean; reading?: Reading | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const store = useAppStore()
const { smAndDown } = useDisplay()

useFocusReturn(() => props.modelValue)

const open = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const form = reactive({
  reading: '',
  at: '',
  hasTopup: false,
  recharge: '',
  cost: '',
  notes: '',
})
const limitAt = ref('')
const error = ref('')
const saving = ref(false)
const pending = ref<string[]>([])

function reset() {
  const row = props.reading
  form.reading = row ? String(row.reading) : ''
  form.at = toWibInput(row ? Date.parse(row.timestamp) : Date.now())
  form.hasTopup = Boolean(row?.rechargeAmount)
  form.recharge = row?.rechargeAmount ? String(row.rechargeAmount) : ''
  form.cost = row?.cost ? String(row.cost) : ''
  form.notes = row?.notes ?? ''
  limitAt.value = toWibInput(Date.now())
  error.value = ''
  pending.value = []
  saving.value = false
}

watch(
  () => props.modelValue,
  (value) => {
    if (value) reset()
  },
)

function parseDecimal(value: string): number {
  const cleaned = value.trim().replace(/\s/g, '')
  if (cleaned.includes(',')) return Number(cleaned.replace(/\./g, '').replace(',', '.'))
  return Number(cleaned)
}

function parseInteger(value: string): number {
  return Number(value.replace(/[^\d]/g, ''))
}

async function submit(confirm: string[] = []) {
  error.value = ''
  pending.value = []
  const reading = parseDecimal(form.reading)
  if (!Number.isFinite(reading) || reading < 0) {
    error.value = 'Angka meter harus berupa angka 0 atau lebih.'
    return
  }
  if (!form.at) {
    error.value = 'Waktu catat wajib diisi.'
    return
  }

  const timestamp = fromWibInput(form.at)
  if (new Date(timestamp).getTime() > Date.now() + 60_000) {
    error.value = 'Waktu catat tidak boleh di masa depan.'
    return
  }

  const recharge = form.hasTopup ? parseDecimal(form.recharge) : 0
  if (form.hasTopup && (!Number.isFinite(recharge) || recharge <= 0)) {
    error.value = 'Jumlah token harus lebih dari 0 kWh.'
    return
  }

  const cost = form.hasTopup ? parseInteger(form.cost) : null
  if (form.hasTopup && (cost == null || !Number.isFinite(cost) || cost <= 0)) {
    error.value = 'Biaya token wajib diisi untuk menghitung harga efektif.'
    return
  }

  const payload = {
    timestamp,
    reading,
    rechargeAmount: form.hasTopup ? recharge : null,
    cost,
    notes: form.notes.trim() || null,
    confirm: confirm as ('increased_without_topup' | 'duplicate_timestamp')[],
  }

  saving.value = true
  try {
    if (props.reading) await store.updateReading(props.reading.id, payload)
    else await store.addReading(payload)
    open.value = false
  } catch (caught) {
    if (caught instanceof ApiError && caught.needsConfirmation) {
      pending.value = caught.detailCodes()
      error.value = caught.message
    } else {
      error.value = caught instanceof Error ? caught.message : 'Gagal menyimpan catatan.'
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <v-dialog
    v-model="open"
    max-width="560"
    :fullscreen="smAndDown"
    transition="dialog-bottom-transition"
    aria-labelledby="add-reading-title"
  >
    <v-card class="dialog-card">
      <div class="dialog-head">
        <div>
          <h2 id="add-reading-title">{{ reading ? 'Ubah catatan' : 'Catat meter' }}</h2>
          <p>Masukkan angka meter dan waktu pembacaan.</p>
        </div>
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Tutup"
          @click="open = false"
        />
      </div>

      <v-card-text class="dialog-body">
        <v-form @submit.prevent="submit()">
          <v-text-field
            v-model="form.reading"
            label="Angka meter"
            suffix="kWh"
            inputmode="decimal"
            autofocus
            placeholder="121,72"
            class="field"
          />
          <v-text-field
            v-model="form.at"
            type="datetime-local"
            label="Waktu catat"
            :max="limitAt"
            class="field"
          />

          <v-switch
            v-model="form.hasTopup"
            color="primary"
            label="Ada isi token pada catatan ini"
            hide-details
            class="field switch-field"
          />

          <v-expand-transition>
            <div v-if="form.hasTopup" class="topup-fields">
              <v-text-field
                v-model="form.recharge"
                label="Jumlah token"
                suffix="kWh"
                inputmode="decimal"
                placeholder="199,70"
                class="field"
              />
              <v-text-field
                v-model="form.cost"
                label="Biaya"
                prefix="Rp"
                inputmode="numeric"
                placeholder="301.500"
                class="field"
                hint="Dipakai untuk harga efektif per kWh."
                persistent-hint
              />
            </div>
          </v-expand-transition>

          <v-text-field
            v-model="form.notes"
            label="Catatan"
            maxlength="200"
            counter="200"
            class="field"
            hint="Opsional."
          />

          <v-alert
            v-if="error"
            :type="pending.length ? 'warning' : 'error'"
            density="compact"
            class="form-alert"
          >
            {{ error }}
            <div v-if="pending.length" class="confirm-actions">
              <v-btn
                size="small"
                color="warning"
                variant="flat"
                :loading="saving"
                @click="submit(pending)"
              >
                Tetap simpan
              </v-btn>
            </div>
          </v-alert>

          <div v-if="store.latest" class="balance-preview">
            <span>Catatan terakhir</span>
            <strong class="tabular">{{ formatKwh(store.latest.reading) }} kWh</strong>
          </div>

          <div class="dialog-actions">
            <v-btn variant="text" @click="open = false">Batal</v-btn>
            <v-btn type="submit" color="primary" variant="flat" :loading="saving">
              Simpan catatan
            </v-btn>
          </div>
        </v-form>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

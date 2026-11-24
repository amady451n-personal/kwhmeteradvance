<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useAppStore } from '../stores/app'
import { ApiError } from '../api'
import { fromWibInput, toWibInput } from '../domain/format'
import { useFocusReturn } from '../composables/useFocusReturn'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const store = useAppStore()
const { smAndDown } = useDisplay()

useFocusReturn(() => props.modelValue)

const open = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const form = reactive({
  code: '',
  kwh: '',
  cost: '',
  at: '',
  type: 'additional' as 'additional' | 'bulk',
  denomination: '',
  notes: '',
})
const limitAt = ref('')
const error = ref('')
const saving = ref(false)
const pending = ref<string[]>([])

const typeOptions = [
  { title: 'Token tambahan', value: 'additional' },
  { title: 'Token isi besar', value: 'bulk' },
]

function reset() {
  form.code = ''
  form.kwh = ''
  form.cost = ''
  form.at = toWibInput(Date.now())
  form.type = 'additional'
  form.denomination = ''
  form.notes = ''
  limitAt.value = toWibInput(Date.now())
  error.value = ''
  saving.value = false
  pending.value = []
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
  if (!form.code.trim()) {
    error.value = 'Kode token wajib diisi.'
    return
  }

  const kwh = parseDecimal(form.kwh)
  if (!Number.isFinite(kwh) || kwh <= 0) {
    error.value = 'Jumlah token harus lebih dari 0 kWh.'
    return
  }

  const cost = parseInteger(form.cost)
  if (!Number.isFinite(cost) || cost <= 0) {
    error.value = 'Biaya token wajib diisi.'
    return
  }

  if (!form.at) {
    error.value = 'Tanggal beli wajib diisi.'
    return
  }

  const purchasedAt = fromWibInput(form.at)
  if (new Date(purchasedAt).getTime() > Date.now() + 60_000) {
    error.value = 'Tanggal beli tidak boleh di masa depan.'
    return
  }

  saving.value = true
  try {
    await store.addToken({
      code: form.code.trim(),
      kwh,
      cost,
      purchasedAt,
      type: form.type,
      denomination: form.denomination ? parseInteger(form.denomination) : null,
      notes: form.notes.trim() || null,
      confirm: confirm as ('denomination_out_of_tolerance' | 'duplicate_timestamp')[],
    })
    open.value = false
  } catch (caught) {
    if (caught instanceof ApiError && caught.needsConfirmation) {
      pending.value = caught.detailCodes()
      error.value = caught.message
    } else {
      error.value = caught instanceof Error ? caught.message : 'Gagal menyimpan token.'
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
    aria-labelledby="add-token-title"
  >
    <v-card class="dialog-card">
      <div class="dialog-head">
        <div>
          <h2 id="add-token-title">Tambah token</h2>
          <p>Catat token yang belum dimasukkan ke meter.</p>
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
            v-model="form.code"
            label="Kode token"
            inputmode="numeric"
            autocomplete="off"
            hint="Disimpan terenkripsi di server, hanya bisa dibuka lewat tombol lihat."
            persistent-hint
            class="field"
          />
          <v-text-field
            v-model="form.kwh"
            label="Jumlah token"
            suffix="kWh"
            inputmode="decimal"
            placeholder="199,70"
            autofocus
            class="field"
          />
          <v-text-field
            v-model="form.cost"
            label="Biaya"
            prefix="Rp"
            inputmode="numeric"
            placeholder="301.500"
            class="field"
          />
          <v-text-field
            v-model="form.at"
            type="datetime-local"
            label="Tanggal beli"
            :max="limitAt"
            class="field"
          />
          <v-select
            v-model="form.type"
            label="Jenis token"
            :items="typeOptions"
            class="field"
          />
          <v-text-field
            v-model="form.denomination"
            label="Nominal"
            prefix="Rp"
            inputmode="numeric"
            class="field"
            hint="Opsional. Diisi otomatis dari daftar nominal bila kosong."
          />
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

          <div class="dialog-actions">
            <v-btn variant="text" @click="open = false">Batal</v-btn>
            <v-btn type="submit" color="primary" variant="flat" :loading="saving">
              Simpan token
            </v-btn>
          </div>
        </v-form>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

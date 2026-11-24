<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const pin = ref('')
const confirmPin = ref('')
const error = ref('')
const busy = ref(false)

const setup = computed(() => store.session?.pinSet === false)
const title = computed(() => (setup.value ? 'Buat PIN' : 'Masuk'))
const hint = computed(() =>
  setup.value
    ? 'PIN 6 angka dipakai untuk membuka aplikasi di perangkat ini.'
    : 'Masukkan PIN untuk membuka aplikasi.',
)

function press(digit: string) {
  error.value = ''
  if (setup.value && pin.value.length >= 6) {
    if (confirmPin.value.length < 6) confirmPin.value += digit
    return
  }
  if (pin.value.length < 6) pin.value += digit
}

function backspace() {
  if (setup.value && confirmPin.value) {
    confirmPin.value = confirmPin.value.slice(0, -1)
    return
  }
  pin.value = pin.value.slice(0, -1)
}

async function submit() {
  error.value = ''
  if (pin.value.length !== 6) {
    error.value = 'PIN harus 6 angka.'
    return
  }
  if (setup.value && confirmPin.value !== pin.value) {
    error.value = 'Ulangi PIN dengan angka yang sama.'
    return
  }

  busy.value = true
  try {
    if (setup.value) await store.setPin(null, pin.value)
    else await store.login(pin.value)
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Gagal membuka aplikasi.'
    pin.value = ''
    confirmPin.value = ''
  } finally {
    busy.value = false
  }
}

const digits = ['1', '2', '3', '4', '5', '6', '7', '8', '9']
</script>

<template>
  <div class="lock-screen">
    <div class="lock-card">
      <h1>{{ title }}</h1>
      <p>{{ hint }}</p>

      <div class="lock-dots" aria-hidden="true">
        <span v-for="index in 6" :key="index" :class="{ 'is-filled': pin.length >= index }" />
      </div>
      <p v-if="setup" class="lock-sub">
        {{ confirmPin.length < 6 ? 'Ulangi PIN' : 'PIN siap' }}
      </p>

      <div class="lock-pad" role="group" aria-label="Tombol PIN">
        <v-btn
          v-for="digit in digits"
          :key="digit"
          variant="tonal"
          size="large"
          class="lock-key"
          :aria-label="`Angka ${digit}`"
          @click="press(digit)"
        >
          {{ digit }}
        </v-btn>
        <v-btn
          variant="text"
          size="large"
          class="lock-key"
          aria-label="Hapus satu angka"
          @click="backspace"
        >
          <v-icon icon="mdi-backspace-outline" />
        </v-btn>
        <v-btn
          v-for="digit in ['0']"
          :key="digit"
          variant="tonal"
          size="large"
          class="lock-key"
          :aria-label="`Angka ${digit}`"
          @click="press(digit)"
        >
          {{ digit }}
        </v-btn>
        <v-btn
          color="primary"
          variant="flat"
          size="large"
          class="lock-key"
          aria-label="Buka aplikasi"
          :loading="busy"
          @click="submit"
        >
          <v-icon icon="mdi-check" />
        </v-btn>
      </div>

      <v-alert v-if="error" type="error" density="compact" class="form-alert">
        {{ error }}
      </v-alert>
    </div>
  </div>
</template>

<style scoped>
.lock-screen {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--v-theme-background));
}

.lock-card {
  width: min(360px, 100%);
  text-align: center;
}

.lock-dots {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin: 24px 0 8px;
}

.lock-dots span {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid rgb(var(--v-theme-primary));
}

.lock-dots span.is-filled {
  background: rgb(var(--v-theme-primary));
}

.lock-sub {
  min-height: 20px;
  margin-bottom: 8px;
}

.lock-pad {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-top: 16px;
}

.lock-key {
  min-height: 56px;
}
</style>
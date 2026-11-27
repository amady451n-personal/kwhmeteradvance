<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useTheme } from 'vuetify'
import AppShell from './layouts/AppShell.vue'
import LockView from './views/LockView.vue'
import { useAppStore } from './stores/app'

const theme = useTheme()
const store = useAppStore()

const locked = computed(
  () => store.mode === 'server' && store.session != null && !store.session.authenticated,
)

let media: MediaQueryList | null = null
let timer = 0
let syncTimer = 0

function syncTheme() {
  if (!media) return
  const dark = media.matches
  theme.change(dark ? 'dark' : 'light')
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', dark ? '#0C1513' : '#F6F3EB')
}

onMounted(() => {
  media = window.matchMedia('(prefers-color-scheme: dark)')
  syncTheme()
  media.addEventListener('change', syncTheme)
  timer = window.setInterval(() => store.tick(), 60_000)
  syncTimer = window.setInterval(() => void store.syncPending(), 30_000)
  window.addEventListener('online', retry)
  void store.init()
})

function retry() {
  void store.init().then(() => store.syncPending())
}

onUnmounted(() => {
  media?.removeEventListener('change', syncTheme)
  window.removeEventListener('online', retry)
  window.clearInterval(timer)
  window.clearInterval(syncTimer)
})
</script>

<template>
  <LockView v-if="locked" />
  <AppShell v-else />
</template>

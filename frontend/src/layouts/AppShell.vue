<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useDisplay } from 'vuetify'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const route = useRoute()
const { mdAndUp, lgAndUp, smAndDown } = useDisplay()

const nav = [
  { to: '/', title: 'Beranda', short: 'Beranda', icon: 'mdi-home-variant-outline' },
  { to: '/riwayat', title: 'Riwayat', short: 'Riwayat', icon: 'mdi-history' },
  { to: '/token', title: 'Token', short: 'Token', icon: 'mdi-ticket-confirmation-outline' },
  { to: '/laporan', title: 'Laporan', short: 'Laporan', icon: 'mdi-file-chart-outline' },
]

const rail = computed(() => mdAndUp.value && !lgAndUp.value)
const pageTitle = computed(() => (route.meta.title as string | undefined) ?? 'meteran')

function isActive(path: string) {
  return path === '/' ? route.path === '/' : route.path.startsWith(path)
}
</script>

<template>
  <v-app>
    <v-navigation-drawer
      v-if="mdAndUp"
      permanent
      :rail="rail"
      :expand-on-hover="rail"
      :width="252"
      class="nav-drawer"
    >
      <div class="brand" :class="{ 'brand--rail': rail }">
        <span class="brand-word">meteran</span>
        <span v-if="!rail" class="brand-unit">kWh</span>
      </div>

      <v-list nav density="comfortable" class="nav-list">
        <v-list-item
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          :prepend-icon="item.icon"
          :title="item.title"
          :aria-label="item.title"
          rounded="lg"
          class="nav-item"
        />
      </v-list>

      <template #append>
        <v-list nav density="comfortable" class="nav-list nav-list--foot">
          <v-list-item
            to="/pengaturan"
            prepend-icon="mdi-cog-outline"
            title="Pengaturan"
            aria-label="Pengaturan"
            rounded="lg"
            class="nav-item"
          />
        </v-list>
      </template>
    </v-navigation-drawer>

    <v-app-bar flat height="64" class="top-bar">
      <v-app-bar-title class="top-title">
        <span v-if="smAndDown" class="wordmark">meteran</span>
        <span v-else>{{ pageTitle }}</span>
      </v-app-bar-title>

      <v-chip
        v-if="store.queuedCount > 0"
        size="small"
        variant="tonal"
        color="warning"
        class="status-chip"
        :aria-label="`${store.queuedCount} catatan menunggu antrean`"
      >
        <v-icon icon="mdi-cloud-upload-outline" size="16" start />
        {{ store.queuedCount }} antre
      </v-chip>

      <v-chip
        size="small"
        variant="tonal"
        :color="store.inputStatus.color"
        class="status-chip"
        :aria-label="`Catatan terakhir: ${store.inputStatus.label}`"
      >
        <span class="status-dot" aria-hidden="true" />
        {{ store.inputStatus.label }}
      </v-chip>

      <v-btn
        icon="mdi-cog-outline"
        variant="text"
        aria-label="Pengaturan"
        to="/pengaturan"
        class="settings-btn"
      />
    </v-app-bar>

    <v-main class="app-main">
      <RouterView />
    </v-main>

    <nav v-if="smAndDown" class="bottom-nav" aria-label="Navigasi utama">
      <RouterLink
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="bottom-link"
        :class="{ 'is-active': isActive(item.to) }"
        :aria-current="isActive(item.to) ? 'page' : undefined"
      >
        <v-icon :icon="item.icon" size="22" />
        <span>{{ item.short }}</span>
      </RouterLink>
    </nav>

    <v-snackbar v-model="store.toast.open" :timeout="3200" location="bottom" class="app-toast">
      {{ store.toast.message }}
      <template #actions>
        <v-btn variant="text" @click="store.toast.open = false">Tutup</v-btn>
      </template>
    </v-snackbar>
  </v-app>
</template>

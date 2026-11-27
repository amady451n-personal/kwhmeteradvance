import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@fontsource-variable/plus-jakarta-sans'
import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles'
import './styles/main.css'
import './styles/dashboard.css'
import './styles/views.css'
import './styles/dialogs.css'
import App from './App.vue'
import router from './router'
import { vuetify } from './plugins/vuetify'

createApp(App).use(createPinia()).use(router).use(vuetify).mount('#app')

if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {})
  })
  navigator.serviceWorker.addEventListener('message', (event) => {
    if (event.data?.type === 'reload') window.location.reload()
  })
}

if (new URLSearchParams(location.search).has('debug')) {
  window.setTimeout(() => {
    const app = document.querySelector('.v-application') as HTMLElement | null
    const nav = document.querySelector('.bottom-nav') as HTMLElement | null
    const debug = document.createElement('pre')
    debug.style.cssText =
      'position:fixed;left:0;bottom:0;z-index:99999;margin:0;padding:6px;background:#fff;color:#000;font:11px monospace;white-space:pre-wrap;max-width:100vw'
    debug.textContent = [
      `inner=${window.innerWidth} client=${document.documentElement.clientWidth} scroll=${document.documentElement.scrollWidth} body=${document.body.scrollWidth}`,
      `app=${app?.getBoundingClientRect().width ?? 'n/a'} nav=${nav?.getBoundingClientRect().width ?? 'n/a'}`,
    ].join('\n')
    document.body.appendChild(debug)
  }, 1200)
}

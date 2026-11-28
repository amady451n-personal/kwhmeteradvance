import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'

const code = readFileSync(new URL('../public/sw.js', import.meta.url), 'utf8')

test('service worker navigation handler serves a response without throwing', async () => {
  const handlers = {}
  const pending = []
  const response = { ok: true, status: 200, clone: () => response, text: async () => '<html></html>' }
  const cache = { put: async () => {}, add: async () => {}, match: async () => undefined }
  const sandbox = {
    URL,
    Promise,
    setTimeout,
    console,
    Response: { error: () => response },
    fetch: async () => response,
    caches: {
      keys: async () => [],
      open: async () => cache,
      match: async () => undefined,
      delete: async () => true,
    },
    self: {
      location: { origin: 'https://example.test' },
      addEventListener: (name, fn) => {
        handlers[name] = fn
      },
      skipWaiting: () => {},
      clients: { claim: () => {}, matchAll: async () => [], openWindow: async () => {} },
      registration: { showNotification: async () => {} },
    },
  }
  vm.createContext(sandbox)
  vm.runInContext(code, sandbox)

  let result
  handlers.fetch({
    request: { method: 'GET', mode: 'navigate', url: 'https://example.test/' },
    respondWith: (promise) => {
      result = promise
    },
    waitUntil: (promise) => pending.push(promise),
  })
  assert.equal((await result).ok, true)
  await Promise.all(pending)
})

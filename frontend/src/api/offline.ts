// Local PIN gate and encrypted device snapshot. No plaintext PIN is stored.
const PIN_KEY = 'meteran.pin.v1'
const DATA_KEY = 'meteran.key.v1'
const SNAPSHOT_KEY = 'meteran.snapshot.v1'
const encoder = new TextEncoder()
const decoder = new TextDecoder()

function toBase64(bytes: Uint8Array): string {
  let raw = ''
  for (const byte of bytes) raw += String.fromCharCode(byte)
  return btoa(raw)
}

function fromBase64(value: string): Uint8Array {
  const raw = atob(value)
  return Uint8Array.from(raw, (char) => char.charCodeAt(0))
}

function equal(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i]
  return diff === 0
}

interface PinTrace {
  v: 1
  salt: string
  verifier: string
}

function trace(): PinTrace | null {
  const raw = localStorage.getItem(PIN_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as PinTrace
    return parsed.v === 1 && parsed.salt && parsed.verifier ? parsed : null
  } catch {
    return null
  }
}

async function derive(pin: string, salt: Uint8Array): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey('raw', encoder.encode(pin), 'PBKDF2', false, [
    'deriveBits',
  ])
  const bits = await crypto.subtle.deriveBits(
    { name: 'PBKDF2', hash: 'SHA-256', salt: salt as BufferSource, iterations: 120_000 },
    key,
    256,
  )
  return new Uint8Array(bits)
}

export function hasPinTrace(): boolean {
  return trace() !== null
}

export async function rememberPin(pin: string): Promise<void> {
  const salt = crypto.getRandomValues(new Uint8Array(16))
  const verifier = await derive(pin, salt)
  const value: PinTrace = { v: 1, salt: toBase64(salt), verifier: toBase64(verifier) }
  localStorage.setItem(PIN_KEY, JSON.stringify(value))
}

export async function verifyPin(pin: string): Promise<boolean> {
  const saved = trace()
  if (!saved) return false
  const candidate = await derive(pin, fromBase64(saved.salt))
  return equal(candidate, fromBase64(saved.verifier))
}

export async function ensureDataKey(): Promise<CryptoKey> {
  let raw = localStorage.getItem(DATA_KEY)
  if (!raw) {
    raw = toBase64(crypto.getRandomValues(new Uint8Array(32)))
    localStorage.setItem(DATA_KEY, raw)
  }
  return crypto.subtle.importKey('raw', fromBase64(raw) as BufferSource, 'AES-GCM', false, [
    'encrypt',
    'decrypt',
  ])
}

export async function sealText(text: string): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const sealed = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    await ensureDataKey(),
    encoder.encode(text),
  )
  const body = new Uint8Array(iv.length + sealed.byteLength)
  body.set(iv, 0)
  body.set(new Uint8Array(sealed), iv.length)
  return toBase64(body)
}

export async function openText(sealed: string): Promise<string> {
  const body = fromBase64(sealed)
  const decrypted = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: body.subarray(0, 12) as BufferSource },
    await ensureDataKey(),
    body.subarray(12) as BufferSource,
  )
  return decoder.decode(decrypted)
}

export async function saveSnapshot(data: unknown): Promise<void> {
  localStorage.setItem(SNAPSHOT_KEY, await sealText(JSON.stringify(data)))
}

export async function loadSnapshot<T>(): Promise<T | null> {
  const raw = localStorage.getItem(SNAPSHOT_KEY)
  if (!raw) return null
  try {
    return JSON.parse(await openText(raw)) as T
  } catch {
    return null
  }
}

export function clearOffline(): void {
  localStorage.removeItem(PIN_KEY)
  localStorage.removeItem(SNAPSHOT_KEY)
}

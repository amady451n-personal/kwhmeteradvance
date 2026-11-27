/** Return the fallback when a request hangs; never leave the caller waiting. */
export async function withTimeout<T>(work: Promise<T>, ms: number, fallback: () => T): Promise<T> {
  return Promise.race([
    work,
    new Promise<T>((resolve) => setTimeout(() => resolve(fallback()), ms)),
  ])
}

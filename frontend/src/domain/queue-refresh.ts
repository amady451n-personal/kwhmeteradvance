/** Only re-load from the server after actually draining pending work. */
export function shouldRefreshAfterDrain(before: number, remaining: number): boolean {
  return before > 0 && remaining === 0
}

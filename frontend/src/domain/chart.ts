/** Index of the point closest to a 0..1 horizontal position on a chart. */
export function nearestIndex(ratio: number, xs: number[]): number | null {
  if (xs.length === 0) return null
  let best = 0
  let distance = Infinity
  for (let i = 0; i < xs.length; i++) {
    const d = Math.abs(xs[i] - ratio)
    if (d < distance) {
      distance = d
      best = i
    }
  }
  return best
}

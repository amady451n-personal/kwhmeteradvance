import { nextTick, watch } from 'vue'

/**
 * Vuetify only returns focus to an element passed as `activator`. Dialogs that
 * open from a plain `@click` would drop focus on <body>, so remember the
 * trigger and hand focus back when the dialog closes.
 */
export function useFocusReturn(isOpen: () => boolean) {
  let previous: HTMLElement | null = null

  watch(
    isOpen,
    (open) => {
      if (open) {
        const el = document.activeElement
        previous = el instanceof HTMLElement ? el : null
        return
      }

      const target = previous
      previous = null
      if (!target || target === document.body || !target.isConnected) return
      void nextTick(() => target.focus())
    },
    { flush: 'pre' },
  )
}

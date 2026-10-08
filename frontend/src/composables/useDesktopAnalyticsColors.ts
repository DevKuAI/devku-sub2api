import { computed, ref, type Ref } from 'vue'
import { useMutationObserver } from '@vueuse/core'

export function useDesktopAnalyticsColors(root: Ref<HTMLElement | null>) {
  const themeRevision = ref(0)
  useMutationObserver(document.documentElement, () => { themeRevision.value++ }, { attributes: true, attributeFilter: ['class'] })
  return computed(() => {
    // The revision invalidates canvas colors when the CSS theme changes.
    void themeRevision.value
    const element = root.value
    if (!element) return { line: '', fill: '', text: '', grid: '' }
    const style = getComputedStyle(element)
    const read = (role: string) => style.getPropertyValue(`--analytics-${role}`).trim()
    return { line: read('line'), fill: read('fill'), text: read('text'), grid: read('grid') }
  })
}

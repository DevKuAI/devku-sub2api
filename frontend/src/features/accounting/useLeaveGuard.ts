import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, useRouter, type RouteLocationRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'

// Tracks unsaved form changes against a baseline and asks before leaving the page.
export function useLeaveGuard(snapshot: () => unknown) {
  const { t } = useI18n()
  const router = useRouter()
  const baseline = ref<string | null>(null)
  let allowLeave = false

  const serialize = () => JSON.stringify(snapshot())
  const dirty = computed(() => baseline.value !== null && serialize() !== baseline.value)

  function markClean() {
    baseline.value = serialize()
  }
  function confirmDiscard() {
    return !dirty.value || window.confirm(t('accounting.form.discardChanges'))
  }
  async function leave(to: RouteLocationRaw, force = false) {
    if (!force && !confirmDiscard()) return
    allowLeave = true
    await router.push(to)
  }
  function beforeUnload(event: BeforeUnloadEvent) {
    if (!dirty.value) return
    event.preventDefault()
    event.returnValue = ''
  }

  onBeforeRouteLeave(() => allowLeave || confirmDiscard())
  onMounted(() => window.addEventListener('beforeunload', beforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))

  return { dirty, markClean, leave }
}

import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const drafts = new Map<string, unknown>()

export function useDesktopForm(snapshot: () => unknown, busy: () => boolean, organizationId: () => string = () => '') {
  const { t } = useI18n()
  const route = useRoute(), router = useRouter(), auth = useAuthStore()
  const key = () => `${auth.user?.id}:${organizationId()}:${route.path}`
  const baseline = ref('')
  const ready = ref(false)
  const dirty = computed(() => ready.value && JSON.stringify(snapshot()) !== baseline.value)
  let allow = false
  const mayLeave = () => allow || (!busy() && (!dirty.value || window.confirm(t('desktopStructure.discard'))))
  onBeforeRouteLeave(mayLeave)
  onBeforeRouteUpdate(mayLeave)
  const beforeUnload = (event: BeforeUnloadEvent) => { if (busy() || dirty.value) { event.preventDefault(); event.returnValue = '' } }
  onMounted(() => window.addEventListener('beforeunload', beforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
  return {
    dirty,
    restore: () => drafts.get(key()),
    clean() { baseline.value = JSON.stringify(snapshot()); ready.value = true; allow = false },
    async leave(path: string, force = false) {
      if (!force && !mayLeave()) return
      drafts.delete(key()); allow = true
      try { await router.push(path) } finally { allow = false }
    },
    async child(path: string) {
      drafts.set(key(), JSON.parse(JSON.stringify(snapshot()))); allow = true
      try { await router.push(path) } finally { allow = false }
    },
  }
}

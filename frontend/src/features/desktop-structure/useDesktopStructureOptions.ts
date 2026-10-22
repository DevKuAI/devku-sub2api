import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { desktopStructureError, type desktopStructure, type OrganizationUnit } from '@/api/desktopStructure'

export function useDesktopStructureOptions() {
  const { t } = useI18n()
  const departments = ref<OrganizationUnit[]>([])
  const positions = ref<OrganizationUnit[]>([])
  const loading = ref(false), error = ref('')
  let controller: AbortController | undefined
  let generation = 0

  function cancel() { generation++; controller?.abort(); loading.value = false }
  function reset() { cancel(); departments.value = []; positions.value = []; error.value = '' }
  async function load(api: ReturnType<typeof desktopStructure>, options: { status?: 'active'; positions?: boolean } = {}) {
    cancel()
    const request = generation
    controller = new AbortController()
    const { signal } = controller
    loading.value = true; error.value = ''
    const params: Record<string, string> = options.status ? { status: options.status } : {}
    try {
      const [ds, ps] = await Promise.all([
        api.all('departments', params, signal),
        options.positions === false ? Promise.resolve([]) : api.all('positions', params, signal),
      ])
      if (request === generation) { departments.value = ds; positions.value = ps }
    } catch (cause) {
      if (request === generation && !signal.aborted) { error.value = desktopStructureError(cause, t); controller?.abort() }
    } finally { if (request === generation) loading.value = false }
  }
  onBeforeUnmount(cancel)
  return { departments, positions, loading, error, load, reset }
}

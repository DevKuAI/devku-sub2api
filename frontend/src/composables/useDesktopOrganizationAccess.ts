import { computed, ref } from 'vue'
import desktopOrganizationAPI from '@/api/desktopOrganization'
import { useAuthStore } from '@/stores/auth'
import type { DesktopOrganization } from '@/api/admin/desktop'

const organization = ref<DesktopOrganization | null>(null)
const loadedForUserID = ref<number | null>(null)
let generation = 0
let pendingUserID: number | null = null
let pendingLoad: Promise<DesktopOrganization | null> | null = null

function resetDesktopOrganizationAccess(): void {
  generation++
  pendingLoad = null
  pendingUserID = null
  organization.value = null
  loadedForUserID.value = null
}

async function loadDesktopOrganizationAccess(force = false): Promise<DesktopOrganization | null> {
  const authStore = useAuthStore()
  const userID = Number(authStore.user?.id || 0)
  if (!authStore.isAuthenticated || userID <= 0) {
    resetDesktopOrganizationAccess()
    return null
  }

  if (loadedForUserID.value !== null && loadedForUserID.value !== userID) {
    resetDesktopOrganizationAccess()
  }

  if (!force && loadedForUserID.value === userID) {
    return organization.value
  }
  if (pendingLoad && pendingUserID === userID) {
    return pendingLoad
  }

  if (pendingUserID !== null && pendingUserID !== userID) resetDesktopOrganizationAccess()
  const current = ++generation
  pendingUserID = userID
  pendingLoad = desktopOrganizationAPI.getOrganization()
    .then((result) => {
      if (current === generation && authStore.isAuthenticated && Number(authStore.user?.id || 0) === userID) {
        organization.value = result
        loadedForUserID.value = userID
      }
      return current === generation && authStore.isAuthenticated && Number(authStore.user?.id || 0) === userID ? result : null
    })
    .catch(() => {
      if (current === generation && authStore.isAuthenticated && Number(authStore.user?.id || 0) === userID) {
        resetDesktopOrganizationAccess()
      }
      return null
    })
    .finally(() => {
      if (current === generation && pendingUserID === userID) {
        pendingUserID = null
        pendingLoad = null
      }
    })

  return pendingLoad
}

export function useDesktopOrganizationAccess() {
  return {
    managedDesktopOrganization: computed(() => organization.value),
    canManageDesktopOrganization: computed(() => organization.value !== null),
    refreshDesktopOrganizationAccess: loadDesktopOrganizationAccess,
    resetDesktopOrganizationAccess,
  }
}

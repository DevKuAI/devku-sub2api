import { computed, ref } from 'vue'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { useAuthStore } from '@/stores/auth'

const hasAccounts = ref(false)
const loadedForUserID = ref<number | null>(null)
let pendingUserID: number | null = null
let pendingLoad: Promise<boolean> | null = null
let requestGeneration = 0

function currentUserID(): number | null {
  const authStore = useAuthStore()
  const userID = Number(authStore.user?.id || 0)
  return userID > 0 ? userID : null
}

function resetSubscriptionAccountAccess(): void {
  requestGeneration += 1
  hasAccounts.value = false
  loadedForUserID.value = null
  pendingUserID = null
  pendingLoad = null
}

function setSubscriptionAccountAccess(value: boolean): void {
  requestGeneration += 1
  hasAccounts.value = value
  loadedForUserID.value = currentUserID()
  pendingUserID = null
  pendingLoad = null
}

async function loadSubscriptionAccountAccess(force = false): Promise<boolean> {
  const authStore = useAuthStore()
  const userID = Number(authStore.user?.id || 0)
  if (!authStore.isAuthenticated || userID <= 0) {
    resetSubscriptionAccountAccess()
    return false
  }
  if (loadedForUserID.value !== null && loadedForUserID.value !== userID) {
    resetSubscriptionAccountAccess()
  }
  if (!force && loadedForUserID.value === userID) return hasAccounts.value
  if (!force && pendingLoad && pendingUserID === userID) return pendingLoad

  const generation = ++requestGeneration
  const load = subscriptionAccountsAPI.list(false)
    .then((accounts) => {
      const result = accounts.length > 0
      if (generation === requestGeneration && Number(authStore.user?.id || 0) === userID) {
        hasAccounts.value = result
        loadedForUserID.value = userID
      }
      return result
    })
    .catch(() => {
      if (generation === requestGeneration && Number(authStore.user?.id || 0) === userID) {
        return hasAccounts.value
      }
      return false
    })
    .finally(() => {
      if (generation === requestGeneration && pendingLoad === load) {
        pendingUserID = null
        pendingLoad = null
      }
    })
  pendingUserID = userID
  pendingLoad = load
  return load
}

export function useSubscriptionAccountAccess() {
  return {
    hasSubscriptionAccounts: computed(() => hasAccounts.value),
    refreshSubscriptionAccountAccess: loadSubscriptionAccountAccess,
    setSubscriptionAccountAccess,
    resetSubscriptionAccountAccess,
  }
}

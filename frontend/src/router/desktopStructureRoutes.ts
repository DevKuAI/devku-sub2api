import type { RouteRecordRaw } from 'vue-router'

export const desktopStructureRoutes: RouteRecordRaw[] = [false, true].flatMap(selfManaged => {
  const base = selfManaged ? '/desktop/organization' : '/admin/desktop/organizations/:organizationId'
  const meta = { requiresAuth: true, requiresAdmin: !selfManaged, requiresDesktop: true, titleKey: 'desktopStructure.title' }
  const shared = { selfManaged }
  const routes: RouteRecordRaw[] = [
    { path: `${base}/structure`, component: () => import('@/features/desktop-structure/DesktopStructureView.vue'), props: shared, meta },
    { path: `${base}/members/new`, component: () => import('@/features/desktop-structure/DesktopMemberFormView.vue'), props: shared, meta },
    { path: `${base}/members/import`, component: () => import('@/features/desktop-structure/DesktopMemberImportView.vue'), props: shared, meta },
    { path: `${base}/members/:memberId/edit`, component: () => import('@/features/desktop-structure/DesktopMemberFormView.vue'), props: shared, meta },
    { path: `${base}/members/:memberId/position`, component: () => import('@/features/desktop-structure/DesktopMemberPositionView.vue'), props: shared, meta },
  ]
  for (const kind of ['departments', 'positions'] as const) {
    routes.push({ path: `${base}/${kind}/new`, component: () => import('@/features/desktop-structure/DesktopUnitFormView.vue'), props: { ...shared, kind }, meta })
    routes.push({ path: `${base}/${kind}/:unitId/edit`, component: () => import('@/features/desktop-structure/DesktopUnitFormView.vue'), props: { ...shared, kind }, meta })
  }
  return routes
})

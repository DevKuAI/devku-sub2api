import type { RouteRecordRaw } from 'vue-router'

function children(prefix: string): RouteRecordRaw[] {
  return [
    { path: '', name: `${prefix}Access`, component: () => import('@/components/bi/BIGrantManagement.vue'), meta: { titleKey: 'biNative.accessManagement' } },
    { path: 'invitations/new', name: `${prefix}InvitationNew`, component: () => import('@/components/bi/BIGrantForm.vue'), meta: { titleKey: 'biNative.createInvite' } },
    { path: 'users/:biUserId/edit', name: `${prefix}AccessEdit`, component: () => import('@/components/bi/BIGrantForm.vue'), meta: { titleKey: 'biNative.editAccess' } },
    { path: 'invitations/:invitationId', name: `${prefix}InvitationResult`, component: () => import('@/components/bi/BIInvitationDetails.vue'), meta: { titleKey: 'biNative.created' } },
  ]
}

export const biRoutes: RouteRecordRaw[] = [
  {
    path: '/admin/desktop/organizations/:organizationId/bi',
    component: () => import('@/views/admin/BIWorkspaceView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
    children: children('AdminBI'),
  },
  {
    path: '/desktop/organization/bi',
    component: () => import('@/views/admin/BIWorkspaceView.vue'),
    props: { selfManaged: true },
    meta: { requiresAuth: true, requiresAdmin: false, requiresDesktop: true },
    children: children('OrganizationBI'),
  },
]

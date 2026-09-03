import { beforeEach, describe, expect, it, vi } from 'vitest'

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient: apiClientMock }))

import desktopOrganizationAPI from '@/api/desktopOrganization'

describe('Desktop managed organization API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('treats a 204 response as no associated organization', async () => {
    apiClientMock.get.mockResolvedValue({ data: '', status: 204 })

    await expect(desktopOrganizationAPI.getOrganization()).resolves.toBeNull()
    expect(apiClientMock.get).toHaveBeenCalledWith('/desktop/organization')
  })

  it('uses user-scoped paths without accepting an organization ID', async () => {
    apiClientMock.get.mockResolvedValue({ data: { items: [] } })
    apiClientMock.patch.mockResolvedValue({ data: {} })
    apiClientMock.put.mockResolvedValue({ data: {} })
    apiClientMock.post.mockResolvedValue({ data: {} })
    apiClientMock.delete.mockResolvedValue({ data: { deleted: true } })

    await desktopOrganizationAPI.updateOrganization('org_other', { name: 'Managed' })
    await desktopOrganizationAPI.listMembers('org_other', 2, 50, { search: 'name', status: 'active' })
    await desktopOrganizationAPI.updateModelConfiguration('org_other', { schema_version: 1, targets: {} })
    await desktopOrganizationAPI.createMember('org_other', { name: 'Member', phone: '13800138000' })
    await desktopOrganizationAPI.updateMember('org_other', 'mem one', { status: 'disabled' })
    await desktopOrganizationAPI.deleteMember('org_other', 'mem one')
    await desktopOrganizationAPI.rotateModelToken('org_other', 'mem one')

    expect(apiClientMock.patch.mock.calls[0][0]).toBe('/desktop/organization')
    expect(apiClientMock.get.mock.calls[0][0]).toBe('/desktop/organization/members')
    expect(apiClientMock.put.mock.calls[0][0]).toBe('/desktop/organization/model-configuration')
    expect(apiClientMock.post.mock.calls[0][0]).toBe('/desktop/organization/members')
    expect(apiClientMock.patch.mock.calls[1][0]).toBe('/desktop/organization/members/mem%20one')
    expect(apiClientMock.delete.mock.calls[0][0]).toBe('/desktop/organization/members/mem%20one')
    expect(apiClientMock.post.mock.calls[1][0]).toBe('/desktop/organization/members/mem%20one/model-token/rotate')
    expect(apiClientMock.post.mock.calls[0][2].headers['Idempotency-Key']).toMatch(/^desktop-managed-member-create-/)
    expect(apiClientMock.post.mock.calls[1][2].headers['Idempotency-Key']).toMatch(/^desktop-managed-model-token-rotate-/)
  })
})

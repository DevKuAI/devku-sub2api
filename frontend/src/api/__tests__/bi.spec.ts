import { beforeEach, describe, expect, it, vi } from 'vitest'
import { biManagement } from '../bi'
import { apiClient } from '../client'
vi.mock('../client', () => ({ apiClient: { post: vi.fn() } }))
beforeEach(() => vi.clearAllMocks())
describe('BI official invitation code API', () => {
  it.each([
    [false, '/admin/desktop/operations/organizations/org%2Fone'],
    [true, '/desktop/organization/bi'],
  ])('uses the correct management scope (owner=%s) and release version', async (owner, base) => {
    const response = { image_base64: 'aW1hZ2U=', mime_type: 'image/png' }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: response })
    await expect(biManagement('org/one', owner).qrcode('invite/one', 's'.repeat(32))).resolves.toEqual(response)
    expect(apiClient.post).toHaveBeenCalledWith(`${base}/invitations/invite%2Fone/qrcode`, { scene: 's'.repeat(32), env_version: 'release' })
  })
})

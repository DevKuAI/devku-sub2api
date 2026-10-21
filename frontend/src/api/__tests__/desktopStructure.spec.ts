import { beforeEach, describe, expect, it, vi } from 'vitest'
const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn() }))
vi.mock('../client', () => ({ apiClient: http }))
import { desktopStructure, desktopStructureError, safeDesktopReturn } from '../desktopStructure'

describe('Desktop organization structure client', () => {

  it('explains structured server errors and preserves plain error messages', () => {
    const t = (key: string) => key
    expect(desktopStructureError({ reason: 'ORGANIZATION_STRUCTURE_CONFLICT', message: 'stale' }, t)).toBe('desktopStructure.conflict')
    expect(desktopStructureError({ reason: 'ORGANIZATION_UNIT_NAME_EXISTS' }, t)).toBe('desktopStructure.nameExists')
    expect(desktopStructureError({ message: 'Connection failed' }, t)).toBe('Connection failed')
    expect(desktopStructureError(new Error('Unavailable'), t)).toBe('Unavailable')
  })
  beforeEach(() => vi.clearAllMocks())
  it.each([false, true])('loads every position page with selfManaged=%s', async selfManaged => {
    http.get.mockResolvedValueOnce({ data: { items: [{ public_id: 'pos_one' }], total: 2 } }).mockResolvedValueOnce({ data: { items: [{ public_id: 'pos_two' }], total: 2 } })
    const rows = await desktopStructure('org_one', selfManaged).all('positions')
    expect(rows).toHaveLength(2)
    expect(http.get).toHaveBeenNthCalledWith(2, `${selfManaged ? '/desktop/organization' : '/admin/desktop/organizations/org_one'}/positions`, { params: { page: 2, page_size: 100 } })
  })
  it('submits an assignment with its explicit idempotency key', async () => {
    http.put.mockResolvedValue({ data: { public_id: 'mem_one' } })
    const input = { position_id: 'pos_one', expected_revision: 1, reason: '' }
    await desktopStructure('org_one', true).assign('mem_one', input, 'assignment-key')
    expect(http.put).toHaveBeenCalledWith('/desktop/organization/members/mem_one/position', input, { headers: { 'Idempotency-Key': 'assignment-key' } })
  })
  it('keeps return navigation inside the current enterprise after URL normalization', () => {
    const base = '/admin/desktop/organizations/org_one'
    for (const path of ['https://example.com', '//example.com', `${base}/../../outside`, `${base}/%2e%2e/outside`, '/admin/desktop/organizations/org_other', `${base}\\outside`]) expect(safeDesktopReturn(path, base)).toBe(base)
    expect(safeDesktopReturn(`${base}?tab=members&page=2`, base)).toBe(`${base}?tab=members&page=2`)
  })
})

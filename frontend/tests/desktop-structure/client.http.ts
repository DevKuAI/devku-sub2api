import { expect, it } from 'vitest'
import { apiClient } from '@/api/client'
import { authStorage } from '@/utils/authStorage'
import { desktopStructure, desktopStructureError, structureIdempotencyKey } from '@/api/desktopStructure'

// Invoked by the Go integration fixture against a real HTTP server and PostgreSQL.
it('runs the actual Web client through organization structure and assignment operations', async () => {
  expect(process.env.DESKTOP_STRUCTURE_HTTP_URL).toBeTruthy()
  const memberId = process.env.DESKTOP_STRUCTURE_HTTP_MEMBER!
  const organizationId = process.env.DESKTOP_STRUCTURE_HTTP_ORGANIZATION!
  apiClient.defaults.baseURL = process.env.DESKTOP_STRUCTURE_HTTP_URL!
  apiClient.defaults.adapter = 'http'
  authStorage.setItem('auth_token', 'desktop-structure-http-test')
  const api = desktopStructure(organizationId, true)
  expect((await api.organization())?.public_id).toBe(organizationId)

  const department = await api.save('departments', '', { name: 'HTTP 联调部门' }, structureIdempotencyKey())
  const position = await api.save('positions', '', { name: '集成工程师', department_id: department.public_id }, structureIdempotencyKey())
  expect((await api.all('positions', { department_id: department.public_id })).map(item => item.public_id)).toEqual([position.public_id])

  const created = await api.members.createMember(organizationId, { name: 'HTTP 新员工', phone: process.env.DESKTOP_STRUCTURE_HTTP_PHONE!, position_id: position.public_id, budget: { quota: 1, limit_1d: 0, limit_7d: 0 } }, structureIdempotencyKey())
  expect(created.position?.public_id).toBe(position.public_id)
  expect(created.department?.public_id).toBe(department.public_id)
  expect(created.budget?.quota).toBe(1)
  expect(created.model_token_status).toBe('active')
  expect(JSON.stringify(created)).not.toMatch(/"(?:api_key|model_token|key)":/)

  const before = await api.member(memberId)
  const payload = { position_id: position.public_id, expected_revision: before.position_revision!, reason: '真实 Web client 联调' }
  const key = structureIdempotencyKey()
  const updated = await api.assign(memberId, payload, key)
  expect(updated.department?.public_id).toBe(department.public_id)
  expect(updated.position?.public_id).toBe(position.public_id)
  expect((await api.unit('positions',position.public_id)).member_count).toBe(2)
  expect((await api.assign(memberId, payload, key)).position_revision).toBe(updated.position_revision)
  await expect(api.assign(memberId, payload, structureIdempotencyKey())).rejects.toMatchObject({ status: 409, reason: 'ORGANIZATION_STRUCTURE_CONFLICT' })

  await api.save('positions', position.public_id, { name: '已更名岗位', expected_revision: position.revision }, structureIdempotencyKey())
  expect((await api.member(memberId)).position?.name).toBe('已更名岗位')
  const history = await api.history(memberId)
  expect(history.total).toBe(2)
  expect(history.items[0].position.name).toBe('集成工程师')
  expect(history.items[0].source).toBe('transfer')
  expect(history.items[1].ended_at).not.toBeNull()
  expect((await api.status()).unassigned_members).toBe(0)
  expect(JSON.stringify(updated)).not.toContain('sk-desktop-')

  try {
    await api.save('departments', '', { name: 'HTTP 联调部门' }, structureIdempotencyKey())
    throw new Error('Duplicate department was accepted')
  } catch (cause) {
    expect(cause).toMatchObject({ status: 409, reason: 'ORGANIZATION_UNIT_NAME_EXISTS' })
    expect(desktopStructureError(cause, key => key)).toBe('desktopStructure.nameExists')
  }
  console.log('Desktop structure TypeScript HTTP flow verified')
})

import { describe, expect, it } from 'vitest'
import type { DesktopMember } from '@/api/admin/desktop'
import { createDesktopMemberUsageCsv } from '../desktopMemberUsageCsv'

const organization = { name: '测试企业', code: 'company' }
const member: DesktopMember = {
  public_id: 'mem_one',
  name: '成员甲',
  phone: '+8613800000000',
  status: 'active',
  model_token_status: 'active',
  usage: {
    today_tokens: 1250,
    last_30_days_tokens: 2500000,
    total_tokens: 4000000000,
    today_actual_cost: 0.12345678,
    last_30_days_actual_cost: 2.5,
    total_actual_cost: 4000.00000001,
  },
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-24T01:23:45Z',
}
const t = (key: string) => key

describe('Desktop member usage CSV', () => {
  it('includes a BOM and preserves complete costs, token counts and phone prefixes', () => {
    const csv = createDesktopMemberUsageCsv(organization, [member], t)
    expect(csv.charCodeAt(0)).toBe(0xFEFF)
    expect(csv.split('\r\n')[1]).toBe(
      "测试企业,company,mem_one,成员甲,'+8613800000000,common.active,admin.desktop.tokenStatus.active,0.12345678,2.5,4000.00000001,1250,2500000,4000000000,2026-09-24T01:23:45Z",
    )
  })

  it('escapes commas, quotes and line breaks in organization and member names', () => {
    const csv = createDesktopMemberUsageCsv(
      { ...organization, name: '公司,"甲"' },
      [{ ...member, name: '成员\n乙' }],
      t,
    )
    expect(csv).toContain('"公司,""甲""",company,mem_one,"成员\n乙",')
  })

  it.each(['=1+1', '+1+1', '-1+1', '@SUM(1)', '  =1+1', '\t=1+1', '\r=1+1'])('treats formula-like text %j as text', name => {
    const csv = createDesktopMemberUsageCsv(organization, [{ ...member, name }], t)
    expect(csv).toContain(`'${name}`)
  })

  it('leaves unavailable usage empty and exports disabled states', () => {
    const csv = createDesktopMemberUsageCsv(organization, [{ ...member, status: 'disabled', model_token_status: 'missing', usage: undefined }], t)
    expect(csv).toContain('common.disabled,admin.desktop.tokenStatus.missing,,,,,,,2026-09-24T01:23:45Z')
  })

  it('exports only approved columns even when API objects contain raw credentials', () => {
    const unexpectedMember = {
      ...member,
      model_token: 'PRIVATE-MODEL-TOKEN',
      api_key: 'PRIVATE-API-KEY',
      credentials: { access_token: 'PRIVATE-ACCESS-TOKEN', refresh_token: 'PRIVATE-REFRESH-TOKEN' },
    }
    const unexpectedOrganization = { ...organization, model_token: 'PRIVATE-ORGANIZATION-TOKEN' }
    const csv = createDesktopMemberUsageCsv(unexpectedOrganization, [unexpectedMember], t)
    expect(csv).not.toContain('PRIVATE-')
    expect(csv).not.toMatch(/\b(?:model_token|api_key|access_token|refresh_token)\b/)
    expect(csv).toContain('admin.desktop.tokenStatus.active')
    expect(csv.split('\r\n')[1].split(',')).toHaveLength(14)
  })
})

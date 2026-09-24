import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const directory = dirname(fileURLToPath(import.meta.url))
const sources = [
  readFileSync(resolve(directory, './DesktopOrganizationsView.vue'), 'utf8'),
  readFileSync(resolve(directory, './DesktopOrganizationDetailView.vue'), 'utf8'),
  readFileSync(resolve(directory, '../../../utils/desktopMemberUsageCsv.ts'), 'utf8'),
].join('\n')

describe('Desktop Admin credential safety', () => {
  it('never renders or exports raw credentials and only displays model token status', () => {
    expect(sources).not.toMatch(/\b(?:model_token|api_key|access_token|refresh_token)\b/)
    expect(sources).not.toContain('clipboard')
    expect(sources).toContain('model_token_status')
  })
})

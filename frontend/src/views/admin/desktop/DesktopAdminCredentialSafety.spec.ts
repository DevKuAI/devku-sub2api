import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const directory = dirname(fileURLToPath(import.meta.url))
const sources = [
  readFileSync(resolve(directory, './DesktopOrganizationsView.vue'), 'utf8'),
  readFileSync(resolve(directory, './DesktopOrganizationDetailView.vue'), 'utf8'),
].join('\n')

describe('Desktop Admin credential safety', () => {
  it('never renders or adds copy/download controls for a raw model token', () => {
    expect(sources).not.toMatch(/\bmodel_token\b/)
    expect(sources).not.toContain('clipboard')
    expect(sources).not.toContain('download')
    expect(sources).toContain('model_token_status')
  })
})

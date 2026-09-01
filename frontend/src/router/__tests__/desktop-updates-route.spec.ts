import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('Desktop software update route', () => {
  it('requires admin access without the Desktop organization feature gate', () => {
    const route = routerSource.match(/\{\s*path: '\/admin\/desktop\/updates',[\s\S]*?\n\s{2}\},/)

    expect(route).not.toBeNull()
    expect(route?.[0]).toContain('requiresAdmin: true')
    expect(route?.[0]).not.toContain('requiresDesktop')
  })

  it('keeps simple-mode restriction scoped to organizations', () => {
    expect(routerSource).toContain("'/admin/desktop/organizations'")
    expect(routerSource).not.toMatch(/restrictedPaths = \[[\s\S]*?'\/admin\/desktop',[\s\S]*?\]/)
  })
})

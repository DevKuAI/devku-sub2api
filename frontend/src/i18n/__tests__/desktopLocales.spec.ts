import { describe, expect, it } from 'vitest'

import zh from '../locales/zh'

type LocaleValue = string | { [key: string]: LocaleValue }

function collectStrings(value: LocaleValue): string[] {
  if (typeof value === 'string') return [value]
  return Object.values(value).flatMap(collectStrings)
}

describe('Desktop locale copy', () => {
  it('uses Chinese management labels while allowing Token as a usage unit', () => {
    const copy = collectStrings(zh.admin.desktop).join('\n')

    expect(copy).not.toMatch(/\b(?:User|Group|Key|active)\b|Provider ID|Wire API/)
    const managementCopy = collectStrings({ ...zh.admin.desktop, usageStatistics: {} }).join('\n')
    // Usage units also appear in member tables, CSV exports and analysis history.
    expect(managementCopy.replace(/\bToken 用量/g, '用量')).not.toMatch(/\bToken\b/)
  })
})

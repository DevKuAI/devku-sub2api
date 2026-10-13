import { describe, expect, it } from 'vitest'
import { lastClosedPeriod } from '@/api/desktopPeriods'
describe('Enterprise summary calendar', () => {
  it('uses natural Shanghai weeks and months across UTC and year boundaries', () => { expect(lastClosedPeriod('week',new Date('2026-10-11T16:00:00Z'))).toBe('2026-10-05');expect(lastClosedPeriod('month',new Date('2025-12-31T16:00:00Z'))).toBe('2025-12-01') })
})

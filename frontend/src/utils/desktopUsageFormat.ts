export type TokenNumberMode = 'compact' | 'exact'

export function createDesktopUsageFormatter(locale: string) {
  const integer = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
  const compact = new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 2 })
  const billion = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 })
  const cost = new Intl.NumberFormat(locale, { minimumFractionDigits: 4, maximumFractionDigits: 4 })
  return {
    number: (value: number) => integer.format(value),
    cost: (value: number) => `$${cost.format(value)}`,
    tokens: (value: number, mode: TokenNumberMode = 'compact') => mode === 'exact' ? integer.format(value)
      : value >= 1_000_000_000_000 ? `${billion.format(value / 1_000_000_000)}B` : compact.format(value),
  }
}

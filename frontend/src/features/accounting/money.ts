// Exact decimal arithmetic for API amounts (strings with up to six fraction digits).
interface TaxInput {
  currency: string
  amount: string
  details: Record<string, unknown>
}
const scale = 1_000_000n
function units(raw: string): bigint {
  if (!/^-?\d+(\.\d{0,6})?$/.test(raw)) throw new Error('amount')
  const negative = raw.startsWith('-'),
    [whole, fraction = ''] = raw.replace('-', '').split('.')
  const value = BigInt(whole!) * scale + BigInt(fraction.padEnd(6, '0'))
  return negative ? -value : value
}
function value(raw: bigint): string {
  const negative = raw < 0n,
    absolute = negative ? -raw : raw
  const fraction = String(absolute % scale)
    .padStart(6, '0')
    .replace(/0+$/, '')
  return `${negative ? '-' : ''}${absolute / scale}${fraction ? `.${fraction}` : ''}`
}
export function subtractMoney(a: string, b: string) {
  try {
    return value(units(a) - units(b))
  } catch {
    return ''
  }
}
export function addMoney(a: string, b: string) {
  try {
    return value(units(a) + units(b))
  } catch {
    return ''
  }
}
export function compareMoney(a: string, b: string) {
  try {
    const difference = units(a) - units(b)
    return difference > 0n ? 1 : difference < 0n ? -1 : 0
  } catch {
    return 0
  }
}
export function positiveMoney(raw: unknown) {
  try {
    return units(String(raw)) > 0n
  } catch {
    return false
  }
}
export function taxPreview(input: TaxInput) {
  try {
    const sales = units(String(input.details.sales_amount)),
      rate = units(String(input.details.tax_rate))
    if (sales <= 0n || rate < 0n || rate > scale) return null
    const digits =
      new Intl.NumberFormat('en', {
        style: 'currency',
        currency: input.currency,
      }).resolvedOptions().maximumFractionDigits ?? 2
    const step = 10n ** BigInt(6 - digits)
    const numerator = sales * rate,
      denominator = input.details.tax_inclusive ? scale + rate : scale
    const tax =
      ((numerator + (denominator * step) / 2n) / (denominator * step)) * step
    const net = input.details.tax_inclusive ? sales - tax : sales
    const receivable = input.details.tax_inclusive ? sales : sales + tax
    return {
      tax: value(tax),
      net: value(net),
      receivable: value(receivable),
      difference: subtractMoney(String(input.amount), value(receivable)),
    }
  } catch {
    return null
  }
}

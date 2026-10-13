// Format decimal API amounts without losing cents through a floating-point conversion.
export function formatMoney(
  value: string | number | undefined,
  currency: string,
): string {
  const raw = String(value ?? '0')
  try {
    const [integer, fraction = ''] = raw.split('.')
    const negativeZero = integer === '-0'
    const formatter = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
    })
    const digits = formatter.resolvedOptions().maximumFractionDigits ?? 2
    return formatter
      .formatToParts(BigInt(negativeZero ? '-1' : integer || '0'))
      .map((part) => {
        if (part.type === 'fraction')
          return fraction.padEnd(digits, '0').slice(0, digits)
        if (negativeZero && part.type === 'integer') return '0'
        return part.value
      })
      .join('')
  } catch {
    return `${currency} ${raw}`
  }
}

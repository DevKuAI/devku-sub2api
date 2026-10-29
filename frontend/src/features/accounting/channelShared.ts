import type { ChannelProblem } from './types'

type Translate = (key: string, values?: Record<string, unknown>) => string
type Exists = (key: string) => boolean

// Wallet and line type codes come from the platform side; unknown codes are shown as they are.
export function walletLabel(wallet: string, t: Translate, te: Exists) {
  const key = `accounting.channels.wallets.${wallet}`
  return te(key) ? t(key) : wallet
}

export function lineTypeLabel(type: string, t: Translate, te: Exists) {
  const key = `accounting.channels.types.${type}`
  return te(key) ? t(key) : type
}

// The import endpoints list every rejected row in error metadata.
export function uploadProblems(cause: unknown): ChannelProblem[] {
  const raw = (cause as { metadata?: { problems?: string } }).metadata?.problems
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    return Array.isArray(parsed) ? (parsed as ChannelProblem[]) : []
  } catch {
    return []
  }
}

export function saveFile(name: string, content: Blob | string, type = 'text/csv;charset=utf-8') {
  const blob = typeof content === 'string' ? new Blob([content], { type }) : content
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
}

export function signedClass(amount: string) {
  return amount.startsWith('-') ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-400'
}

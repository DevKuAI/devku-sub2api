// Match Go's per-code-point strings.ToLower rather than contextual/full casing.
export function normalizeDesktopStructureName(value: string): string {
  return Array.from(value.trim().normalize('NFC'), character => character === '\u0130' ? 'i' : character.toLowerCase()).join('')
}

export function statusTone(status: string) {
  if (['RECEIVED', 'CONFIRMED', 'SIGNED', 'VERIFIED', 'PAID', 'ACTIVE', 'MATCHED'].includes(status)) return 'badge-success'
  if (['UNPAID', 'PENDING', 'DIFFERENCE', 'UNMATCHED'].includes(status)) return 'badge-warning'
  if (status === 'VOID') return 'badge-danger'
  return 'badge-gray'
}

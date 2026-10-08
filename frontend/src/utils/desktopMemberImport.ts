import type { CreateDesktopMemberRequest } from '@/api/admin/desktop'

export const DESKTOP_MEMBER_IMPORT_MAX_ROWS = 200
export const DESKTOP_MEMBER_IMPORT_MAX_BYTES = 5 * 1024 * 1024

export interface DesktopMemberImportRow {
  rowNumber: number
  input: Required<CreateDesktopMemberRequest>
  errors: string[]
}

const headers = {
  name: ['姓名', '成员', '成员姓名', 'name', 'membername'],
  phone: ['联系电话', '手机号', '手机号码', '电话', '手机', 'phone', 'mobile', 'telephone'],
  remark: ['备注', 'remark', 'notes', 'note'],
}
const text = (value: unknown) => String(value ?? '').trim().normalize('NFC')
const headerText = (value: unknown) => text(value).replace(/[\s_]/g, '').toLowerCase()

function normalizePhone(value: unknown): string {
  let phone = text(value).replace(/^'/, '').replace(/[\s()（）-]/g, '')
  if (phone.startsWith('00')) phone = '+' + phone.slice(2)
  if (/^1[3-9]\d{9}$/.test(phone)) phone = '+86' + phone
  if (/^86\d{11}$/.test(phone)) phone = '+' + phone
  return phone
}

export function parseDesktopMemberImportRows(data: unknown[][]): DesktopMemberImportRow[] {
  const headerIndex = data.slice(0, 50).findIndex(row => {
    const values = row.map(headerText)
    return headers.name.some(value => values.includes(value)) && headers.phone.some(value => values.includes(value))
  })
  if (headerIndex < 0) throw new Error('missingHeaders')
  const values = data[headerIndex].map(headerText)
  const column = (field: keyof typeof headers) => values.findIndex(value => headers[field].includes(value))
  for (const field of ['name', 'phone', 'remark'] as const) {
    if (values.filter(value => headers[field].includes(value)).length > 1) throw new Error('missingHeaders')
  }
  const names = column('name'), phones = column('phone'), remarks = column('remark')
  const seenPhones = new Set<string>()
  const rows: DesktopMemberImportRow[] = []
  for (let index = headerIndex + 1; index < data.length; index++) {
    const row = data[index]
    const input = { name: text(row[names]), phone: normalizePhone(row[phones]), remark: remarks < 0 ? '' : text(row[remarks]) }
    if (!input.name && !input.phone && !input.remark) continue
    const errors: string[] = []
    if (!input.name || Array.from(input.name).length > 100) errors.push('invalidName')
    if (!/^\+?[1-9]\d{6,14}$/.test(input.phone)) errors.push('invalidPhone')
    if (Array.from(input.remark).length > 500) errors.push('invalidRemark')
    if (input.phone && seenPhones.has(input.phone)) errors.push('duplicatePhone')
    if (input.phone) seenPhones.add(input.phone)
    rows.push({ rowNumber: index + 1, input, errors })
    if (rows.length > DESKTOP_MEMBER_IMPORT_MAX_ROWS) throw new Error('tooManyRows')
  }
  if (!rows.length) throw new Error('noRows')
  return rows
}

async function fileBuffer(file: File): Promise<ArrayBuffer> {
  if (typeof file.arrayBuffer === 'function') return file.arrayBuffer()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(new Error('invalidFile'))
    reader.readAsArrayBuffer(file)
  })
}

export async function parseDesktopMemberImportFile(file: File): Promise<DesktopMemberImportRow[]> {
  if (file.size > DESKTOP_MEMBER_IMPORT_MAX_BYTES) throw new Error('fileTooLarge')
  const extension = file.name.split('.').pop()?.toLowerCase()
  if (extension !== 'xlsx' && extension !== 'csv') throw new Error('unsupportedFile')
  let rows: unknown[][]
  try {
    const buffer = await fileBuffer(file)
    if (extension === 'xlsx') {
      const { readSheet } = await import('read-excel-file/browser')
      rows = await readSheet(buffer, 1)
    } else {
      const { default: Papa } = await import('papaparse')
      const parsed = Papa.parse<string[]>(new TextDecoder().decode(buffer), { skipEmptyLines: false })
      if (parsed.errors.some(error => error.code !== 'UndetectableDelimiter')) throw new Error('invalidFile')
      rows = parsed.data
    }
  } catch {
    throw new Error('invalidFile')
  }
  return parseDesktopMemberImportRows(rows)
}

export async function createDesktopMemberImportTemplate(): Promise<Blob> {
  const XLSX = await import('xlsx')
  const sheet = XLSX.utils.aoa_to_sheet([
    ['公司内部通讯录'], ['序号', '姓名', '联系电话', '备注'],
    ...Array.from({ length: 20 }, (_, index) => [index + 1, '', '', '']),
  ])
  sheet['!merges'] = [{ s: { r: 0, c: 0 }, e: { r: 0, c: 3 } }]
  sheet['!cols'] = [{ wch: 8 }, { wch: 20 }, { wch: 24 }, { wch: 45 }]
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, sheet, '成员')
  return new Blob([XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
}

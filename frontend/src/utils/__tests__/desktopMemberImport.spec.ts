import { describe, expect, it } from 'vitest'
import { readSheet } from 'read-excel-file/browser'
import * as XLSX from 'xlsx'
import { createDesktopMemberImportTemplate, parseDesktopMemberImportFile, parseDesktopMemberImportRows } from '../desktopMemberImport'

const buffer = (blob: Blob): Promise<ArrayBuffer> => new Promise((resolve, reject) => {
  const reader = new FileReader()
  reader.onload = () => resolve(reader.result as ArrayBuffer)
  reader.onerror = reject
  reader.readAsArrayBuffer(blob)
})

describe('Desktop member import', () => {
  it('recognizes a title row and Chinese headers, keeps remarks, and ignores serial-only blank rows', () => {
    const rows = parseDesktopMemberImportRows([
      ['公司内部通讯录'], ['序号', '姓名', '联系电话', '备注'],
      [1, ' 张明 ', 13800138000, '  财务部\n主管  '], [2], [3, '李明', '+8613900138000'],
    ])
    expect(rows).toEqual([
      { rowNumber: 3, input: { name: '张明', phone: '+8613800138000', remark: '财务部\n主管' }, errors: [] },
      { rowNumber: 5, input: { name: '李明', phone: '+8613900138000', remark: '' }, errors: [] },
    ])
  })

  it('supports reordered English columns and optional remarks', () => {
    expect(parseDesktopMemberImportRows([['Phone', 'Name'], ["'+8613800138000", 'Alice']])[0]).toEqual({
      rowNumber: 2, input: { name: 'Alice', phone: '+8613800138000', remark: '' }, errors: [],
    })
  })

  it('detects duplicate phone numbers after normalization and validates required fields and lengths', () => {
    const rows = parseDesktopMemberImportRows([
      ['姓名', '手机号', '备注'], ['One', '13800138000'], ['Two', '+86 138-0013-8000'],
      ['', 'invalid', '字'.repeat(501)], ['字'.repeat(101), '13900138000'],
    ])
    expect(rows[1].errors).toEqual(['duplicatePhone'])
    expect(rows[2].errors).toEqual(['invalidName', 'invalidPhone', 'invalidRemark'])
    expect(rows[3].errors).toEqual(['invalidName'])
  })

  it('rejects missing or ambiguous headers, blank templates, and imports over 200 members', () => {
    expect(() => parseDesktopMemberImportRows([['Name'], ['Alice']])).toThrow('missingHeaders')
    expect(() => parseDesktopMemberImportRows([['姓名', 'name', '电话'], ['One', 'Two', '13800138000']])).toThrow('missingHeaders')
    expect(() => parseDesktopMemberImportRows([['姓名', '电话'], [null, null]])).toThrow('noRows')
    expect(() => parseDesktopMemberImportRows([['姓名', '电话'], ...Array.from({ length: 201 }, () => ['One', '13800138000'])])).toThrow('tooManyRows')
  })

  it('reads quoted CSV remarks and line breaks without interpreting formulas', async () => {
    const file = new File(['\uFEFF姓名,联系电话,备注\r\nAlice,13800138000,"=1+1,quoted\ntext"'], 'members.csv')
    expect((await parseDesktopMemberImportFile(file))[0].input).toEqual({ name: 'Alice', phone: '+8613800138000', remark: '=1+1,quoted\ntext' })
    await expect(parseDesktopMemberImportFile(new File(['姓名,电话\nAlice,"13800138000'], 'bad.csv'))).rejects.toThrow('invalidFile')
  })

  it('rejects oversized files, unsupported formats, and invalid Excel data', async () => {
    await expect(parseDesktopMemberImportFile({ name: 'large.xlsx', size: 5 * 1024 * 1024 + 1 } as File)).rejects.toThrow('fileTooLarge')
    await expect(parseDesktopMemberImportFile(new File(['x'], 'members.xls'))).rejects.toThrow('unsupportedFile')
    await expect(parseDesktopMemberImportFile(new File(['not excel'], 'members.xlsx'))).rejects.toThrow('invalidFile')
  })

  it('generates a downloadable Excel template and reads a filled template using the import parser', async () => {
    const template = await createDesktopMemberImportTemplate()
    expect(template.type).toBe('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
    const rows = await readSheet(await buffer(template), 1)
    expect(rows[0][0]).toBe('公司内部通讯录')
    expect(rows[1]).toEqual(['序号', '姓名', '联系电话', '备注'])
    expect(() => parseDesktopMemberImportRows(rows)).toThrow('noRows')
    rows[2] = [1, '王明', '13800138000', '产品部']
    const workbook = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet(rows), '成员')
    XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([['ignore me']]), '其他')
    const file = new File([XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })], 'members.xlsx')
    expect((await parseDesktopMemberImportFile(file))[0]).toEqual({ rowNumber: 3, input: { name: '王明', phone: '+8613800138000', remark: '产品部' }, errors: [] })
  })
})

import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import DesktopPeriodReports from '../DesktopPeriodReports.vue'

vi.mock('vue-i18n', async original => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({
    t: (key: string) => {
      let value: unknown = zh
      for (const part of key.split('.')) value = value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined
      return typeof value === 'string' ? value : key
    },
    te: (key: string) => {
      let value: unknown = zh
      for (const part of key.split('.')) value = value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined
      return typeof value === 'string'
    },
  }),
}))

const client = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), sources: vi.fn(), source: vi.fn(), run: vi.fn() }))
vi.mock('@/api/desktopPeriods', async original => ({ ...await original<typeof import('@/api/desktopPeriods')>(), desktopPeriods: () => client }))
const dates = Array.from({ length: 29 }, (_, index) => `2026-09-${String(index + 1).padStart(2, '0')}`)
const report = { id: 'period_one', kind: 'month', range: { start_date: '2026-09-01', end_date: '2026-09-30', timezone: 'Asia/Shanghai' }, revision: 1, status: 'partial', reason: '', generated_at: '2026-10-01T02:00:00Z', model: 'snapshot-model', source_changed: false, coverage: { available_dates: ['2026-09-30'], partial_dates: [], missing_dates: dates, no_record_dates: [], reasons: Object.fromEntries(dates.map(date => [date, 'not_generated'])) } }
function render() {
  return mount(DesktopPeriodReports, { props: { organizationId: 'org', enabled: true, analysisModel: 'current-model', initialKind: 'month' }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
}
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-14T04:00:00Z')); vi.clearAllMocks()
  client.list.mockResolvedValue({ items: [report], has_more: false })
  client.detail.mockResolvedValue({ report, content: '# 总结正文\n\n本月进展。\n\n[日总结:2026-09-30]', summary_outline: { validated_source_dates: ['2026-09-30'] } })
  client.sources.mockResolvedValue({ items: [], has_more: false })
  client.source.mockResolvedValue({ date: '2026-09-30', revision: 1, status: 'completed', content: '冻结的日总结正文' })
})
enableAutoUnmount(afterEach)
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('Period report reading order', () => {
  it('renders Markdown before metadata, date coverage and history', async () => {
    const wrapper = render(); await flushPromises()
    const body = wrapper.get('article'), metadata = wrapper.get('[data-testid="period-metadata"]'), coverage = wrapper.get('[data-testid="period-coverage"]')
    expect(body.get('h1').text()).toBe('总结正文')
    for (const after of [metadata, coverage, wrapper.get('[data-testid="period-history"]')]) {
      expect(body.element.compareDocumentPosition(after.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    }
    expect(metadata.attributes('open')).toBeUndefined()
    expect(coverage.attributes('open')).toBeUndefined()
    expect(wrapper.text()).not.toContain('not_generated')
    expect(wrapper.get('[data-testid="period-history"]').text()).toContain('部分完成')
    expect(coverage.findAll('h4')).toHaveLength(1)
    expect(coverage.get('h4').text()).toContain('(29)')
    expect(coverage.text()).toContain('2026-09-01')
    expect(coverage.text()).toContain('2026-09-29')
    wrapper.unmount()
  })

  it('continues to read a frozen source from a validated Markdown reference', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('a[href="#frozen-source-2026-09-30"]').trigger('click'); await flushPromises()
    expect(client.source).toHaveBeenCalledWith('period_one', '2026-09-30')
    expect(wrapper.get('.report-source-detail').text()).toContain('冻结的日总结正文')
    wrapper.unmount()
  })

  it('refreshes the selected text after a queued report completes', async () => {
    const queued = { ...report, status: 'pending' }
    client.list.mockResolvedValueOnce({ items: [queued], has_more: false })
    client.detail.mockResolvedValueOnce({ report: queued, content: null })
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('article').exists()).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === '刷新')!.trigger('click'); await flushPromises()
    expect(wrapper.get('article').text()).toContain('总结正文')
    wrapper.unmount()
  })

  it('keeps the latest readable report visible while a newer task is pending', async () => {
    const queued = { ...report, id: 'period_new', revision: 2, status: 'pending' }
    client.list.mockResolvedValue({ items: [queued, report], has_more: false })
    client.detail.mockImplementation(async (id: string) => id === queued.id ? { report: queued, content: null } : { report, content: '上一份可读总结' })
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('article').text()).toBe('上一份可读总结')
    expect(wrapper.findAll('button').find(button => button.text() === '重生成新版本')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('polls queued tasks while visible, then displays the completed text', async () => {
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    const queued = { ...report, status: 'pending' }
    client.list.mockResolvedValueOnce({ items: [queued], has_more: false })
    client.detail.mockResolvedValueOnce({ report: queued, content: null })
    const wrapper = render(); await flushPromises()
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.get('article').text()).toContain('总结正文')
    expect(client.list).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(5000)
    expect(client.list).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('retains a chosen historical version when refreshing the list', async () => {
    const newer = { ...report, id: 'period_two', revision: 2, status: 'completed' }
    client.list.mockResolvedValue({ items: [newer, report], has_more: false })
    client.detail.mockImplementation(async (id: string) => ({ report: id === newer.id ? newer : report, content: id === newer.id ? '新版正文' : '旧版正文' }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('[data-testid="period-history"] button')[1].trigger('click'); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '刷新')!.trigger('click'); await flushPromises()
    expect(wrapper.get('article').text()).toBe('旧版正文')
    expect(wrapper.text()).toContain('正在查看历史版本')
    await wrapper.findAll('button').find(button => button.text() === '查看最新可读总结')!.trigger('click'); await flushPromises()
    expect(wrapper.get('article').text()).toBe('新版正文')
    wrapper.unmount()
  })

  it('clears protected cached text when permission is revoked on refresh', async () => {
    const wrapper = render(); await flushPromises()
    client.detail.mockRejectedValueOnce({ status: 403 })
    await wrapper.findAll('button').find(button => button.text() === '刷新')!.trigger('click'); await flushPromises()
    expect(wrapper.find('article').exists()).toBe(false)
    wrapper.unmount()
  })

  it('loads the source index independently from an opened frozen source', async () => {
    let finish!: (value: unknown) => void
    client.sources.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = render(); await flushPromises()
    await wrapper.get('a[href="#frozen-source-2026-09-30"]').trigger('click'); await flushPromises()
    finish({ items: [{ date: '2026-09-30', revision: 1, status: 'completed' }], has_more: false }); await flushPromises()
    expect(wrapper.findAll('.report-source-list button')).toHaveLength(1)
    expect(wrapper.get('.report-source-detail').text()).toContain('冻结的日总结正文')
    wrapper.unmount()
  })

  it('offers regeneration rather than a no-op retry once text is available', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('button').find(button => button.text() === '生成')!.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button').find(button => button.text() === '重试')!.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button').find(button => button.text() === '重生成新版本')!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('renders historical content when empty coverage arrays are null', async () => {
    client.detail.mockResolvedValue({ report: { ...report, coverage: { available_dates: null, missing_dates: null, reasons: null } }, content: '历史正文' })
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('article').text()).toBe('历史正文')
    expect(wrapper.get('[data-testid="period-coverage"]').text()).toContain('可用日期 0')
    wrapper.unmount()
  })
})

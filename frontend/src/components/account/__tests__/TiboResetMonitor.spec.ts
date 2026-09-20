import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TiboResetMonitor from '../TiboResetMonitor.vue'
import { i18n } from '@/i18n'
import type { TiboResetMonitorEvent } from '@/types'

const { getTiboResetMonitor } = vi.hoisted(() => ({ getTiboResetMonitor: vi.fn() }))
const previousLocale = i18n.global.locale.value
afterAll(() => { i18n.global.locale.value = previousLocale })

vi.mock('@/api/subscriptionAccounts', () => ({ default: { getTiboResetMonitor } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function event(overrides: Partial<TiboResetMonitorEvent> = {}): TiboResetMonitorEvent {
  return {
    id: 'credit-sep-4', type: 'reset_credit', label: '发重置卡', status: 'confirmed',
    title: '重置卡已发放', scope: '', createdAt: '2026-09-04T07:12:09+08:00',
    updatedAt: '2026-09-13T10:36:02+08:00', confirmedAt: null, occurredOn: '2026-09-04',
    confirmationBasis: 'receipt_review', schedule: null, posts: [], url: 'https://aihot.news/codex-reset',
    ...overrides,
  }
}

async function render(events: TiboResetMonitorEvent[]) {
  getTiboResetMonitor.mockResolvedValue({
    schemaVersion: 1, timezone: 'Asia/Shanghai', checkedAt: '',
    historyFrom: '2026-06-12T00:00:00+08:00', count: events.length, events,
  })
  const wrapper = mount(TiboResetMonitor)
  await flushPromises()
  return wrapper
}

describe('TiboResetMonitor event dates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    i18n.global.locale.value = 'zh'
  })

  it('shows the September 22 card forecast alongside the confirmed September 12 direct reset', async () => {
    const forecast = event({
      id: 'banked-2101352781219258527-1-1', status: 'announced', title: 'Tibo 预告发放重置卡',
      createdAt: '2026-09-20T00:48:38+08:00', updatedAt: '2026-09-20T00:48:38+08:00', occurredOn: null,
      schedule: {
        precision: 'date', from: '2026-09-22T15:00:00+08:00', through: '2026-09-23T15:00:00+08:00',
        label: '北京时间预计 9月22日 15:00–9月23日 15:00',
      },
    })
    const direct = event({
      id: 'reset-2098612714704891959-1-1', type: 'direct_reset', title: 'Codex 额度重置已完成',
      confirmedAt: '2026-09-12T16:09:17+08:00', occurredOn: null,
    })
    const wrapper = await render([event(), forecast, direct])

    expect(wrapper.text()).toContain(forecast.title)
    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.cardAnnouncement')
    expect(wrapper.text()).toContain('2026/09/22 15:00')
    expect(wrapper.text()).toContain(forecast.schedule!.label)
    expect(wrapper.text()).toContain('2026/09/12 16:09')
    expect(wrapper.text()).not.toContain('重置卡已发放')
    expect(wrapper.text()).not.toContain('2026/09/13 10:36')
  })

  it('orders receipt-confirmed cards by occurrence date rather than a later editorial update', async () => {
    const wrapper = await render([
      event({ title: 'September 4 card', updatedAt: '2026-09-21T10:00:00+08:00' }),
      event({ id: 'credit-sep-5', title: 'September 5 card', occurredOn: '2026-09-05' }),
    ])

    expect(wrapper.text()).toContain('September 5 card')
    expect(wrapper.text()).not.toContain('September 4 card')
    expect(wrapper.text()).toContain('2026/09/05')
    expect(wrapper.text()).not.toContain('00:00')
  })

  it('never labels a reset card or an unconfirmed direct reset as a completed global reset', async () => {
    const wrapper = await render([
      event(),
      event({ id: 'unconfirmed', type: 'direct_reset', status: 'announced', title: 'Not completed' }),
    ])

    expect(wrapper.text()).not.toContain('subscriptionAccounts.tiboReset.directReset')
    expect(wrapper.text()).not.toContain('Not completed')
    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.noData')
    expect(wrapper.text()).toContain('重置卡已发放')
  })

  it('keeps the latest confirmed reset and links its confirmation post instead of a newer announcement', async () => {
    const wrapper = await render([
      event({ id: 'older', type: 'direct_reset', title: 'Older reset', confirmedAt: '2026-09-08T12:05:53+08:00', updatedAt: '2026-09-23T00:00:00+08:00' }),
      event({
        id: 'latest', type: 'direct_reset', title: 'Latest reset', confirmedAt: '2026-09-12T16:09:17+08:00',
        posts: [
          { id: 'announcement', publishedAt: '2026-09-13T00:00:00+08:00', stage: '预告', text: '', url: 'https://x.com/announcement' },
          { id: 'confirmation', publishedAt: '2026-09-12T16:09:17+08:00', stage: '确认完成', text: '', url: 'https://x.com/confirmation' },
        ],
      }),
    ])

    expect(wrapper.text()).toContain('Latest reset')
    expect(wrapper.text()).not.toContain('Older reset')
    expect(wrapper.get('a').attributes('href')).toBe('https://x.com/confirmation')
  })

  it('does not invent a completion time from createdAt or updatedAt when receipt evidence has no date', async () => {
    const wrapper = await render([event({ occurredOn: null })])

    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.unknown')
    expect(wrapper.text()).not.toContain('2026/09/13 10:36')
    expect(wrapper.text()).not.toContain('2026/09/04 07:12')
  })

  it('keeps an elapsed forecast unconfirmed and does not use its publication time as the schedule', async () => {
    const wrapper = await render([event({
      status: 'announced', title: 'Pending confirmation', occurredOn: null,
      createdAt: '2026-08-01T12:00:00+08:00', schedule: null,
    })])

    expect(wrapper.text()).toContain('Pending confirmation')
    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.cardAnnouncement')
    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.unknown')
    expect(wrapper.text()).not.toContain('subscriptionAccounts.tiboReset.resetCard')
  })

  it('compares date-only records and exact times in the same source timezone', async () => {
    const wrapper = await render([
      event({ title: 'Date-only record', occurredOn: '2026-09-05' }),
      event({ id: 'exact', title: 'Confirmed after midnight', confirmedAt: '2026-09-05T00:15:00+08:00', occurredOn: null }),
    ])

    expect(wrapper.text()).toContain('Confirmed after midnight')
    expect(wrapper.text()).not.toContain('Date-only record')
    expect(wrapper.text()).toContain('2026/09/05 00:15')
  })

  it('prefers confirmation when a forecast has the same event time', async () => {
    const wrapper = await render([
      event({
        status: 'announced', title: 'Forecast', occurredOn: null,
        schedule: { precision: 'window', from: '2026-09-22T15:00:00+08:00', through: '2026-09-22T16:00:00+08:00', label: '' },
      }),
      event({ id: 'confirmed', title: 'Confirmed card', confirmedAt: '2026-09-22T15:00:00+08:00', occurredOn: null }),
    ])

    expect(wrapper.text()).toContain('Confirmed card')
    expect(wrapper.text()).not.toContain('Forecast')
    expect(wrapper.text()).not.toContain('subscriptionAccounts.tiboReset.cardAnnouncement')
  })

  it('does not accept a nonexistent calendar date as an occurrence', async () => {
    const wrapper = await render([event({ confirmedAt: 'invalid', occurredOn: '2026-02-30' })])

    expect(wrapper.text()).toContain('subscriptionAccounts.tiboReset.unknown')
    expect(wrapper.text()).not.toContain('2026/02/30')
  })
})

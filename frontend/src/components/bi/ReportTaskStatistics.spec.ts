import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ReportTaskStatistics from './ReportTaskStatistics.vue'
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string, args?: unknown) => args ? key + JSON.stringify(args) : key }) }))

describe('Report usage statistics', () => {
  it('distinguishes unknown usage from recorded zeros in totals and model breakdowns', () => {
    const wrapper = mount(ReportTaskStatistics, { props: { loading: false, error: '', statistics: {
      task_count: '2', organization_count: '1', execution_count: '3', total_tokens: null, actual_cost: '0.0000000000', missing_token_count: '3', missing_cost_count: '2',
      task_statuses: { completed: '2' }, execution_statuses: { completed: '2', running: '1' },
      models: [{ model: 'model-a', execution_count: '3', total_tokens: null, actual_cost: '0.0000000000', missing_token_count: '3', missing_cost_count: '2' }],
    } } })
    expect(wrapper.findAll('.bi-report-metrics > div > dd:first-of-type').map(value => value.text())).toEqual(['2', '1', '1', '3', '-', '0.0000000000'])
    expect(wrapper.text()).toContain('biNative.missingUsage{"count":"3"}')
    expect(wrapper.get('tbody').text()).toContain('0.0000000000')
    wrapper.unmount()
  })
})

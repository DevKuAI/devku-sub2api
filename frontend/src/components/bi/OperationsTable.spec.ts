import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import OperationsTable from './OperationsTable.vue'
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

describe('BI operations metadata table', () => {
  it('uses hyphens for missing metadata while retaining zero counts, zero fees and disabled flags', () => {
    const wrapper = mount(OperationsTable, { props: {
      rows: [{ id: 'row', name: '   ', analysis_model: '', last_received_at: null, status: '', users: 0, actual_cost: '0.000000', reporting_enabled: false }],
      columns: ['name', 'analysis_model', 'last_received_at', 'status', 'users', 'actual_cost', 'reporting_enabled'], loading: false, error: '',
    } })
    expect(wrapper.findAll('tbody td').map(cell => cell.text())).toEqual(['-', '-', '-', '-', '0', '0.000000', 'biNative.notEnabled'])
    expect(wrapper.find('time').exists()).toBe(false)
    wrapper.unmount()
  })

  it('opens metadata outside table cells without stretching the row or showing business text', async () => {
    const wrapper = mount(OperationsTable, { props: {
      rows: [{ id: 'task', status: 'failed', error: 'Upstream failure', attempt: 0, model: '', content: 'PRIVATE_BODY' }],
      columns: ['status'], details: ['error', 'attempt', 'model'], loading: false, error: '',
    }, global: { stubs: { teleport: true } } })
    await wrapper.get('button[aria-haspopup="dialog"]').trigger('click')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.find('td dl').exists()).toBe(false)
    expect(wrapper.get('[role="dialog"]').text()).toContain('Upstream failure')
    expect(wrapper.findAll('[role="dialog"] dd').map(cell => cell.text())).toEqual(['admin.desktop.reports.statuses.failed', 'Upstream failure', '0', '-'])
    expect(wrapper.text()).not.toContain('PRIVATE_BODY')
    wrapper.unmount()
  })
})

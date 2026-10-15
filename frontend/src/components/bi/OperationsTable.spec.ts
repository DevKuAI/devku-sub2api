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

  it('retains secondary metadata in an expandable detail without showing business text', () => {
    const wrapper = mount(OperationsTable, { props: {
      rows: [{ id: 'task', status: 'failed', error: 'Upstream failure', attempt: 0, model: '', content: 'PRIVATE_BODY' }],
      columns: ['status'], details: ['error', 'attempt', 'model'], loading: false, error: '',
    } })
    expect(wrapper.get('details summary').text()).toBe('biNative.recordDetails')
    expect(wrapper.findAll('details dd').map(cell => cell.text())).toEqual(['Upstream failure', '0', '-'])
    expect(wrapper.text()).not.toContain('PRIVATE_BODY')
    wrapper.unmount()
  })
})

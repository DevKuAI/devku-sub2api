import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import DesktopReportContent from '../DesktopReportContent.vue'

const global = { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] }
describe('Desktop report Markdown', () => {
  it('renders headings, lists, code and keyboard reachable tables', () => {
    const wrapper = mount(DesktopReportContent, { props: { content: '# 总结\n\n## 进展\n\n- 完成任务\n\n```go\nfmt.Println("ok")\n```\n\n| 成员 | 进展 |\n| --- | --- |\n| 测试 | 完成 |' }, global })
    expect(wrapper.get('h1').text()).toBe('总结')
    expect(wrapper.get('h2').text()).toBe('进展')
    expect(wrapper.get('li').text()).toBe('完成任务')
    expect(wrapper.get('pre code').text()).toContain('fmt.Println')
    expect(wrapper.get('.report-content-table').attributes('tabindex')).toBe('0')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    wrapper.unmount()
  })

  it('sanitizes model HTML and preserves plain text line breaks', () => {
    const wrapper = mount(DesktopReportContent, { props: { content: '第一行\n第二行\n\n<script>alert(1)</script><img src="x" onerror="alert(2)"><a href="javascript:alert(3)">危险链接</a>' }, global })
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.get('img').attributes('onerror')).toBeUndefined()
    expect(wrapper.get('a').attributes('href')).toBeUndefined()
    expect(wrapper.find('br').exists()).toBe(true)
    wrapper.unmount()
  })

  it('opens only references validated against frozen source dates', async () => {
    const wrapper = mount(DesktopReportContent, { props: { content: '[日总结:2026-10-10]\n\n[日总结:2026-10-11]\n\n[伪造](#frozen-source-2026-10-12)', sourceDates: ['2026-10-10'] }, global })
    await wrapper.get('a[href="#frozen-source-2026-10-10"]').trigger('click')
    await wrapper.get('a[href="#frozen-source-2026-10-12"]').trigger('click')
    expect(wrapper.emitted('source')).toEqual([['2026-10-10']])
    expect(wrapper.text()).toContain('[日总结:2026-10-11]')
    wrapper.unmount()
  })
})

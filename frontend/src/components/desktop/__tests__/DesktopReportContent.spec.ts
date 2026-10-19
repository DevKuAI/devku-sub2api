import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import DesktopReportContent from '../DesktopReportContent.vue'

const global = { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] }
describe('Desktop report Markdown', () => {
  it.each([
    ['**范围： **1 位成员。', '范围：'],
    ['**证据口径： **以下执行结果来自日报。', '证据口径：'],
    ['- **微服务迁移与本机联调推进： **AI 报告测试通过。', '微服务迁移与本机联调推进：'],
    ['**范围：**1 位成员。', '范围：'],
    ['进展：**已完成。**继续跟进。', '已完成。'],
    ['**Progress: **All checks passed.', 'Progress:'],
  ])('renders generated Chinese bold labels in %s', (content, label) => {
    const wrapper = mount(DesktopReportContent, { props: { content }, global })
    expect(wrapper.findAll('strong').map(node => node.text())).toContain(label)
    expect(wrapper.text()).not.toContain('**')
    wrapper.unmount()
  })

  it('preserves code, escapes and nested inline formatting when accepting generated labels', () => {
    const code = '**范围： **1 位成员。'
    const content = [
      `\`${code}\``, '', '```markdown', code, '```', '',
      '\\*\\*范围： \\*\\*1 位成员。', '',
      '**检查 `**原文**` 与 _重点_： **继续跟进。', '',
      '**成员**：测试；***重要***；**外层 *内层* 内容**。',
    ].join('\n')
    const wrapper = mount(DesktopReportContent, { props: { content }, global })
    expect(wrapper.get('p > code').text()).toBe(code)
    expect(wrapper.get('pre code').text().trim()).toBe(code)
    expect(wrapper.findAll('p')[1].text()).toBe(code)
    expect(wrapper.get('strong code').text()).toBe('**原文**')
    expect(wrapper.get('strong em').text()).toBe('重点')
    expect(wrapper.findAll('strong').map(node => node.text())).toEqual(['检查 **原文** 与 重点：', '成员', '重要', '外层 内层 内容'])
    wrapper.unmount()
  })

  it('renders remaining GFM formatting and sanitizes links inside generated labels', () => {
    const content = [
      '### 三级', '#### 四级', '##### 五级', '###### 六级', '',
      '> **证据口径： **仅统计已验证结果。', '',
      '1. *分析*与~~旧结论~~', '   - **下一步： **复核', '',
      '- [x] 完成', '- [ ] 跟进', '', '---', '',
      '**来源：[文档](https://example.com)： **已核对。', '',
      '**风险：[危险](javascript:alert%281%29)： **<img src="x" onerror="alert(1)">',
    ].join('\n')
    const wrapper = mount(DesktopReportContent, { props: { content }, global })
    for (const level of [3, 4, 5, 6]) expect(wrapper.find(`h${level}`).exists()).toBe(true)
    expect(wrapper.get('blockquote strong').text()).toBe('证据口径：')
    expect(wrapper.get('ol em').text()).toBe('分析')
    expect(wrapper.get('ol del').text()).toBe('旧结论')
    expect(wrapper.get('ol ul strong').text()).toBe('下一步：')
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
    expect(wrapper.find('hr').exists()).toBe(true)
    expect(wrapper.get('strong a[href="https://example.com"]').text()).toBe('文档')
    expect(wrapper.findAll('a').find(link => link.text() === '危险')?.attributes('href')).toBeUndefined()
    expect(wrapper.get('img').attributes('onerror')).toBeUndefined()
    wrapper.unmount()
  })

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

import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useStepUp } from '@/composables/useStepUp'
import BIStepUpDialog from './BIStepUpDialog.vue'
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

describe('BI verification overlay', () => {
  it('makes invitation editing inert during verification and restores focus after cancellation', async () => {
    const controller = useStepUp()
    const fixture = defineComponent({ components: { BaseDialog, BIStepUpDialog }, setup: () => ({ controller }), template: '<BaseDialog :show="true" title="Invitation"><button id="invite-trigger">Create</button><BIStepUpDialog :controller="controller" /></BaseDialog>' })
    const wrapper = mount(fixture, { attachTo: document.body, global: { stubs: { TotpStepUpDialog: { props: ['controller'], template: '<div><input aria-label="Code"/><button @click="controller.onCancel()">Cancel</button></div>' } } } })
    await flushPromises()
    const trigger = document.getElementById('invite-trigger')!
    trigger.focus()
    const prompt = controller.prompt()
    await flushPromises()
    const dialogs = document.querySelectorAll('[role="dialog"]')
    expect(dialogs).toHaveLength(2)
    expect(dialogs[0].hasAttribute('inert')).toBe(true)
    expect(dialogs[1].hasAttribute('inert')).toBe(false)
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Code')
    controller.onCancel()
    expect(await prompt).toBe(false)
    await flushPromises()
    expect(document.activeElement).toBe(trigger)
    expect(dialogs[0].hasAttribute('inert')).toBe(false)
    wrapper.unmount()
  })
})

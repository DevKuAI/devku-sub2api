<template>
  <section class="card p-6" aria-labelledby="bi-privacy-heading">
    <h2 id="bi-privacy-heading" class="text-lg font-semibold">{{ t('bi.privacy.heading') }}</h2>
    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('bi.privacy.description') }}</p>
    <p v-if="loading" class="mt-4" role="status">{{ t('common.loading') }}</p>
    <p v-if="error" class="mt-4 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <button v-if="loadFailed" type="button" class="btn btn-secondary mt-4" @click="load">{{ t('common.refresh') }}</button>
    <div v-if="!loading && !loadFailed" class="mt-5 space-y-4" @keydown.enter="onEnter" @input="saved = false">
      <p v-if="!noticeURL" class="text-sm text-amber-700 dark:text-amber-400">{{ t('bi.privacy.urlHint') }}</p>
      <div>
        <label for="bi-privacy-site-url" class="mb-1 block text-sm font-medium">{{ t('bi.privacy.siteURL') }}</label>
        <input id="bi-privacy-site-url" v-model="form.site_url" type="text" inputmode="url" class="input" maxlength="2048" placeholder="https://bi.example.com" :disabled="saving" aria-describedby="bi-privacy-site-url-hint" />
        <p id="bi-privacy-site-url-hint" class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('bi.privacy.siteURLHint') }}</p>
      </div>
      <div>
        <label for="bi-privacy-title" class="mb-1 block text-sm font-medium">{{ t('bi.privacy.title') }}</label>
        <input id="bi-privacy-title" v-model="form.title" class="input" maxlength="80" :disabled="saving" />
      </div>
      <div>
        <label for="bi-privacy-version" class="mb-1 block text-sm font-medium">{{ t('bi.privacy.version') }}</label>
        <input id="bi-privacy-version" v-model="form.version" class="input" maxlength="64" :disabled="saving" aria-describedby="bi-privacy-version-hint" />
        <p id="bi-privacy-version-hint" class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('bi.privacy.versionHint') }}</p>
      </div>
      <div>
        <label for="bi-privacy-content" class="mb-1 block text-sm font-medium">{{ t('bi.privacy.content') }}</label>
        <textarea id="bi-privacy-content" v-model="form.content_md" rows="12" class="input font-mono text-sm" :disabled="saving" aria-describedby="bi-privacy-content-hint"></textarea>
        <p id="bi-privacy-content-hint" class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('bi.privacy.contentHint') }}</p>
      </div>
      <p v-if="noticeURL" class="break-all text-sm text-gray-500 dark:text-dark-400">{{ noticeURL }}</p>
      <div class="flex flex-wrap items-center gap-3">
        <button type="button" class="btn btn-primary" :disabled="saving || !canSave" @click="save">
          {{ saving ? t('common.saving') : t('bi.privacy.publish') }}
        </button>
        <a v-if="published && noticeURL" :href="noticeURL" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">{{ t('bi.privacy.view') }}</a>
      </div>
      <p v-if="saved" class="text-sm text-green-700 dark:text-green-400" role="status">{{ t('bi.privacy.saved') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getBIPrivacyNotice, saveBIPrivacyNotice } from '@/api/biPrivacy'

const { t } = useI18n()
const form = reactive({ site_url: '', title: '', version: '', content_md: '' })
const noticeURL = ref('')
const published = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const saved = ref(false)
const error = ref('')
const canSave = computed(() => form.site_url.trim() && form.title.trim() && form.version.trim() && form.content_md.trim())

// This card has its own save action inside the existing settings form.
function onEnter(event: KeyboardEvent) {
  if (event.target instanceof HTMLInputElement) event.preventDefault()
}

async function load() {
  loading.value = true
  loadFailed.value = false
  error.value = ''
  try {
    const notice = await getBIPrivacyNotice(true)
    Object.assign(form, { site_url: notice.site_url, title: notice.title, version: notice.version, content_md: notice.content_md })
    noticeURL.value = notice.url
    published.value = Boolean(notice.content_md && notice.version)
  } catch {
    loadFailed.value = true
    error.value = t('bi.privacy.loadFailed')
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!canSave.value || saving.value || loading.value || loadFailed.value) return
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    const notice = await saveBIPrivacyNotice({ ...form })
    Object.assign(form, { site_url: notice.site_url, title: notice.title, version: notice.version, content_md: notice.content_md })
    noticeURL.value = notice.url
    published.value = true
    saved.value = true
  } catch (cause) {
    const reason = (cause as { reason?: string }).reason
    error.value = reason === 'BI_PRIVACY_VERSION_REQUIRED' ? t('bi.privacy.versionRequired')
      : reason === 'INVALID_BI_PRIVACY_SITE_URL' ? t('bi.privacy.siteURLInvalid') : t('bi.privacy.saveFailed')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getAuthMethods } from '@/api/auth'

const DEFAULT_LOGO_LIGHT = '/images/logo+title-circle.png'
const DEFAULT_LOGO_DARK = '/images/logo+title-circle-darkmode.png'

/**
 * Shared store for site-wide branding. Loaded once on layout mount
 * so the sidebar / header can display the configurable site title.
 * Uses the public /auth/methods endpoint (no auth required) instead
 * of the admin-only settings API.
 */
export const useSiteStore = defineStore('site', () => {
  const title = ref('Passwall')
  const logoUrl = ref('')
  const logoUrlDark = ref('')
  const loaded = ref(false)

  const logoLight = computed(() => logoUrl.value || DEFAULT_LOGO_LIGHT)
  const logoDark = computed(() => logoUrlDark.value || logoUrl.value || DEFAULT_LOGO_DARK)

  async function load() {
    if (loaded.value) return
    try {
      const m = await getAuthMethods()
      title.value = m.site_title || 'Passwall'
      logoUrl.value = m.logo_url || ''
      logoUrlDark.value = m.logo_url_dark || ''
    } catch {
      // keep defaults
    }
    loaded.value = true
  }

  /** Called after admin saves settings so the UI updates immediately. */
  function update(t: string, logo: string, logoDk: string) {
    title.value = t || 'Passwall'
    logoUrl.value = logo || ''
    logoUrlDark.value = logoDk || ''
  }

  return { title, logoUrl, logoUrlDark, logoLight, logoDark, loaded, load, update }
})

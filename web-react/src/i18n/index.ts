import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import zhCommon from '@/locales/zh-CN/common.json'
import zhAppearance from '@/locales/zh-CN/appearance.json'
import zhLanguage from '@/locales/zh-CN/language.json'
import zhAuth from '@/locales/zh-CN/auth.json'
import zhNav from '@/locales/zh-CN/nav.json'
import zhAdmin from '@/locales/zh-CN/admin.json'
import zhUser from '@/locales/zh-CN/user.json'
import enCommon from '@/locales/en-US/common.json'
import enAppearance from '@/locales/en-US/appearance.json'
import enLanguage from '@/locales/en-US/language.json'
import enAuth from '@/locales/en-US/auth.json'
import enNav from '@/locales/en-US/nav.json'
import enAdmin from '@/locales/en-US/admin.json'
import enUser from '@/locales/en-US/user.json'

import type { AppLanguage } from '@/theme'

export const SUPPORTED_LANGUAGES: AppLanguage[] = ['zh-CN', 'en-US']

// Flatten { a: { b: { c: 'x' } } } → { 'a.b.c': 'x' }.
// We do this at init time + run i18n with keySeparator:false so a call like
// t('admin:servers.title') treats 'servers.title' as one flat key. Workaround
// for an i18next quirk where instance-level keySeparator doesn't actually get
// used during nested-key resolution.
type Nested = { [k: string]: string | Nested }
function flatten(obj: Nested, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') out[key] = v
    else if (v && typeof v === 'object') Object.assign(out, flatten(v, key))
  }
  return out
}

const resources = {
  'zh-CN': {
    common: flatten(zhCommon as Nested),
    appearance: flatten(zhAppearance as Nested),
    language: flatten(zhLanguage as Nested),
    auth: flatten(zhAuth as Nested),
    nav: flatten(zhNav as Nested),
    admin: flatten(zhAdmin as Nested),
    user: flatten(zhUser as Nested),
  },
  'en-US': {
    common: flatten(enCommon as Nested),
    appearance: flatten(enAppearance as Nested),
    language: flatten(enLanguage as Nested),
    auth: flatten(enAuth as Nested),
    nav: flatten(enNav as Nested),
    admin: flatten(enAdmin as Nested),
    user: flatten(enUser as Nested),
  },
} as const

export const i18nReady = i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    // Map generic browser language tags (en/zh) onto the exact bundles we ship.
    // Keep zh-CN as the final fallback so missing translations never surface
    // raw keys in normal use.
    fallbackLng: {
      en: ['en-US', 'zh-CN'],
      zh: ['zh-CN'],
      default: ['zh-CN'],
    },
    supportedLngs: SUPPORTED_LANGUAGES,
    load: 'currentOnly',
    preload: SUPPORTED_LANGUAGES,
    ns: ['common', 'appearance', 'language', 'auth', 'nav', 'admin', 'user'],
    defaultNS: 'common',
    fallbackNS: 'common',
    // Resources are pre-flattened to dotted keys, so the runtime no longer
    // needs to walk a nested object — keySeparator:false makes t() treat the
    // whole 'servers.title' string as one flat lookup.
    keySeparator: false,
    nsSeparator: ':',
    interpolation: { escapeValue: false },
    detection: {
      order: ['querystring', 'localStorage', 'navigator'],
      lookupQuerystring: 'lang',
      lookupLocalStorage: 'psp-lang',
      caches: ['localStorage'],
    },
    react: {
      useSuspense: false,
    },
  })

export function setLanguage(lang: AppLanguage) {
  void i18n.changeLanguage(lang)
}

export function currentLanguage(): AppLanguage {
  const lng = i18n.resolvedLanguage
  return SUPPORTED_LANGUAGES.includes(lng as AppLanguage) ? (lng as AppLanguage) : 'zh-CN'
}

export default i18n

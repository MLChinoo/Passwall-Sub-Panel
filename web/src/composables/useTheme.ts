import { computed, ref, watchEffect } from 'vue'

export type ThemeMode = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'psp-theme-mode'
const LEGACY_STORAGE_KEY = 'psp-theme'

function getSystemDark() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

function readSavedMode(): ThemeMode {
  localStorage.removeItem(LEGACY_STORAGE_KEY)
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return 'system'
}

const themeMode = ref<ThemeMode>(readSavedMode())
const systemDark = ref(getSystemDark())
const initialized = ref(false)
let mediaQuery: MediaQueryList | null = null

const isDark = computed(() => {
  if (themeMode.value === 'dark') return true
  if (themeMode.value === 'light') return false
  return systemDark.value
})

function persistMode(mode: ThemeMode) {
  if (mode === 'system') {
    localStorage.removeItem(STORAGE_KEY)
    return
  }
  localStorage.setItem(STORAGE_KEY, mode)
}

function setMode(mode: ThemeMode) {
  themeMode.value = mode
  persistMode(mode)
}

watchEffect(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
  document.documentElement.classList.toggle('light', !isDark.value)
  document.documentElement.dataset.themeMode = themeMode.value
  document.documentElement.style.colorScheme = isDark.value ? 'dark' : 'light'
})

export function useTheme() {
  const initTheme = () => {
    if (initialized.value) return
    initialized.value = true
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    systemDark.value = mediaQuery.matches
    themeMode.value = readSavedMode()
    mediaQuery.addEventListener('change', (e) => {
      systemDark.value = e.matches
    })
  }

  const toggleTheme = () => {
    const nextDark = !isDark.value
    setMode(nextDark === systemDark.value ? 'system' : nextDark ? 'dark' : 'light')
  }

  return { isDark, systemDark, themeMode, initTheme, toggleTheme, setMode }
}

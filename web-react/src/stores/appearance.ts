import { create } from 'zustand'
import { DEFAULT_PRESET_HEX } from '@/theme'

const USER_COLOR_KEY = 'psp-user-theme-color'
const USER_MODE_KEY = 'psp-user-theme-mode'

interface AppearanceState {
  // System default — populated from /auth/methods on app boot.
  // Falls back to M3 baseline purple until the site store loads.
  systemColor: string
  // User-level override stored in localStorage. null = follow system.
  userColor: string | null
  mode: 'light' | 'dark'

  setSystemColor: (hex: string | undefined) => void
  setUserColor: (hex: string | null) => void
  setMode: (mode: 'light' | 'dark') => void
}

function loadUserColor(): string | null {
  const v = localStorage.getItem(USER_COLOR_KEY)
  return v && /^#[0-9a-fA-F]{6}$/.test(v) ? v.toUpperCase() : null
}

function loadUserMode(): 'light' | 'dark' {
  return localStorage.getItem(USER_MODE_KEY) === 'dark' ? 'dark' : 'light'
}

export const useAppearanceStore = create<AppearanceState>((set) => ({
  systemColor: DEFAULT_PRESET_HEX,
  userColor: loadUserColor(),
  mode: loadUserMode(),

  setSystemColor(hex) {
    if (!hex || !/^#[0-9a-fA-F]{6}$/.test(hex)) return
    set({ systemColor: hex.toUpperCase() })
  },

  setUserColor(hex) {
    if (hex === null) {
      localStorage.removeItem(USER_COLOR_KEY)
      set({ userColor: null })
      return
    }
    if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return
    const next = hex.toUpperCase()
    localStorage.setItem(USER_COLOR_KEY, next)
    set({ userColor: next })
  },

  setMode(mode) {
    localStorage.setItem(USER_MODE_KEY, mode)
    set({ mode })
  },
}))

export const selectEffectiveColor = (s: AppearanceState) => s.userColor ?? s.systemColor

import {
  argbFromHex,
  hexFromArgb,
  themeFromSourceColor,
} from '@material/material-color-utilities'

// M3 token shape consumed by createAppTheme. Mirrors the Material Design 3
// color role spec — see https://m3.material.io/styles/color/the-color-system.
export interface M3Tokens {
  primary: string
  onPrimary: string
  primaryContainer: string
  onPrimaryContainer: string
  secondary: string
  onSecondary: string
  secondaryContainer: string
  onSecondaryContainer: string
  tertiary: string
  onTertiary: string
  tertiaryContainer: string
  onTertiaryContainer: string
  error: string
  onError: string
  errorContainer: string
  onErrorContainer: string
  surface: string
  surfaceDim: string
  surfaceBright: string
  surfaceContainerLowest: string
  surfaceContainerLow: string
  surfaceContainer: string
  surfaceContainerHigh: string
  surfaceContainerHighest: string
  onSurface: string
  onSurfaceVariant: string
  outline: string
  outlineVariant: string
}

// Surface-container tones from the M3 2024 spec. Standard scheme objects
// from material-color-utilities don't yet expose these directly, so we read
// them off the neutral palette ourselves.
const LIGHT_SURFACE_TONES = {
  surface: 98,
  surfaceDim: 87,
  surfaceBright: 98,
  surfaceContainerLowest: 100,
  surfaceContainerLow: 96,
  surfaceContainer: 94,
  surfaceContainerHigh: 92,
  surfaceContainerHighest: 90,
  onSurface: 10,
  onSurfaceVariantTone: 30,
  outlineTone: 50,
  outlineVariantTone: 80,
}

const DARK_SURFACE_TONES = {
  surface: 6,
  surfaceDim: 6,
  surfaceBright: 24,
  surfaceContainerLowest: 4,
  surfaceContainerLow: 10,
  surfaceContainer: 12,
  surfaceContainerHigh: 17,
  surfaceContainerHighest: 22,
  onSurface: 90,
  onSurfaceVariantTone: 80,
  outlineTone: 60,
  outlineVariantTone: 30,
}

export function tokensFromSource(sourceHex: string, mode: 'light' | 'dark'): M3Tokens {
  const theme = themeFromSourceColor(argbFromHex(sourceHex))
  const palettes = theme.palettes
  const t = (palette: typeof palettes.primary, tone: number) => hexFromArgb(palette.tone(tone))
  const tones = mode === 'light' ? LIGHT_SURFACE_TONES : DARK_SURFACE_TONES

  if (mode === 'light') {
    return {
      primary: t(palettes.primary, 40),
      onPrimary: t(palettes.primary, 100),
      primaryContainer: t(palettes.primary, 90),
      onPrimaryContainer: t(palettes.primary, 10),
      secondary: t(palettes.secondary, 40),
      onSecondary: t(palettes.secondary, 100),
      secondaryContainer: t(palettes.secondary, 90),
      onSecondaryContainer: t(palettes.secondary, 10),
      tertiary: t(palettes.tertiary, 40),
      onTertiary: t(palettes.tertiary, 100),
      tertiaryContainer: t(palettes.tertiary, 90),
      onTertiaryContainer: t(palettes.tertiary, 10),
      error: t(palettes.error, 40),
      onError: t(palettes.error, 100),
      errorContainer: t(palettes.error, 90),
      onErrorContainer: t(palettes.error, 10),
      surface: t(palettes.neutral, tones.surface),
      surfaceDim: t(palettes.neutral, tones.surfaceDim),
      surfaceBright: t(palettes.neutral, tones.surfaceBright),
      surfaceContainerLowest: t(palettes.neutral, tones.surfaceContainerLowest),
      surfaceContainerLow: t(palettes.neutral, tones.surfaceContainerLow),
      surfaceContainer: t(palettes.neutral, tones.surfaceContainer),
      surfaceContainerHigh: t(palettes.neutral, tones.surfaceContainerHigh),
      surfaceContainerHighest: t(palettes.neutral, tones.surfaceContainerHighest),
      onSurface: t(palettes.neutral, tones.onSurface),
      onSurfaceVariant: t(palettes.neutralVariant, tones.onSurfaceVariantTone),
      outline: t(palettes.neutralVariant, tones.outlineTone),
      outlineVariant: t(palettes.neutralVariant, tones.outlineVariantTone),
    }
  }
  return {
    primary: t(palettes.primary, 80),
    onPrimary: t(palettes.primary, 20),
    primaryContainer: t(palettes.primary, 30),
    onPrimaryContainer: t(palettes.primary, 90),
    secondary: t(palettes.secondary, 80),
    onSecondary: t(palettes.secondary, 20),
    secondaryContainer: t(palettes.secondary, 30),
    onSecondaryContainer: t(palettes.secondary, 90),
    tertiary: t(palettes.tertiary, 80),
    onTertiary: t(palettes.tertiary, 20),
    tertiaryContainer: t(palettes.tertiary, 30),
    onTertiaryContainer: t(palettes.tertiary, 90),
    error: t(palettes.error, 80),
    onError: t(palettes.error, 20),
    errorContainer: t(palettes.error, 30),
    onErrorContainer: t(palettes.error, 90),
    surface: t(palettes.neutral, tones.surface),
    surfaceDim: t(palettes.neutral, tones.surfaceDim),
    surfaceBright: t(palettes.neutral, tones.surfaceBright),
    surfaceContainerLowest: t(palettes.neutral, tones.surfaceContainerLowest),
    surfaceContainerLow: t(palettes.neutral, tones.surfaceContainerLow),
    surfaceContainer: t(palettes.neutral, tones.surfaceContainer),
    surfaceContainerHigh: t(palettes.neutral, tones.surfaceContainerHigh),
    surfaceContainerHighest: t(palettes.neutral, tones.surfaceContainerHighest),
    onSurface: t(palettes.neutral, tones.onSurface),
    onSurfaceVariant: t(palettes.neutralVariant, tones.onSurfaceVariantTone),
    outline: t(palettes.neutralVariant, tones.outlineTone),
    outlineVariant: t(palettes.neutralVariant, tones.outlineVariantTone),
  }
}

const HEX_RE = /^#[0-9a-fA-F]{6}$/
export function isValidHex(value: string): boolean {
  return HEX_RE.test(value)
}

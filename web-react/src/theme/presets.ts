// M3 baseline palette source colors. Each one feeds material-color-utilities
// to generate a full tonal scheme.
export interface ColorPreset {
  id: string
  labelKey: string
  hex: string
}

export const COLOR_PRESETS: ColorPreset[] = [
  // labelKey is relative to the `appearance` namespace — AppearanceMenu calls
  // useTranslation('appearance'), so a leading "appearance." would make the
  // lookup miss and surface the raw key as the tooltip text.
  { id: 'blue', labelKey: 'preset.blue', hex: '#0061A4' },
  { id: 'purple', labelKey: 'preset.purple', hex: '#6750A4' },
  { id: 'teal', labelKey: 'preset.teal', hex: '#006A6B' },
  { id: 'green', labelKey: 'preset.green', hex: '#386A20' },
  { id: 'orange', labelKey: 'preset.orange', hex: '#825500' },
  { id: 'pink', labelKey: 'preset.pink', hex: '#9D2C5F' },
]

export const DEFAULT_PRESET_HEX = COLOR_PRESETS[0].hex

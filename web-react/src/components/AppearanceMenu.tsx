import { useState, type MouseEvent } from 'react'
import {
  IconButton,
  Popover,
  Box,
  Typography,
  TextField,
  Button,
  Tooltip,
  ToggleButton,
  ToggleButtonGroup,
  alpha,
  useTheme,
} from '@mui/material'
import PaletteIcon from '@mui/icons-material/Palette'
import CheckIcon from '@mui/icons-material/Check'
import LightModeIcon from '@mui/icons-material/LightMode'
import DarkModeIcon from '@mui/icons-material/DarkMode'
import { useTranslation } from 'react-i18next'
import { COLOR_PRESETS, isValidHex } from '@/theme'

export interface AppearanceState {
  systemColor: string
  userColor: string | null
  mode: 'light' | 'dark'
}

interface Props {
  state: AppearanceState
  onChange: (next: Partial<Pick<AppearanceState, 'userColor' | 'mode'>>) => void
}

export default function AppearanceMenu({ state, onChange }: Props) {
  const { t } = useTranslation('appearance')
  const theme = useTheme()
  const md = theme.palette.md
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const [customDraft, setCustomDraft] = useState(state.userColor ?? state.systemColor)

  const effectiveColor = (state.userColor ?? state.systemColor).toUpperCase()
  const isOverridden = state.userColor !== null
  const customValid = isValidHex(customDraft)

  function open(e: MouseEvent<HTMLElement>) {
    setAnchor(e.currentTarget)
    setCustomDraft(state.userColor ?? state.systemColor)
  }

  function pickPreset(hex: string) {
    onChange({ userColor: hex.toUpperCase() })
  }

  function applyCustom() {
    if (customValid) onChange({ userColor: customDraft.toUpperCase() })
  }

  function resetToSystem() {
    onChange({ userColor: null })
    setCustomDraft(state.systemColor)
  }

  return (
    <>
      <Tooltip title={t('title')}>
        <IconButton onClick={open} aria-label={t('title')}>
          <PaletteIcon />
        </IconButton>
      </Tooltip>
      <Popover
        open={!!anchor}
        anchorEl={anchor}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { mt: 1, p: 2.5, width: 320, borderRadius: 3, bgcolor: md.surfaceContainer } } }}
      >
        {/* Header: source-of-truth indicator */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
          <Box
            sx={{
              width: 36, height: 36, borderRadius: '50%',
              bgcolor: effectiveColor,
              border: `2px solid ${md.outlineVariant}`,
              flexShrink: 0,
            }}
          />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant }}>
              {isOverridden ? t('user_override') : t('system_default')}
            </Typography>
            <Typography sx={{ fontSize: 14, fontWeight: 500 }}>
              {effectiveColor}
            </Typography>
          </Box>
          {isOverridden && (
            <Button size="small" variant="text" onClick={resetToSystem}>
              {t('reset')}
            </Button>
          )}
        </Box>

        {/* Mode selector */}
        <Typography sx={{ fontSize: 12, fontWeight: 500, color: md.onSurfaceVariant, mb: 1, textTransform: 'uppercase', letterSpacing: '.5px' }}>
          {t('mode_section')}
        </Typography>
        <ToggleButtonGroup
          value={state.mode}
          exclusive
          fullWidth
          size="small"
          onChange={(_, v) => v && onChange({ mode: v })}
          sx={{
            mb: 2.5,
            gap: 1,
            '& .MuiToggleButton-root': {
              // Cancel MUI's default negative margin / shared-border collapsing
              // — we want two distinct pills, not a fused segmented control.
              borderRadius: '9999px !important',
              border: `1px solid ${md.outlineVariant}`,
              ml: '0 !important',
              py: 0.75,
              '&.Mui-selected, &.Mui-selected:hover': {
                bgcolor: md.secondaryContainer,
                color: md.onSecondaryContainer,
                borderColor: md.secondaryContainer,
              },
            },
          }}
        >
          <ToggleButton value="light"><LightModeIcon fontSize="small" sx={{ mr: 1 }} />{t('mode.light')}</ToggleButton>
          <ToggleButton value="dark"><DarkModeIcon fontSize="small" sx={{ mr: 1 }} />{t('mode.dark')}</ToggleButton>
        </ToggleButtonGroup>

        {/* Presets */}
        <Typography sx={{ fontSize: 12, fontWeight: 500, color: md.onSurfaceVariant, mb: 1, textTransform: 'uppercase', letterSpacing: '.5px' }}>
          {t('preset_section')}
        </Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(6, 1fr)', gap: 1, mb: 2.5 }}>
          {COLOR_PRESETS.map(p => {
            const selected = effectiveColor === p.hex.toUpperCase()
            return (
              <Tooltip key={p.id} title={t(p.labelKey)}>
                <Box
                  onClick={() => pickPreset(p.hex)}
                  sx={{
                    width: 36, height: 36, borderRadius: '50%',
                    bgcolor: p.hex,
                    cursor: 'pointer',
                    display: 'grid', placeItems: 'center',
                    border: selected ? `2px solid ${md.onSurface}` : `2px solid transparent`,
                    boxShadow: selected ? `0 0 0 2px ${md.surfaceContainer}` : undefined,
                    transition: 'transform .15s',
                    '&:hover': { transform: 'scale(1.08)' },
                  }}
                >
                  {selected && <CheckIcon sx={{ color: '#fff', fontSize: 18 }} />}
                </Box>
              </Tooltip>
            )
          })}
        </Box>

        {/* Custom hex */}
        <Typography sx={{ fontSize: 12, fontWeight: 500, color: md.onSurfaceVariant, mb: 1, textTransform: 'uppercase', letterSpacing: '.5px' }}>
          {t('custom_section')}
        </Typography>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
          <Box
            component="input"
            type="color"
            value={customValid ? customDraft : '#6750A4'}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCustomDraft(e.target.value.toUpperCase())}
            sx={{
              width: 40, height: 40, p: 0, border: 'none', borderRadius: 2,
              bgcolor: 'transparent', cursor: 'pointer', flexShrink: 0,
              '&::-webkit-color-swatch-wrapper': { p: 0 },
              '&::-webkit-color-swatch': { border: `1px solid ${md.outlineVariant}`, borderRadius: 8 },
            }}
          />
          <TextField
            size="small"
            value={customDraft}
            onChange={e => setCustomDraft(e.target.value)}
            placeholder="#6750A4"
            error={!customValid}
            sx={{ flex: 1 }}
          />
          <Button
            variant="contained"
            disabled={!customValid || customDraft.toUpperCase() === effectiveColor}
            onClick={applyCustom}
            sx={{ minHeight: 40, height: 40, px: 2, flexShrink: 0 }}
          >
            <CheckIcon fontSize="small" />
          </Button>
        </Box>
        {!customValid && (
          <Typography sx={{ fontSize: 12, color: md.error, mt: 0.5, ml: 6 }}>
            {t('custom_hint')}
          </Typography>
        )}

        {/* Demo helper: simulate "system default" knob */}
        <Box sx={{ mt: 2.5, pt: 2, borderTop: `1px solid ${md.outlineVariant}` }}>
          <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, lineHeight: 1.5 }}>
            {isOverridden
              ? `${t('system_default')}: ${state.systemColor}`
              : `${t('user_override')}: ${alpha(md.onSurface, 0.4) ? '—' : '—'}`}
          </Typography>
        </Box>
      </Popover>
    </>
  )
}

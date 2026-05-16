import { useState, type FormEvent } from 'react'
import {
  Box,
  Button,
  Card,
  CircularProgress,
  TextField,
  Typography,
  useTheme,
} from '@mui/material'
import LoginIcon from '@mui/icons-material/Login'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'

import { useAuthStore, selectIsAdmin } from '@/stores/auth'
import { useSiteStore } from '@/stores/site'
import { useAppearanceStore } from '@/stores/appearance'
import { homeForRole, isAdminPath } from '@/router/home'
import { pushSnack } from '@/components/SnackbarHost'
import LanguageMenu from '@/components/LanguageMenu'
import AppearanceMenu from '@/components/AppearanceMenu'
import BrandLogo from '@/components/BrandLogo'
import { setLanguage, currentLanguage } from '@/i18n'
import type { AppLanguage } from '@/theme'

interface LocationState { returnTo?: string }

export default function LoginLocalView() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation(['auth', 'common'])
  const navigate = useNavigate()
  const location = useLocation()

  const auth = useAuthStore()
  const site = useSiteStore()
  const appearance = useAppearanceStore()

  const [upn, setUpn] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const returnTo =
    (location.state as LocationState | null)?.returnTo
    ?? new URLSearchParams(location.search).get('return_to')
    ?? undefined

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!upn || !password) {
      pushSnack(t('auth:missing_credentials'), 'warning')
      return
    }
    setBusy(true)
    try {
      await auth.login(upn, password)
      const fallback = homeForRole(useAuthStore.getState().role)
      const requested = returnTo ?? fallback
      const isAdmin = selectIsAdmin(useAuthStore.getState())
      const target = isAdminPath(requested) && !isAdmin ? fallback : requested
      navigate(target, { replace: true })
    } catch { /* axios interceptor toasted */ }
    finally { setBusy(false) }
  }

  return (
    <Box sx={{ position: 'fixed', inset: 0, display: 'flex', flexDirection: 'column', bgcolor: md.surface }}>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 1.5, gap: 0.5 }}>
        <LanguageMenu value={currentLanguage()} onChange={(l: AppLanguage) => setLanguage(l)} />
        <AppearanceMenu
          state={{ systemColor: appearance.systemColor, userColor: appearance.userColor, mode: appearance.mode }}
          onChange={(patch) => {
            if ('userColor' in patch) appearance.setUserColor(patch.userColor ?? null)
            if (patch.mode) appearance.setMode(patch.mode)
          }}
        />
      </Box>
      <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', px: 2 }}>
        <Card sx={{
          width: '100%', maxWidth: 400,
          bgcolor: md.surfaceContainerLow,
          boxShadow: '0 1px 2px rgba(0,0,0,.3),0 1px 3px 1px rgba(0,0,0,.15)',
          p: 4,
        }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', mb: 3 }}>
            <BrandLogo height={56} />
            <Typography variant="h5" sx={{ fontWeight: 500, color: md.onSurface, mt: 1.5 }}>
              {site.appTitle || site.siteTitle}
            </Typography>
            <Typography variant="body2" sx={{ mt: 0.5 }}>{t('auth:local_only_subtitle')}</Typography>
          </Box>
          <Box component="form" onSubmit={submit} sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField label={t('auth:username')} value={upn} onChange={e => setUpn(e.target.value)}
              autoComplete="username" autoFocus fullWidth />
            <TextField label={t('auth:password')} type="password" value={password}
              onChange={e => setPassword(e.target.value)} autoComplete="current-password" fullWidth />
            <Button type="submit" variant="contained" fullWidth size="large" disabled={busy}
              startIcon={busy ? <CircularProgress size={16} color="inherit" /> : <LoginIcon />}
              sx={{ mt: 1 }}>
              {t('auth:submit')}
            </Button>
          </Box>
        </Card>
      </Box>
      {site.footerText && (
        <Box sx={{ p: 2, textAlign: 'center', fontSize: 12, color: md.onSurfaceVariant }}>
          {site.footerText}
        </Box>
      )}
    </Box>
  )
}

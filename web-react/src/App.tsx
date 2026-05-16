import { Suspense, useEffect, useMemo, useState } from 'react'
import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider } from '@mui/material/styles'
import { RouterProvider } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Box, CircularProgress } from '@mui/material'

import { createAppTheme, type AppLanguage } from '@/theme'
import { SUPPORTED_LANGUAGES, currentLanguage } from '@/i18n'
import { useAppearanceStore, selectEffectiveColor } from '@/stores/appearance'
import SnackbarHost from '@/components/SnackbarHost'
import ConfirmHost from '@/components/ConfirmHost'
import { router } from '@/router'

export default function App() {
  const mode = useAppearanceStore(s => s.mode)
  const sourceColor = useAppearanceStore(selectEffectiveColor)
  const { i18n } = useTranslation()
  const [language, setLanguageState] = useState<AppLanguage>(currentLanguage)

  useEffect(() => {
    const handler = (lng: string) => {
      const next = (SUPPORTED_LANGUAGES.includes(lng as AppLanguage) ? lng : 'zh-CN') as AppLanguage
      setLanguageState(next)
      document.documentElement.lang = next
    }
    handler(i18n.resolvedLanguage ?? 'zh-CN')
    i18n.on('languageChanged', handler)
    return () => { i18n.off('languageChanged', handler) }
  }, [i18n])

  const theme = useMemo(
    () => createAppTheme({ mode, sourceColor, language }),
    [mode, sourceColor, language],
  )

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Suspense fallback={<RouteFallback />}>
        <RouterProvider router={router} />
      </Suspense>
      <SnackbarHost />
      <ConfirmHost />
    </ThemeProvider>
  )
}

function RouteFallback() {
  return (
    <Box sx={{ position: 'fixed', inset: 0, display: 'grid', placeItems: 'center' }}>
      <CircularProgress />
    </Box>
  )
}

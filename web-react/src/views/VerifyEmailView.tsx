import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Box, Button, Card, CircularProgress, Link as MuiLink, TextField, Typography, useTheme } from '@mui/material'
import { Link as RouterLink, useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import type { AxiosError } from 'axios'

import { verifyEmail } from '@/api/auth'
import { useSiteStore } from '@/stores/site'
import BrandLogo from '@/components/BrandLogo'

export default function VerifyEmailView() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation(['auth'])
  const navigate = useNavigate()
  const site = useSiteStore()
  const [params] = useSearchParams()

  const token = params.get('token') ?? ''
  const otpMode = token === ''

  const [ident, setIdent] = useState('')
  const [code, setCode] = useState('')
  const [state, setState] = useState<'idle' | 'verifying' | 'done' | 'error'>('idle')
  const [error, setError] = useState('')
  const autoTried = useRef(false)

  useEffect(() => { void site.load() }, [site])

  async function run(input: { token?: string; ident?: string; code?: string }) {
    setState('verifying')
    setError('')
    try {
      await verifyEmail(input)
      setState('done')
    } catch (err) {
      const status = (err as AxiosError).response?.status
      setError(status === 401 ? t('auth:verify_email_invalid_token') : t('auth:verify_email_error'))
      setState('error')
    }
  }

  // Link mode: verify automatically on load.
  useEffect(() => {
    if (!otpMode && !autoTried.current) {
      autoTried.current = true
      void run({ token })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function submitOtp(e: FormEvent) {
    e.preventDefault()
    if (!ident.trim() || !code.trim()) return
    void run({ ident: ident.trim(), code: code.trim() })
  }

  return (
    <Box sx={{ position: 'fixed', inset: 0, display: 'grid', placeItems: 'center', bgcolor: md.surface, px: 2 }}>
      <Card sx={{ width: '100%', maxWidth: 400, bgcolor: md.surfaceContainerLow, p: 4 }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', mb: 3 }}>
          <BrandLogo height={48} />
          <Typography variant="h5" sx={{ fontWeight: 500, mt: 1.5 }}>{t('auth:verify_email_title')}</Typography>
          <Typography variant="body2" sx={{ mt: 0.5, color: md.onSurfaceVariant }}>{t('auth:verify_email_subtitle')}</Typography>
        </Box>

        {state === 'done' ? (
          <>
            <Alert severity="success" sx={{ mb: 2 }}>{t('auth:verify_email_success')}</Alert>
            <Button variant="contained" fullWidth size="large" onClick={() => navigate('/login')}>
              {t('auth:logged_out_sign_in_again')}
            </Button>
          </>
        ) : !otpMode ? (
          <Box sx={{ display: 'grid', placeItems: 'center', py: 2, gap: 2 }}>
            {state === 'verifying' && <CircularProgress size={28} />}
            {state === 'error' && (
              <>
                <Alert severity="error" sx={{ width: '100%' }}>{error}</Alert>
                <Button variant="outlined" onClick={() => navigate('/login')}>{t('auth:back_to_login')}</Button>
              </>
            )}
          </Box>
        ) : (
          <Box component="form" onSubmit={submitOtp} sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField label={t('auth:register_email_label')} type="email" value={ident}
              onChange={e => setIdent(e.target.value)} autoComplete="email" autoFocus fullWidth />
            <TextField label={t('auth:verify_email_code_label')} value={code}
              onChange={e => setCode(e.target.value)} inputProps={{ inputMode: 'numeric', maxLength: 8 }} fullWidth />
            {error && <Alert severity="error" sx={{ py: 0 }}>{error}</Alert>}
            <Button type="submit" variant="contained" fullWidth size="large" disabled={state === 'verifying'}
              startIcon={state === 'verifying' ? <CircularProgress size={16} color="inherit" /> : undefined}>
              {t('auth:verify_email_submit')}
            </Button>
          </Box>
        )}

        {state !== 'done' && (
          <Box sx={{ mt: 2.5, textAlign: 'center' }}>
            <MuiLink component={RouterLink} to="/login" variant="body2"
              onClick={(e) => { e.preventDefault(); navigate('/login') }}>
              {t('auth:back_to_login')}
            </MuiLink>
          </Box>
        )}
      </Card>
    </Box>
  )
}

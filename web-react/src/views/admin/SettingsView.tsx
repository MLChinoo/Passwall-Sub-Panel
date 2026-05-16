import { useEffect, useState, type FormEvent } from 'react'
import {
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Divider,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography,
  useTheme,
} from '@mui/material'
import SaveIcon from '@mui/icons-material/Save'
import VisibilityIcon from '@mui/icons-material/Visibility'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOff'
import SendIcon from '@mui/icons-material/Send'
import { useTranslation } from 'react-i18next'

import {
  getMailSettings,
  getOIDC,
  getSAML,
  getUISettings,
  previewMailTemplate,
  putMailSettings,
  putMailTemplate,
  resetMailTemplate,
  putOIDC,
  putSAML,
  putUISettings,
  sendTestMail,
  type MailReminderKind,
  type MailSettings,
  type MailTemplate,
  type OIDCConfig,
  type QuickLink,
  type SAMLConfig,
  type SubClientRule,
  type SubImportClient,
  type UISettings,
} from '@/api/settings'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/DeleteOutline'
import type { LoginMode } from '@/api/types'
import { pushSnack } from '@/components/SnackbarHost'
import { confirm } from '@/components/ConfirmHost'
import { useSiteStore } from '@/stores/site'
import { useTabParam } from '@/hooks/useTabParam'

type TabKey = 'general' | 'brand' | 'subscription' | 'portal' | 'mail' | 'sso'

export default function SettingsView() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')
  const site = useSiteStore()

  const [tab, setTab] = useTabParam<TabKey>('tab', 'general',
    ['general', 'brand', 'subscription', 'portal', 'mail', 'sso'])
  const [settings, setSettings] = useState<UISettings | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => { void load() }, [])

  async function load() {
    setLoading(true)
    try {
      const loaded = await getUISettings()
      if (!loaded.sub_base_url) {
        loaded.sub_base_url = window.location.origin
      }
      setSettings(loaded)
    }
    finally { setLoading(false) }
  }

  async function save(e?: FormEvent) {
    e?.preventDefault()
    if (!settings) return
    setSaving(true)
    try {
      const saved = await putUISettings(settings)
      setSettings(saved)
      // Mirror brand-relevant fields into the live site store so the layout/header
      // updates immediately without a page reload.
      site.update({
        siteTitle: saved.site_title || 'Passwall',
        appTitle: saved.app_title || 'Passwall',
        logoUrl: saved.logo_url || '',
        logoUrlDark: saved.logo_url_dark || '',
        iconUrl: saved.icon_url || '',
        footerText: saved.footer_text || '© Passwall Sub Panel',
        themeColor: saved.theme_color || undefined,
      })
      pushSnack(t('settings.saved'), 'success')
    } finally { setSaving(false) }
  }

  function patch<K extends keyof UISettings>(key: K, value: UISettings[K]) {
    setSettings(prev => prev ? { ...prev, [key]: value } : prev)
  }

  if (loading || !settings) {
    return <Box sx={{ p: 3, display: 'grid', placeItems: 'center', minHeight: 400 }}><CircularProgress /></Box>
  }

  const tabs: { key: TabKey; labelKey: string }[] = [
    { key: 'general', labelKey: 'settings.tab_general' },
    { key: 'brand', labelKey: 'settings.tab_brand' },
    { key: 'subscription', labelKey: 'settings.tab_subscription' },
    { key: 'portal', labelKey: 'settings.tab_portal' },
    { key: 'mail', labelKey: 'settings.tab_mail' },
    { key: 'sso', labelKey: 'settings.tab_sso' },
  ]

  const saveBar = (
    <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
      <Button variant="contained" type="submit" disabled={saving}
        startIcon={saving ? <CircularProgress size={14} color="inherit" /> : <SaveIcon />}>
        {t('settings.save')}
      </Button>
    </Box>
  )

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" sx={{ mb: 1 }}>{t('settings.title')}</Typography>

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mt: 2, mb: 3, borderBottom: `1px solid ${md.outlineVariant}` }}>
        {tabs.map(tb => <Tab key={tb.key} value={tb.key} label={t(tb.labelKey)} />)}
      </Tabs>

      {tab === 'general' && (
        <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {saveBar}
          <Section title={t('settings.general.section_login')} md={md}>
            <TextField select fullWidth size="small" label={t('settings.general.login_mode')}
              value={settings.login_mode}
              onChange={e => patch('login_mode', e.target.value as LoginMode)}>
              <MenuItem value="dual">{t('settings.general.login_mode_dual')}</MenuItem>
              <MenuItem value="sso_first">{t('settings.general.login_mode_sso_first')}</MenuItem>
              <MenuItem value="sso_redirect">{t('settings.general.login_mode_sso_redirect')}</MenuItem>
              <MenuItem value="local_only">{t('settings.general.login_mode_local_only')}</MenuItem>
            </TextField>
            <FormControlLabel label={t('settings.general.disallow_user_local_login')}
              control={<Switch checked={settings.disallow_user_local_login}
                onChange={(_, c) => patch('disallow_user_local_login', c)} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
            <FormControlLabel label={t('settings.general.disallow_user_password_change')}
              control={<Switch checked={settings.disallow_user_password_change}
                onChange={(_, c) => patch('disallow_user_password_change', c)} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
          </Section>

          <Section title={t('settings.general.section_security')} md={md}>
            <Pair>
              <TextField fullWidth label={t('settings.general.jwt_issuer')}
                value={settings.jwt_issuer} onChange={e => patch('jwt_issuer', e.target.value)} />
            </Pair>
            <Pair>
              <NumField label={t('settings.general.jwt_access_ttl_minutes')} value={settings.jwt_access_ttl_minutes}
                onChange={v => patch('jwt_access_ttl_minutes', v)} />
              <NumField label={t('settings.general.jwt_refresh_ttl_minutes')} value={settings.jwt_refresh_ttl_minutes}
                onChange={v => patch('jwt_refresh_ttl_minutes', v)} />
            </Pair>
            <Pair>
              <NumField label={t('settings.general.sub_per_ip_per_min')} value={settings.sub_per_ip_per_min}
                onChange={v => patch('sub_per_ip_per_min', v)} />
              <NumField label={t('settings.general.login_per_ip_per_min')} value={settings.login_per_ip_per_min}
                onChange={v => patch('login_per_ip_per_min', v)} />
            </Pair>
          </Section>

          <Section title={t('settings.general.section_runtime')} md={md}>
            <Pair>
              <NumField label={t('settings.general.cron_traffic_pull_minutes')} value={settings.cron_traffic_pull_minutes}
                onChange={v => patch('cron_traffic_pull_minutes', v)} />
              <NumField label={t('settings.general.cron_reconcile_minutes')} value={settings.cron_reconcile_minutes}
                onChange={v => patch('cron_reconcile_minutes', v)} />
            </Pair>
            <Pair>
              <NumField label={t('settings.general.audit_retention_days')} value={settings.audit_retention_days}
                onChange={v => patch('audit_retention_days', v)} />
              <NumField label={t('settings.general.sync_task_retention_days')} value={settings.sync_task_retention_days}
                onChange={v => patch('sync_task_retention_days', v)} />
            </Pair>
          </Section>

          <Section title={t('settings.general.emergency_section')} md={md}>
            <FormControlLabel label={t('settings.general.emergency_access_enabled')}
              control={<Switch checked={settings.emergency_access_enabled}
                onChange={(_, c) => patch('emergency_access_enabled', c)} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
            <Pair>
              <NumField label={t('settings.general.emergency_access_hours')} value={settings.emergency_access_hours}
                onChange={v => patch('emergency_access_hours', v)} />
              <NumField label={t('settings.general.emergency_access_max_count')} value={settings.emergency_access_max_count}
                onChange={v => patch('emergency_access_max_count', v)} />
            </Pair>
            <NumField label={t('settings.general.emergency_access_quota_gb')}
              value={settings.emergency_access_quota_gb}
              onChange={v => patch('emergency_access_quota_gb', v)}
              helperText={t('settings.general.emergency_access_quota_gb_hint')} />
          </Section>
        </Box>
      )}

      {tab === 'brand' && (
        <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {saveBar}
          <Section title={t('settings.brand.section_text')} md={md}>
            <Pair>
              <TextField fullWidth label={t('settings.brand.site_title')}
                value={settings.site_title} onChange={e => patch('site_title', e.target.value)} />
              <TextField fullWidth label={t('settings.brand.app_title')}
                value={settings.app_title} onChange={e => patch('app_title', e.target.value)} />
            </Pair>
            <TextField fullWidth label={t('settings.brand.footer_text')}
              value={settings.footer_text} onChange={e => patch('footer_text', e.target.value)} />
            <TextField required fullWidth label={t('settings.brand.sub_base_url')}
              value={settings.sub_base_url} onChange={e => patch('sub_base_url', e.target.value)} />
          </Section>

          <Section title={t('settings.brand.section_assets')} md={md}>
            <TextField fullWidth label={t('settings.brand.icon_url')}
              value={settings.icon_url} onChange={e => patch('icon_url', e.target.value)} />
            <Pair>
              <TextField fullWidth label={t('settings.brand.logo_url')}
                value={settings.logo_url} onChange={e => patch('logo_url', e.target.value)} />
              <TextField fullWidth label={t('settings.brand.logo_url_dark')}
                value={settings.logo_url_dark} onChange={e => patch('logo_url_dark', e.target.value)} />
            </Pair>
            <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant }}>
              {t('settings.brand.asset_hint')}
            </Typography>
          </Section>

          <Section title={t('settings.brand.section_theme')} md={md}>
            <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center' }}>
              <Box
                component="input"
                type="color"
                value={settings.theme_color || '#0061A4'}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => patch('theme_color', e.target.value.toUpperCase())}
                sx={{
                  width: 56, height: 56, p: 0, border: 'none', borderRadius: 2,
                  bgcolor: 'transparent', cursor: 'pointer', flexShrink: 0,
                  '&::-webkit-color-swatch-wrapper': { p: 0 },
                  '&::-webkit-color-swatch': { border: `1px solid ${md.outlineVariant}`, borderRadius: 8 },
                }}
              />
              <TextField fullWidth label={t('settings.brand.theme_color')}
                value={settings.theme_color}
                onChange={e => patch('theme_color', e.target.value)}
                placeholder="#0061A4" />
              {settings.theme_color && (
                <Button size="small" variant="text" onClick={() => patch('theme_color', '')} sx={{ flexShrink: 0 }}>
                  {t('settings.brand.theme_color_clear')}
                </Button>
              )}
            </Box>
            <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant }}>
              {t('settings.brand.theme_color_hint')}
            </Typography>
          </Section>

          <Section title={t('settings.brand.section_email')} md={md}>
            <TextField fullWidth label={t('settings.brand.email_domain')}
              value={settings.email_domain} onChange={e => patch('email_domain', e.target.value)} />
          </Section>
        </Box>
      )}

      {tab === 'subscription' && (
        <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {saveBar}
          <Section title={t('settings.subscription.section_basic')} md={md}>
            <TextField fullWidth label={t('settings.subscription.sub_path')}
              value={'/' + (settings.sub_path || '').replace(/^\/+/, '')}
              onChange={e => {
                // Force a single leading slash; users can't delete it past
                // the first character. Stripping multiples handles paste of
                // "/sub" into the existing displayed slash.
                const stripped = e.target.value.replace(/^\/+/, '')
                patch('sub_path', stripped)
              }} />
            <TextField fullWidth label={t('settings.subscription.sub_import_tutorial_url')}
              value={settings.sub_import_tutorial_url}
              onChange={e => patch('sub_import_tutorial_url', e.target.value)} />
            <Pair>
              <NumField label={t('settings.subscription.sub_log_retention_days')}
                value={settings.sub_log_retention_days}
                onChange={v => patch('sub_log_retention_days', v)} />
              <NumField label={t('settings.subscription.sub_update_interval_hours')}
                value={settings.sub_update_interval_hours}
                onChange={v => patch('sub_update_interval_hours', v)} />
            </Pair>
          </Section>

          <Section title={t('settings.subscription.section_protection')} md={md}>
            <FormControlLabel label={t('settings.subscription.sub_block_auto_disable')}
              control={<Switch checked={settings.sub_block_auto_disable}
                onChange={(_, c) => patch('sub_block_auto_disable', c)} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
            <NumField label={t('settings.subscription.sub_block_auto_disable_count')}
              value={settings.sub_block_auto_disable_count}
              onChange={v => patch('sub_block_auto_disable_count', v)} />
          </Section>

          <ClientRulesEditor
            rules={settings.sub_client_rules}
            onChange={v => patch('sub_client_rules', v)}
            md={md}
          />

          <ImportClientsEditor
            clients={settings.sub_import_clients}
            onChange={v => patch('sub_import_clients', v)}
            md={md}
          />
        </Box>
      )}

      {tab === 'portal' && (
        <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {saveBar}
          <QuickLinksEditor
            links={settings.quick_links}
            onChange={v => patch('quick_links', v)}
            md={md}
          />
          <Section title={t('settings.portal.section_announcement')} md={md}>
            <FormControlLabel label={t('settings.portal.announcement.enabled')}
              control={<Switch checked={settings.global_announcement?.enabled ?? false}
                onChange={(_, c) => patch('global_announcement', { ...settings.global_announcement, enabled: c })} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
            <Pair>
              <TextField fullWidth label={t('settings.portal.announcement.title')}
                value={settings.global_announcement?.title ?? ''}
                onChange={e => patch('global_announcement', { ...settings.global_announcement, title: e.target.value })} />
              <TextField select size="small" fullWidth label={t('settings.portal.announcement.level')}
                value={settings.global_announcement?.level ?? 'info'}
                onChange={e => patch('global_announcement', { ...settings.global_announcement, level: e.target.value as 'info' | 'warning' | 'danger' })}>
                <MenuItem value="info">{t('settings.portal.announcement.level_info')}</MenuItem>
                <MenuItem value="warning">{t('settings.portal.announcement.level_warning')}</MenuItem>
                <MenuItem value="danger">{t('settings.portal.announcement.level_danger')}</MenuItem>
              </TextField>
            </Pair>
            <TextField fullWidth multiline minRows={4} label={t('settings.portal.announcement.content')}
              value={settings.global_announcement?.content ?? ''}
              onChange={e => patch('global_announcement', { ...settings.global_announcement, content: e.target.value })} />
          </Section>
        </Box>
      )}

      {tab === 'mail' && <MailTab />}

      {tab === 'sso' && <SsoTab />}
    </Box>
  )
}

function MailTab() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')

  const [mail, setMail] = useState<MailSettings | null>(null)
  const [templates, setTemplates] = useState<MailTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [changePwd, setChangePwd] = useState(false)
  const [showPwd, setShowPwd] = useState(false)
  const [testTo, setTestTo] = useState('')
  const [testBusy, setTestBusy] = useState(false)
  const [activeTpl, setActiveTpl] = useTabParam<MailReminderKind>('tpl', 'expire_before',
    ['expire_before', 'expired', 'traffic_low', 'traffic_exhausted', 'account_disabled', 'account_enabled', 'announcement'])
  const [tplBusy, setTplBusy] = useState(false)
  const [previewBusy, setPreviewBusy] = useState(false)
  const [preview, setPreview] = useState<{ subject: string; body: string; kind: MailReminderKind } | null>(null)

  useEffect(() => { void load() }, [])

  async function load() {
    setLoading(true)
    try {
      const r = await getMailSettings()
      setMail(r.settings)
      setTemplates(r.templates)
    } finally { setLoading(false) }
  }

  function patchMail<K extends keyof MailSettings>(key: K, value: MailSettings[K]) {
    setMail(prev => prev ? { ...prev, [key]: value } : prev)
  }

  async function save(e?: FormEvent) {
    e?.preventDefault()
    if (!mail) return
    setSaving(true)
    try {
      const payload: MailSettings = { ...mail }
      // If admin didn't elect to change password, drop the field so backend
      // keeps the existing one. has_smtp_password tells us there IS one.
      if (!changePwd) delete payload.smtp_password
      const saved = await putMailSettings(payload)
      setMail(saved)
      setChangePwd(false)
      pushSnack(t('settings.mail.saved'), 'success')
    } finally { setSaving(false) }
  }

  async function test() {
    if (!testTo) return
    setTestBusy(true)
    try {
      await sendTestMail(testTo)
      pushSnack(t('settings.mail.test_sent'), 'success')
    } finally { setTestBusy(false) }
  }

  function patchTpl(kind: MailReminderKind, patch: Partial<MailTemplate>) {
    setTemplates(prev => prev.map(t => t.kind === kind ? { ...t, ...patch } : t))
  }

  async function saveTpl(tpl: MailTemplate) {
    setTplBusy(true)
    try {
      const saved = await putMailTemplate(tpl)
      setTemplates(prev => prev.map(t => t.kind === saved.kind ? saved : t))
      pushSnack(t('settings.mail.saved'), 'success')
    } finally { setTplBusy(false) }
  }

  async function previewTpl(tpl: MailTemplate) {
    setPreviewBusy(true)
    try {
      const rendered = await previewMailTemplate(tpl)
      setPreview({ ...rendered, kind: tpl.kind })
    } finally { setPreviewBusy(false) }
  }

  async function resetTpl(kind: MailReminderKind) {
    if (!(await confirm({
      title: t('settings.mail.reset_confirm_title', { defaultValue: '重置为默认模板？' }),
      message: t('settings.mail.reset_confirm_body', { defaultValue: '当前模板将被默认模板覆盖，自定义内容会丢失。' }),
      confirmText: t('settings.mail.reset_confirm_ok', { defaultValue: '重置' }),
      destructive: true,
    }))) return
    setTplBusy(true)
    try {
      const restored = await resetMailTemplate(kind)
      setTemplates(prev => prev.map(t => t.kind === restored.kind ? restored : t))
      pushSnack(t('settings.mail.reset_done', { defaultValue: '已重置为默认模板' }), 'success')
    } finally { setTplBusy(false) }
  }

  if (loading || !mail) {
    return <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}><CircularProgress /></Box>
  }

  const TPL_KINDS: MailReminderKind[] = ['expire_before', 'expired', 'traffic_low', 'traffic_exhausted', 'account_disabled', 'account_enabled', 'announcement']
  // Fall back to a synthesized empty template if the backend response doesn't
  // include the active kind (e.g., user is on a pre-update binary that doesn't
  // know about `traffic_exhausted` yet). Without this, switching to such a tab
  // would silently render nothing — which looks like "click does nothing".
  // The user can click "重置为默认" to pull the real default from the backend.
  const currentTpl: MailTemplate = templates.find(t => t.kind === activeTpl) ?? {
    kind: activeTpl,
    subject: '',
    body: '',
    enabled: false,
  }
  const tplMissing = !templates.some(t => t.kind === activeTpl)

  return (
    <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button variant="contained" startIcon={saving ? <CircularProgress size={14} color="inherit" /> : <SaveIcon />}
          disabled={saving} type="submit">
          {t('settings.save')}
        </Button>
      </Box>

      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <FormControlLabel label={t('settings.mail.enabled')}
          control={<Switch checked={mail.enabled} onChange={(_, c) => patchMail('enabled', c)} />}
          sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
      </Card>

      {/* SMTP server */}
      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <Typography sx={{ fontWeight: 500, mb: 1.5 }}>{t('settings.mail.section_smtp')}</Typography>
        <Divider sx={{ mb: 2 }} />
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            <TextField fullWidth label={t('settings.mail.smtp_host')}
              value={mail.smtp_host} onChange={e => patchMail('smtp_host', e.target.value)}
              sx={{ flex: '2 1 280px', '& input': {  } }} />
            <TextField type="number" label={t('settings.mail.smtp_port')}
              value={mail.smtp_port} onChange={e => patchMail('smtp_port', Number(e.target.value))}
              sx={{ width: 120 }} />
          </Box>
          <TextField select size="small" fullWidth label={t('settings.mail.encryption')}
            value={mail.encryption}
            onChange={e => patchMail('encryption', e.target.value as MailSettings['encryption'])}>
            <MenuItem value="none">{t('settings.mail.encryption_none')}</MenuItem>
            <MenuItem value="starttls">{t('settings.mail.encryption_starttls')}</MenuItem>
            <MenuItem value="tls">{t('settings.mail.encryption_tls')}</MenuItem>
          </TextField>
          <TextField fullWidth label={t('settings.mail.smtp_username')}
            value={mail.smtp_username} onChange={e => patchMail('smtp_username', e.target.value)} />

          {/* Password — kept-unchanged pattern */}
          {mail.has_smtp_password && !changePwd ? (
            <Box>
              <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mb: 0.5 }}>{t('settings.mail.smtp_password')}</Typography>
              <Box sx={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                gap: 1.5, height: 56, px: 1.75,
                borderRadius: 1.5, border: `1px solid ${md.outlineVariant}`,
              }}>
                <Typography variant="body2">{t('settings.mail.password_kept')}</Typography>
                <Button size="small" variant="text" onClick={() => setChangePwd(true)}>
                  {t('settings.mail.password_change')}
                </Button>
              </Box>
            </Box>
          ) : (
            <TextField fullWidth type={showPwd ? 'text' : 'password'} label={t('settings.mail.smtp_password')}
              value={mail.smtp_password ?? ''}
              onChange={e => patchMail('smtp_password', e.target.value)}
              autoComplete="new-password"
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton size="small" onClick={() => setShowPwd(!showPwd)}>
                      {showPwd ? <VisibilityOffIcon fontSize="small" /> : <VisibilityIcon fontSize="small" />}
                    </IconButton>
                  </InputAdornment>
                ),
              }} />
          )}
        </Box>
      </Card>

      {/* Sender */}
      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <Typography sx={{ fontWeight: 500, mb: 1.5 }}>{t('settings.mail.section_sender')}</Typography>
        <Divider sx={{ mb: 2 }} />
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <TextField fullWidth label={t('settings.mail.from_email')}
            value={mail.from_email} onChange={e => patchMail('from_email', e.target.value)}
            sx={{ '& input': {  } }} />
          <TextField fullWidth label={t('settings.mail.from_name')}
            value={mail.from_name} onChange={e => patchMail('from_name', e.target.value)} />
        </Box>
      </Card>

      {/* Thresholds */}
      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <Typography sx={{ fontWeight: 500, mb: 1.5 }}>{t('settings.mail.section_thresholds')}</Typography>
        <Divider sx={{ mb: 2 }} />
        <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
          <TextField type="number" label={t('settings.mail.expire_before_days')}
            value={mail.expire_before_days}
            onChange={e => patchMail('expire_before_days', Number(e.target.value))}
            sx={{ flex: '1 1 240px' }} />
          <TextField type="number" label={t('settings.mail.traffic_remain_percent')}
            value={mail.traffic_remain_percent}
            onChange={e => patchMail('traffic_remain_percent', Number(e.target.value))}
            sx={{ flex: '1 1 240px' }} />
        </Box>
      </Card>

      {/* Test send */}
      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <Typography sx={{ fontWeight: 500, mb: 1.5 }}>{t('settings.mail.section_test')}</Typography>
        <Divider sx={{ mb: 2 }} />
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-end' }}>
          <TextField fullWidth label={t('settings.mail.test_to')} type="email"
            value={testTo} onChange={e => setTestTo(e.target.value)} />
          <Button variant="outlined" disabled={!testTo || testBusy} onClick={test}
            startIcon={testBusy ? <CircularProgress size={14} /> : <SendIcon />}>
            {t('settings.mail.test_send')}
          </Button>
        </Box>
      </Card>

      {/* Templates */}
      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <Typography sx={{ fontWeight: 500, mb: 1.5 }}>{t('settings.mail.section_templates')}</Typography>
        <Divider sx={{ mb: 2 }} />
        <Tabs value={activeTpl} onChange={(_, v) => setActiveTpl(v as MailReminderKind)}
          variant="scrollable" scrollButtons="auto"
          sx={{ borderBottom: `1px solid ${md.outlineVariant}`, mb: 2 }}>
          {TPL_KINDS.map(k => <Tab key={k} value={k} label={t(`settings.mail.kind.${k}`)} />)}
        </Tabs>
        {tplMissing && (
          <Box sx={{
            mb: 2, p: 1.5, borderRadius: 1.5,
            bgcolor: md.tertiaryContainer, color: md.onTertiaryContainer,
            fontSize: 13, display: 'flex', alignItems: 'center', gap: 1,
          }}>
            <InfoOutlinedIcon fontSize="small" />
            {t('settings.mail.tpl_missing_hint', { defaultValue: '该模板尚未初始化。点击"重置为默认"加载默认内容。' })}
          </Box>
        )}
        {(
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <FormControlLabel label={t('settings.mail.tpl_enabled')}
              control={<Switch checked={currentTpl.enabled}
                onChange={(_, c) => patchTpl(currentTpl.kind, { enabled: c })} />}
              sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
            <TextField fullWidth label={t('settings.mail.tpl_subject')}
              value={currentTpl.subject}
              onChange={e => patchTpl(currentTpl.kind, { subject: e.target.value })} />
            <TextField fullWidth multiline minRows={10} maxRows={20} label={t('settings.mail.tpl_body')}
              value={currentTpl.body}
              onChange={e => patchTpl(currentTpl.kind, { body: e.target.value })}
              sx={{ '& textarea': { fontSize: 13 } }} />
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
              <Button type="button" variant="text" color="warning" disabled={tplBusy}
                onClick={() => resetTpl(currentTpl.kind)}>
                {t('settings.mail.tpl_reset', { defaultValue: '重置为默认' })}
              </Button>
              <Box sx={{ display: 'flex', gap: 1 }}>
                <Button type="button" variant="outlined" disabled={previewBusy}
                  startIcon={previewBusy ? <CircularProgress size={14} /> : <VisibilityIcon />}
                  onClick={() => previewTpl(currentTpl)}>
                  {t('settings.mail.tpl_preview')}
                </Button>
                <Button variant="contained" disabled={tplBusy}
                  startIcon={tplBusy ? <CircularProgress size={14} color="inherit" /> : <SaveIcon />}
                  onClick={() => saveTpl(currentTpl)}>
                  {t('settings.mail.tpl_save')}
                </Button>
              </Box>
            </Box>
          </Box>
        )}
      </Card>

      <Dialog open={!!preview} onClose={() => setPreview(null)} maxWidth="md" fullWidth
        PaperProps={{ sx: { bgcolor: md.surfaceContainerHigh } }}>
        <DialogTitle>{preview && t('settings.mail.preview_title', { kind: t(`settings.mail.kind.${preview.kind}`) })}</DialogTitle>
        <DialogContent dividers>
          <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mb: 0.75 }}>
            {t('settings.mail.preview_subject')}
          </Typography>
          <Box sx={{
            px: 1.5, py: 1.25, mb: 2, borderRadius: 1,
            border: `1px solid ${md.outlineVariant}`,
            bgcolor: md.surfaceContainerLow,
            wordBreak: 'break-word',
          }}>
            {preview?.subject}
          </Box>
          <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mb: 0.75 }}>
            {t('settings.mail.preview_body')}
          </Typography>
          <Box sx={{ height: 520, border: `1px solid ${md.outlineVariant}`, bgcolor: '#fff' }}>
            <iframe
              title={t('settings.mail.preview_body')}
              srcDoc={preview?.body || ''}
              style={{ width: '100%', height: '100%', border: 0, background: '#fff' }}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPreview(null)}>{t('settings.mail.preview_close')}</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

type MdShape = {
  outlineVariant: string
  outline: string
  onSurface: string
  onSurfaceVariant: string
  surfaceContainerLow: string
  surfaceContainerHigh: string
  surfaceContainerHighest: string
  primary: string
}

function QuickLinksEditor({ links, onChange, md }: { links: QuickLink[]; onChange: (v: QuickLink[]) => void; md: MdShape }) {
  const { t } = useTranslation('admin')
  const update = (i: number, patch: Partial<QuickLink>) =>
    onChange(links.map((l, idx) => idx === i ? { ...l, ...patch } : l))
  const remove = (i: number) => onChange(links.filter((_, idx) => idx !== i))
  const add = () => onChange([
    ...links,
    { label: '', url: '', new_window: true, enabled: true, sort: (links.at(-1)?.sort ?? 0) + 10 },
  ])
  return (
    <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
      <Typography sx={{ fontWeight: 500, mb: 1.5, color: md.onSurface }}>{t('settings.portal.section_quick_links')}</Typography>
      <Divider sx={{ mb: 2 }} />
      {links.length === 0 ? (
        <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant, py: 2, textAlign: 'center' }}>
          {t('settings.portal.no_links')}
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          {links.map((l, i) => (
            <Box key={i} sx={{
              display: 'flex', flexWrap: 'wrap', gap: 1.25, alignItems: 'center',
              p: 1.5, borderRadius: 2, border: `1px solid ${md.outlineVariant}`,
              bgcolor: md.surfaceContainerHigh,
            }}>
              <TextField size="small" label={t('settings.portal.link_table.label')}
                value={l.label} onChange={e => update(i, { label: e.target.value })}
                sx={{ flex: '1 1 160px' }} />
              <TextField size="small" label={t('settings.portal.link_table.url')}
                value={l.url} onChange={e => update(i, { url: e.target.value })}
                sx={{ flex: '2 1 240px' }} />
              <TextField size="small" type="number" label={t('settings.portal.link_table.sort')}
                value={l.sort} onChange={e => update(i, { sort: Number(e.target.value) })}
                sx={{ width: 90 }} />
              <FormControlLabel
                label={t('settings.portal.link_table.new_window')}
                control={<Switch size="small" checked={l.new_window}
                  onChange={(_, c) => update(i, { new_window: c })} />}
                sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1, fontSize: 13 } }}
              />
              <FormControlLabel
                label={t('settings.portal.link_table.enabled')}
                control={<Switch size="small" checked={l.enabled}
                  onChange={(_, c) => update(i, { enabled: c })} />}
                sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1, fontSize: 13 } }}
              />
              <IconButton size="small" onClick={() => remove(i)} sx={{ color: md.onSurfaceVariant }}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Box>
          ))}
        </Box>
      )}
      <Box sx={{ mt: 2 }}>
        <Button variant="outlined" size="small" startIcon={<AddIcon />} onClick={add}>
          {t('settings.portal.add_link')}
        </Button>
      </Box>
    </Card>
  )
}

function ClientRulesEditor({ rules, onChange, md }: { rules: SubClientRule[]; onChange: (v: SubClientRule[]) => void; md: MdShape }) {
  const { t } = useTranslation('admin')
  const update = (i: number, patch: Partial<SubClientRule>) =>
    onChange(rules.map((r, idx) => idx === i ? { ...r, ...patch } : r))
  const remove = (i: number) => onChange(rules.filter((_, idx) => idx !== i))
  const add = () => onChange([
    ...rules,
    { name: '', keywords: [], render_format: 'mihomo', enabled: true },
  ])
  return (
    <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
      <Typography sx={{ fontWeight: 500, mb: 1.5, color: md.onSurface }}>{t('settings.subscription.section_clients')}</Typography>
      <Divider sx={{ mb: 2 }} />
      {rules.length === 0 ? (
        <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant, py: 2, textAlign: 'center' }}>
          {t('settings.subscription.no_rules')}
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          {rules.map((r, i) => (
            <Box key={i} sx={{
              display: 'flex', flexWrap: 'wrap', gap: 1.25, alignItems: 'center',
              p: 1.5, borderRadius: 2, border: `1px solid ${md.outlineVariant}`,
              bgcolor: md.surfaceContainerHigh,
            }}>
              <TextField size="small" label={t('settings.subscription.rule_field.name')}
                value={r.name} onChange={e => update(i, { name: e.target.value })}
                sx={{ flex: '1 1 160px' }} />
              <TextField size="small" label={t('settings.subscription.rule_field.keywords')}
                value={r.keywords.join(', ')}
                onChange={e => update(i, { keywords: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })}
                sx={{ flex: '2 1 240px' }} />
              <TextField select size="small" label={t('settings.subscription.rule_field.render_format')}
                value={r.render_format}
                onChange={e => update(i, { render_format: e.target.value as 'mihomo' | 'sing-box' })}
                sx={{ width: 150 }}>
                <MenuItem value="mihomo">mihomo</MenuItem>
                <MenuItem value="sing-box">sing-box</MenuItem>
              </TextField>
              <FormControlLabel
                label={t('settings.subscription.rule_field.enabled')}
                control={<Switch size="small" checked={r.enabled}
                  onChange={(_, c) => update(i, { enabled: c })} />}
                sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1, fontSize: 13 } }}
              />
              <IconButton size="small" onClick={() => remove(i)} sx={{ color: md.onSurfaceVariant }}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Box>
          ))}
        </Box>
      )}
      <Box sx={{ mt: 2 }}>
        <Button variant="outlined" size="small" startIcon={<AddIcon />} onClick={add}>
          {t('settings.subscription.add_rule')}
        </Button>
      </Box>
    </Card>
  )
}

function ImportClientsEditor({ clients, onChange, md }: { clients: SubImportClient[]; onChange: (v: SubImportClient[]) => void; md: MdShape }) {
  const { t } = useTranslation('admin')
  const update = (i: number, patch: Partial<SubImportClient>) =>
    onChange(clients.map((c, idx) => idx === i ? { ...c, ...patch } : c))
  const remove = (i: number) => onChange(clients.filter((_, idx) => idx !== i))
  const add = () => onChange([
    ...clients,
    {
      name: '', platforms: [], render_format: 'mihomo',
      import_url_template: '', install_url: '', enabled: true,
      sort: (clients.at(-1)?.sort ?? 0) + 10,
      recommended_for: [],
    },
  ])
  const PLATFORM_OPTIONS: Array<SubImportClient['platforms'][number]> = ['windows', 'macos', 'linux', 'android', 'ios', 'other']
  return (
    <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
      <Typography sx={{ fontWeight: 500, mb: 1.5, color: md.onSurface }}>{t('settings.subscription.section_imports')}</Typography>
      <Divider sx={{ mb: 2 }} />
      {clients.length === 0 ? (
        <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant, py: 2, textAlign: 'center' }}>
          {t('settings.subscription.no_clients')}
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {clients.map((c, i) => (
            <Box key={i} sx={{
              display: 'flex', flexDirection: 'column', gap: 1.25,
              p: 2, borderRadius: 2, border: `1px solid ${md.outlineVariant}`,
              bgcolor: md.surfaceContainerHigh,
            }}>
              <Box sx={{ display: 'flex', gap: 1.25, flexWrap: 'wrap', alignItems: 'center' }}>
                <TextField size="small" label={t('settings.subscription.client_field.name')}
                  value={c.name} onChange={e => update(i, { name: e.target.value })}
                  sx={{ flex: '1 1 200px' }} />
                <TextField select size="small" label={t('settings.subscription.client_field.render_format')}
                  value={c.render_format}
                  onChange={e => update(i, { render_format: e.target.value as 'mihomo' | 'sing-box' })}
                  sx={{ width: 150 }}>
                  <MenuItem value="mihomo">mihomo</MenuItem>
                  <MenuItem value="sing-box">sing-box</MenuItem>
                </TextField>
                <TextField size="small" type="number" label={t('settings.subscription.client_field.sort')}
                  value={c.sort} onChange={e => update(i, { sort: Number(e.target.value) })}
                  sx={{ width: 100 }} />
                <FormControlLabel
                  label={t('settings.subscription.client_field.enabled')}
                  control={<Switch size="small" checked={c.enabled}
                    onChange={(_, ck) => update(i, { enabled: ck })} />}
                  sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1, fontSize: 13 } }}
                />
                <Box sx={{ flex: 1 }} />
                <IconButton size="small" onClick={() => remove(i)} sx={{ color: md.onSurfaceVariant }}>
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Box>
              <Box>
                <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mb: 0.75 }}>
                  {t('settings.subscription.client_field.platforms', { defaultValue: '支持的平台' })}
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
                  {PLATFORM_OPTIONS.map(p => {
                    const selected = c.platforms.includes(p)
                    return (
                      <Chip key={p}
                        size="small"
                        label={t(`settings.subscription.platform.${p}`, { defaultValue: p })}
                        color={selected ? 'primary' : 'default'}
                        variant={selected ? 'filled' : 'outlined'}
                        onClick={() => {
                          const cur = c.platforms
                          const nextPlatforms = selected ? cur.filter(x => x !== p) : [...cur, p]
                          // Removing a platform should also drop it from
                          // recommended_for — otherwise the backend silently
                          // strips it on save and the admin would wonder why
                          // the highlight chip disappeared mid-edit.
                          const nextRec = (c.recommended_for ?? []).filter(x => nextPlatforms.includes(x))
                          update(i, {
                            platforms: nextPlatforms as SubImportClient['platforms'],
                            recommended_for: nextRec as SubImportClient['recommended_for'],
                          })
                        }} />
                    )
                  })}
                </Box>
              </Box>
              <Box>
                <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mb: 0.75 }}>
                  {t('settings.subscription.client_field.recommended_for', { defaultValue: '在哪些平台作为推荐客户端' })}
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
                  {PLATFORM_OPTIONS.filter(p => c.platforms.includes(p)).map(p => {
                    const selected = (c.recommended_for ?? []).includes(p)
                    return (
                      <Chip key={p}
                        size="small"
                        label={t(`settings.subscription.platform.${p}`, { defaultValue: p })}
                        color={selected ? 'primary' : 'default'}
                        variant={selected ? 'filled' : 'outlined'}
                        onClick={() => {
                          const cur = c.recommended_for ?? []
                          update(i, {
                            recommended_for: selected ? cur.filter(x => x !== p) : [...cur, p],
                          })
                        }} />
                    )
                  })}
                  {c.platforms.length === 0 && (
                    <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, fontStyle: 'italic' }}>
                      {t('settings.subscription.client_field.recommended_for_empty', { defaultValue: '请先选择支持的平台' })}
                    </Typography>
                  )}
                </Box>
              </Box>
              <TextField size="small" fullWidth label={t('settings.subscription.client_field.import_url_template')}
                value={c.import_url_template}
                onChange={e => update(i, { import_url_template: e.target.value })} />
              <TextField size="small" fullWidth label={t('settings.subscription.client_field.install_url')}
                value={c.install_url}
                onChange={e => update(i, { install_url: e.target.value })} />
            </Box>
          ))}
        </Box>
      )}
      <Box sx={{ mt: 2 }}>
        <Button variant="outlined" size="small" startIcon={<AddIcon />} onClick={add}>
          {t('settings.subscription.add_client')}
        </Button>
      </Box>
    </Card>
  )
}

interface SectionProps { title: string; children: React.ReactNode; md: MdShape }
function Section({ title, children, md }: SectionProps) {
  return (
    <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
      <Typography sx={{ fontWeight: 500, mb: 1.5, color: md.onSurface }}>{title}</Typography>
      <Divider sx={{ mb: 2 }} />
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>{children}</Box>
    </Card>
  )
}

function Pair({ children }: { children: React.ReactNode }) {
  return <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap', '& > *': { flex: '1 1 220px' } }}>{children}</Box>
}

function NumField({ label, value, onChange, helperText }: { label: string; value: number; onChange: (v: number) => void; helperText?: string }) {
  return (
    <TextField fullWidth type="number" label={label}
      value={value} onChange={e => onChange(Number(e.target.value))}
      inputProps={{ min: 0 }} helperText={helperText} />
  )
}

function ResetPeriodField({ value, onChange }: { value: string; onChange: (v: string) => void; md: MdShape }) {
  const { t } = useTranslation('admin')
  return (
    <TextField select size="small" fullWidth label={t('users.field.traffic_reset_period')}
      value={value} onChange={e => onChange(e.target.value)}>
      <MenuItem value="never">{t('users.reset_period.never')}</MenuItem>
      <MenuItem value="monthly">{t('users.reset_period.monthly')}</MenuItem>
      <MenuItem value="quarterly">{t('users.reset_period.quarterly')}</MenuItem>
    </TextField>
  )
}

function SsoTab() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')
  const [sub, setSub] = useTabParam<'saml' | 'oidc'>('sub', 'saml', ['saml', 'oidc'])

  return (
    <Box>
      <Tabs value={sub} onChange={(_, v) => setSub(v)} sx={{ mb: 3, borderBottom: `1px solid ${md.outlineVariant}` }}>
        <Tab value="saml" label={t('settings.sso.tab_saml')} />
        <Tab value="oidc" label={t('settings.sso.tab_oidc')} />
      </Tabs>
      {sub === 'saml' ? <SamlPanel /> : <OidcPanel />}
    </Box>
  )
}

function SamlPanel() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')
  const [cfg, setCfg] = useState<SAMLConfig | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [changeKey, setChangeKey] = useState(false)
  const [keyPem, setKeyPem] = useState('')

  useEffect(() => { void load() }, [])
  async function load() {
    setLoading(true)
    try { setCfg(await getSAML()) }
    finally { setLoading(false) }
  }
  function patch<K extends keyof SAMLConfig>(key: K, value: SAMLConfig[K]) {
    setCfg(prev => prev ? { ...prev, [key]: value } : prev)
  }
  function patchSp<K extends keyof SAMLConfig['sp']>(key: K, value: SAMLConfig['sp'][K]) {
    setCfg(prev => prev ? { ...prev, sp: { ...prev.sp, [key]: value } } : prev)
  }
  function patchIdp<K extends keyof SAMLConfig['idp']>(key: K, value: SAMLConfig['idp'][K]) {
    setCfg(prev => prev ? { ...prev, idp: { ...prev.idp, [key]: value } } : prev)
  }
  function patchAttr<K extends keyof SAMLConfig['attribute_mapping']>(key: K, value: SAMLConfig['attribute_mapping'][K]) {
    setCfg(prev => prev ? { ...prev, attribute_mapping: { ...prev.attribute_mapping, [key]: value } } : prev)
  }
  function patchDef<K extends keyof SAMLConfig['new_user_defaults']>(key: K, value: SAMLConfig['new_user_defaults'][K]) {
    setCfg(prev => prev ? { ...prev, new_user_defaults: { ...prev.new_user_defaults, [key]: value } } : prev)
  }

  async function save(e?: FormEvent) {
    e?.preventDefault()
    if (!cfg) return
    setSaving(true)
    try {
      const res = await putSAML({
        ...cfg,
        sp: {
          entity_id: cfg.sp.entity_id, acs_url: cfg.sp.acs_url, cert_pem: cfg.sp.cert_pem,
          key_pem: changeKey ? keyPem : '',
        },
      })
      setCfg(res.config)
      setChangeKey(false); setKeyPem('')
      if (res.reload_error) pushSnack(t('settings.sso.reload_error', { error: res.reload_error }), 'warning')
      else pushSnack(t('settings.sso.saved'), 'success')
    } finally { setSaving(false) }
  }

  if (loading || !cfg) return <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}><CircularProgress /></Box>

  return (
    <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button variant="contained" type="submit" disabled={saving}
          startIcon={saving ? <CircularProgress size={14} color="inherit" /> : <SaveIcon />}>
          {t('settings.save')}
        </Button>
      </Box>

      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <FormControlLabel label={t('settings.sso.saml.enabled')}
          control={<Switch checked={cfg.enabled} onChange={(_, c) => patch('enabled', c)} />}
          sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
        <Box sx={{ mt: 2 }}>
          <TextField select size="small" fullWidth label={t('settings.sso.saml.mode')}
            value={cfg.mode} onChange={e => patch('mode', e.target.value as 'auto' | 'manual')}>
            <MenuItem value="auto">{t('settings.sso.saml.mode_auto')}</MenuItem>
            <MenuItem value="manual">{t('settings.sso.saml.mode_manual')}</MenuItem>
          </TextField>
        </Box>
      </Card>

      <Section title={t('settings.sso.saml.sp_section')} md={md}>
        <TextField fullWidth label={t('settings.sso.saml.sp_entity_id')} value={cfg.sp.entity_id}
          onChange={e => patchSp('entity_id', e.target.value)} />
        <TextField fullWidth label={t('settings.sso.saml.sp_acs_url')} value={cfg.sp.acs_url}
          onChange={e => patchSp('acs_url', e.target.value)} />
        <TextField fullWidth multiline minRows={4} label={t('settings.sso.saml.sp_cert')} value={cfg.sp.cert_pem}
          onChange={e => patchSp('cert_pem', e.target.value)}
          sx={{ '& textarea': { fontSize: 12 } }} />
        {cfg.sp.has_key_pem && !changeKey ? (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, height: 56, px: 1.75, borderRadius: 1.5, border: `1px solid ${md.outlineVariant}` }}>
            <Typography variant="body2">{t('settings.sso.saml.sp_key_kept')}</Typography>
            <Button size="small" variant="text" onClick={() => setChangeKey(true)}>{t('settings.sso.saml.sp_key_change')}</Button>
          </Box>
        ) : (
          <TextField fullWidth multiline minRows={4} label={t('settings.sso.saml.sp_key')} value={keyPem}
            onChange={e => setKeyPem(e.target.value)}
            sx={{ '& textarea': { fontSize: 12 } }} />
        )}
      </Section>

      <Section title={t('settings.sso.saml.idp_section')} md={md}>
        <TextField fullWidth label={t('settings.sso.saml.idp_metadata_url')} value={cfg.idp.metadata_url}
          onChange={e => patchIdp('metadata_url', e.target.value)} />
        <NumField label={t('settings.sso.saml.idp_refresh_hours')} value={cfg.idp.metadata_refresh_hours}
          onChange={v => patchIdp('metadata_refresh_hours', v)} />
      </Section>

      <Section title={t('settings.sso.saml.attr_section')} md={md}>
        <Pair>
          <TextField fullWidth label={t('settings.sso.saml.attr_upn')} value={cfg.attribute_mapping.upn}
            onChange={e => patchAttr('upn', e.target.value)} />
          <TextField fullWidth label={t('settings.sso.saml.attr_email')} value={cfg.attribute_mapping.email}
            onChange={e => patchAttr('email', e.target.value)} />
        </Pair>
        <Pair>
          <TextField fullWidth label={t('settings.sso.saml.attr_display_name')} value={cfg.attribute_mapping.display_name}
            onChange={e => patchAttr('display_name', e.target.value)} />
          <TextField fullWidth label={t('settings.sso.saml.attr_groups')} value={cfg.attribute_mapping.groups}
            onChange={e => patchAttr('groups', e.target.value)} />
        </Pair>
        <TextField fullWidth label={t('settings.sso.saml.admin_groups')} value={cfg.admin_group_ids.join(', ')}
          onChange={e => patch('admin_group_ids', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
        <TextField fullWidth label={t('settings.sso.saml.default_group')} value={cfg.default_group_slug}
          onChange={e => patch('default_group_slug', e.target.value)} />
      </Section>

      <Section title={t('settings.sso.saml.new_user_section')} md={md}>
        <Pair>
          <NumField label={t('settings.sso.saml.expire_days')} value={cfg.new_user_defaults.expire_days}
            onChange={v => patchDef('expire_days', v)} />
          <NumField label={t('settings.sso.saml.traffic_limit_gb')}
            value={Math.round(cfg.new_user_defaults.traffic_limit_bytes / 1024 / 1024 / 1024)}
            onChange={v => patchDef('traffic_limit_bytes', v * 1024 * 1024 * 1024)} />
        </Pair>
        <ResetPeriodField
          value={cfg.new_user_defaults.traffic_reset_period}
          onChange={v => patchDef('traffic_reset_period', v)}
          md={md}
        />
      </Section>
    </Box>
  )
}

function OidcPanel() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')
  const [cfg, setCfg] = useState<OIDCConfig | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [changeSecret, setChangeSecret] = useState(false)
  const [secret, setSecret] = useState('')

  useEffect(() => { void load() }, [])
  async function load() {
    setLoading(true)
    try { setCfg(await getOIDC()) }
    finally { setLoading(false) }
  }
  function patch<K extends keyof OIDCConfig>(key: K, value: OIDCConfig[K]) {
    setCfg(prev => prev ? { ...prev, [key]: value } : prev)
  }
  function patchAttr<K extends keyof OIDCConfig['attribute_mapping']>(key: K, value: OIDCConfig['attribute_mapping'][K]) {
    setCfg(prev => prev ? { ...prev, attribute_mapping: { ...prev.attribute_mapping, [key]: value } } : prev)
  }
  function patchDef<K extends keyof OIDCConfig['new_user_defaults']>(key: K, value: OIDCConfig['new_user_defaults'][K]) {
    setCfg(prev => prev ? { ...prev, new_user_defaults: { ...prev.new_user_defaults, [key]: value } } : prev)
  }

  async function save(e?: FormEvent) {
    e?.preventDefault()
    if (!cfg) return
    setSaving(true)
    try {
      const res = await putOIDC({ ...cfg, client_secret: changeSecret ? secret : '' })
      setCfg(res.config)
      setChangeSecret(false); setSecret('')
      if (res.reload_error) pushSnack(t('settings.sso.reload_error', { error: res.reload_error }), 'warning')
      else pushSnack(t('settings.sso.saved'), 'success')
    } finally { setSaving(false) }
  }

  if (loading || !cfg) return <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}><CircularProgress /></Box>

  return (
    <Box component="form" onSubmit={save} sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button variant="contained" type="submit" disabled={saving}
          startIcon={saving ? <CircularProgress size={14} color="inherit" /> : <SaveIcon />}>
          {t('settings.save')}
        </Button>
      </Box>

      <Card sx={{ p: 3, bgcolor: md.surfaceContainerLow, border: `1px solid ${md.outlineVariant}` }}>
        <FormControlLabel label={t('settings.sso.oidc.enabled')}
          control={<Switch checked={cfg.enabled} onChange={(_, c) => patch('enabled', c)} />}
          sx={{ ml: 0, '& .MuiFormControlLabel-label': { ml: 1.5 } }} />
      </Card>

      <Section title={t('settings.sso.oidc.issuer_url')} md={md}>
        <TextField fullWidth label={t('settings.sso.oidc.issuer_url')} value={cfg.issuer_url}
          onChange={e => patch('issuer_url', e.target.value)} />
        <TextField fullWidth label={t('settings.sso.oidc.client_id')} value={cfg.client_id}
          onChange={e => patch('client_id', e.target.value)}
          sx={{ '& input': {  } }} />
        {cfg.has_client_secret && !changeSecret ? (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, height: 56, px: 1.75, borderRadius: 1.5, border: `1px solid ${md.outlineVariant}` }}>
            <Typography variant="body2">{t('settings.sso.oidc.client_secret_kept')}</Typography>
            <Button size="small" variant="text" onClick={() => setChangeSecret(true)}>{t('settings.sso.oidc.client_secret_change')}</Button>
          </Box>
        ) : (
          <TextField fullWidth type="password" label={t('settings.sso.oidc.client_secret')} value={secret}
            onChange={e => setSecret(e.target.value)} autoComplete="new-password" />
        )}
        <TextField fullWidth label={t('settings.sso.oidc.redirect_url')} value={cfg.redirect_url}
          onChange={e => patch('redirect_url', e.target.value)} />
        <TextField fullWidth label={t('settings.sso.oidc.scopes')} value={cfg.scopes.join(', ')}
          onChange={e => patch('scopes', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
      </Section>

      <Section title={t('settings.sso.oidc.attr_section')} md={md}>
        <Pair>
          <TextField fullWidth label={t('settings.sso.oidc.attr_username')} value={cfg.attribute_mapping.username}
            onChange={e => patchAttr('username', e.target.value)} />
          <TextField fullWidth label={t('settings.sso.oidc.attr_email')} value={cfg.attribute_mapping.email}
            onChange={e => patchAttr('email', e.target.value)} />
        </Pair>
        <Pair>
          <TextField fullWidth label={t('settings.sso.oidc.attr_display_name')} value={cfg.attribute_mapping.display_name}
            onChange={e => patchAttr('display_name', e.target.value)} />
          <TextField fullWidth label={t('settings.sso.oidc.attr_groups')} value={cfg.attribute_mapping.groups}
            onChange={e => patchAttr('groups', e.target.value)} />
        </Pair>
        <TextField fullWidth label={t('settings.sso.oidc.admin_groups')} value={cfg.admin_group_ids.join(', ')}
          onChange={e => patch('admin_group_ids', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
        <TextField fullWidth label={t('settings.sso.oidc.default_group')} value={cfg.default_group_slug}
          onChange={e => patch('default_group_slug', e.target.value)} />
      </Section>

      <Section title={t('settings.sso.oidc.new_user_section')} md={md}>
        <Pair>
          <NumField label={t('settings.sso.oidc.expire_days')} value={cfg.new_user_defaults.expire_days}
            onChange={v => patchDef('expire_days', v)} />
          <NumField label={t('settings.sso.oidc.traffic_limit_gb')}
            value={Math.round(cfg.new_user_defaults.traffic_limit_bytes / 1024 / 1024 / 1024)}
            onChange={v => patchDef('traffic_limit_bytes', v * 1024 * 1024 * 1024)} />
        </Pair>
        <ResetPeriodField
          value={cfg.new_user_defaults.traffic_reset_period}
          onChange={v => patchDef('traffic_reset_period', v)}
          md={md}
        />
      </Section>
    </Box>
  )
}

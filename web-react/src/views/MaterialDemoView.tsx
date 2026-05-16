import { useMemo, useState } from 'react'
import {
  AppBar,
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Fab,
  IconButton,
  InputBase,
  LinearProgress,
  Snackbar,
  Switch,
  Toolbar,
  Typography,
  alpha,
  useTheme,
} from '@mui/material'
import { useTranslation } from 'react-i18next'
import MenuIcon from '@mui/icons-material/Menu'
import NotificationsNoneIcon from '@mui/icons-material/NotificationsNone'
import DashboardIcon from '@mui/icons-material/Dashboard'
import GroupIcon from '@mui/icons-material/Group'
import DnsIcon from '@mui/icons-material/Dns'
import WorkspacesIcon from '@mui/icons-material/Workspaces'
import RuleIcon from '@mui/icons-material/Rule'
import InsightsIcon from '@mui/icons-material/Insights'
import SettingsIcon from '@mui/icons-material/Settings'
import SwapVertIcon from '@mui/icons-material/SwapVert'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'
import SearchIcon from '@mui/icons-material/Search'
import AddIcon from '@mui/icons-material/Add'
import DownloadIcon from '@mui/icons-material/Download'
import SyncIcon from '@mui/icons-material/Sync'
import CheckIcon from '@mui/icons-material/Check'
import LightbulbOutlinedIcon from '@mui/icons-material/LightbulbOutlined'
import AppearanceMenu from '@/components/AppearanceMenu'
import LanguageMenu from '@/components/LanguageMenu'
import { useAppearanceStore } from '@/stores/appearance'
import { currentLanguage, setLanguage } from '@/i18n'
import type { AppLanguage } from '@/theme'

const NAV_ITEMS = [
  { icon: DashboardIcon, labelKey: 'demo:nav.dashboard' },
  { icon: GroupIcon, labelKey: 'demo:nav.users' },
  { icon: DnsIcon, labelKey: 'demo:nav.nodes' },
  { icon: WorkspacesIcon, labelKey: 'demo:nav.groups' },
  { icon: RuleIcon, labelKey: 'demo:nav.rules' },
  { icon: InsightsIcon, labelKey: 'demo:nav.traffic' },
  { icon: SettingsIcon, labelKey: 'demo:nav.settings' },
] as const

const METRICS = [
  { labelKey: 'demo:metric.online_nodes', value: '18', delta: '+2', tone: 'primary', Icon: DnsIcon },
  { labelKey: 'demo:metric.today_traffic', value: '742 GB', delta: '+12.4%', tone: 'tertiary', Icon: SwapVertIcon },
  { labelKey: 'demo:metric.active_users', value: '163', delta: '+8', tone: 'secondary', Icon: GroupIcon },
  { labelKey: 'demo:metric.sub_requests', value: '4.2k', delta: '-3.1%', tone: 'error', Icon: CloudDownloadIcon },
] as const

interface DemoNode {
  id: number
  name: string
  regionKey: 'hk' | 'jp' | 'sg' | 'us' | 'de' | 'kr'
  regionFallback: string
  protocol: string
  online: boolean
  load: number
  enabled: boolean
}

const INITIAL_NODES: DemoNode[] = [
  { id: 1, name: 'HK · Equinix HK1', regionKey: 'hk', regionFallback: 'Hong Kong', protocol: 'VLESS · Reality', online: true, load: 42, enabled: true },
  { id: 2, name: 'JP · Tokyo Akiba', regionKey: 'jp', regionFallback: 'Japan', protocol: 'VLESS · gRPC', online: true, load: 78, enabled: true },
  { id: 3, name: 'SG · Digital Edge', regionKey: 'sg', regionFallback: 'Singapore', protocol: 'Shadowsocks 2022', online: true, load: 31, enabled: true },
  { id: 4, name: 'US · LAX-CN2', regionKey: 'us', regionFallback: 'USA', protocol: 'VLESS · WS+TLS', online: false, load: 0, enabled: false },
  { id: 5, name: 'DE · Frankfurt', regionKey: 'de', regionFallback: 'Germany', protocol: 'VLESS · Reality', online: true, load: 55, enabled: true },
  { id: 6, name: 'KR · Seoul KT', regionKey: 'kr', regionFallback: 'Korea', protocol: 'Shadowsocks 2022', online: true, load: 18, enabled: true },
]

export default function MaterialDemoView() {
  const { t } = useTranslation(['demo', 'common'])
  const theme = useTheme()
  const md = theme.palette.md
  const appearanceStore = useAppearanceStore()
  const appearance = {
    systemColor: appearanceStore.systemColor,
    userColor: appearanceStore.userColor,
    mode: appearanceStore.mode,
  }
  const language: AppLanguage = currentLanguage()
  const onAppearanceChange = (patch: { userColor?: string | null; mode?: 'light' | 'dark' }) => {
    if ('userColor' in patch) appearanceStore.setUserColor(patch.userColor ?? null)
    if (patch.mode) appearanceStore.setMode(patch.mode)
  }
  const onLanguageChange = (lng: AppLanguage) => setLanguage(lng)
  const [navIndex, setNavIndex] = useState(2)
  const [filter, setFilter] = useState<'all' | 'online' | 'offline'>('all')
  const [search, setSearch] = useState('')
  const [nodes, setNodes] = useState<DemoNode[]>(INITIAL_NODES)
  const [snackMsg, setSnackMsg] = useState<string | null>(null)

  const filtered = useMemo(() => {
    return nodes.filter(n => {
      if (filter === 'online' && !n.online) return false
      if (filter === 'offline' && n.online) return false
      if (search && !n.name.toLowerCase().includes(search.toLowerCase())) return false
      return true
    })
  }, [nodes, filter, search])

  const onlineCount = nodes.filter(n => n.online).length
  const offlineCount = nodes.length - onlineCount

  function regionLabel(n: DemoNode): string {
    const key = `demo:regions.${n.regionKey}`
    const translated = t(key)
    return translated === key ? n.regionFallback : translated
  }

  function toggleNode(id: number) {
    setNodes(prev => prev.map(n => {
      if (n.id !== id) return n
      const updated = { ...n, enabled: !n.enabled }
      setSnackMsg(
        t(updated.enabled ? 'demo:snack.node_enabled' : 'demo:snack.node_disabled', { name: n.name }),
      )
      return updated
    }))
  }

  function toneContainer(tone: string) {
    switch (tone) {
      case 'secondary': return { bg: md.secondaryContainer, fg: md.onSecondaryContainer }
      case 'tertiary': return { bg: md.tertiaryContainer, fg: md.onTertiaryContainer }
      case 'error': return { bg: md.errorContainer, fg: md.onErrorContainer }
      default: return { bg: md.primaryContainer, fg: md.onPrimaryContainer }
    }
  }

  return (
    <Box sx={{ position: 'fixed', inset: 0, display: 'flex', flexDirection: 'column', bgcolor: md.surface, color: md.onSurface }}>
      {/* Top App Bar */}
      <AppBar position="static">
        <Toolbar sx={{ gap: 0.5, minHeight: 64 }}>
          <IconButton edge="start" aria-label="menu"><MenuIcon /></IconButton>
          <Typography variant="h6" sx={{ ml: 1, fontWeight: 500 }}>{t('demo:brand')}</Typography>
          <Box sx={{ flex: 1 }} />
          <LanguageMenu value={language} onChange={onLanguageChange} />
          <AppearanceMenu state={appearance} onChange={onAppearanceChange} />
          <IconButton aria-label="notifications"><NotificationsNoneIcon /></IconButton>
          <Avatar sx={{ ml: 1, width: 36, height: 36, bgcolor: md.primary, color: md.onPrimary, fontWeight: 500 }}>K</Avatar>
        </Toolbar>
      </AppBar>

      <Box sx={{ flex: 1, display: 'flex', minHeight: 0 }}>
        {/* Navigation Rail */}
        <Box
          component="nav"
          sx={{
            width: 88,
            flexShrink: 0,
            display: { xs: 'none', sm: 'flex' },
            flexDirection: 'column',
            alignItems: 'center',
            py: 1.5,
            gap: 0.5,
          }}
        >
          {NAV_ITEMS.map((item, i) => {
            const Icon = item.icon
            const active = navIndex === i
            return (
              <Box
                key={item.labelKey}
                onClick={() => setNavIndex(i)}
                sx={{
                  width: '100%',
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: 0.5,
                  py: 1,
                  cursor: 'pointer',
                  color: active ? md.onSurface : md.onSurfaceVariant,
                }}
              >
                <Box
                  sx={{
                    width: 56,
                    height: 32,
                    borderRadius: '16px',
                    display: 'grid',
                    placeItems: 'center',
                    bgcolor: active ? md.secondaryContainer : 'transparent',
                    color: active ? md.onSecondaryContainer : 'inherit',
                    transition: 'background-color .2s',
                    '&:hover': { bgcolor: active ? md.secondaryContainer : alpha(md.onSurface, 0.08) },
                  }}
                >
                  <Icon sx={{ fontSize: 24 }} />
                </Box>
                <Typography sx={{ fontSize: 12, fontWeight: 500, letterSpacing: '.5px' }}>{t(item.labelKey)}</Typography>
              </Box>
            )
          })}
        </Box>

        {/* Content */}
        <Box sx={{ flex: 1, overflowY: 'auto', px: { xs: 2, md: 4 }, pt: 3, pb: 12 }}>
          {/* Page header */}
          <Box sx={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', flexWrap: 'wrap', gap: 2, mb: 3 }}>
            <Box>
              <Typography variant="h4">{t('demo:page.headline')}</Typography>
              <Typography variant="body2" sx={{ mt: 0.5 }}>
                {t('demo:page.supporting')}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button variant="text" startIcon={<DownloadIcon />}>{t('common:actions.export')}</Button>
              <Button
                variant="contained"
                startIcon={<SyncIcon />}
                sx={{ bgcolor: md.secondaryContainer, color: md.onSecondaryContainer, '&:hover': { bgcolor: md.secondaryContainer } }}
                disableElevation
              >
                {t('common:actions.sync')}
              </Button>
              <Button
                variant="contained"
                startIcon={<CheckIcon />}
                onClick={() => setSnackMsg(t('demo:snack.import_triggered'))}
              >
                {t('common:actions.confirm')}
              </Button>
            </Box>
          </Box>

          {/* Metric cards */}
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))',
              gap: 2,
              mb: 3,
            }}
          >
            {METRICS.map(m => {
              const tone = toneContainer(m.tone)
              return (
                <Card key={m.labelKey} sx={{ bgcolor: md.surfaceContainerHighest }}>
                  <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2, p: 2.5, '&:last-child': { pb: 2.5 } }}>
                    <Box
                      sx={{
                        width: 56, height: 56, borderRadius: '12px',
                        display: 'grid', placeItems: 'center', flexShrink: 0,
                        bgcolor: tone.bg, color: tone.fg,
                      }}
                    >
                      <m.Icon sx={{ fontSize: 28 }} />
                    </Box>
                    <Box>
                      <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant }}>{t(m.labelKey)}</Typography>
                      <Typography sx={{ fontSize: 28, fontWeight: 500, lineHeight: 1.1, mt: 0.25 }}>{m.value}</Typography>
                      <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant, mt: 0.5 }}>{m.delta} {t('common:status.delta_vs_yesterday')}</Typography>
                    </Box>
                  </CardContent>
                </Card>
              )
            })}
          </Box>

          {/* Node list card */}
          <Card sx={{ bgcolor: md.surfaceContainerLow, boxShadow: '0 1px 2px rgba(0,0,0,.3),0 1px 3px 1px rgba(0,0,0,.15)', mb: 3 }}>
            {/* Header */}
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 2, p: 2.5, pb: 1 }}>
              <Typography variant="h6">{t('demo:list.title')}</Typography>
              <Box sx={{
                display: 'flex', alignItems: 'center', gap: 1,
                height: 40, px: 2, borderRadius: 9999,
                bgcolor: md.surfaceContainer, color: md.onSurfaceVariant,
                width: { xs: '100%', sm: 320 },
              }}>
                <SearchIcon sx={{ fontSize: 20 }} />
                <InputBase
                  placeholder={t('demo:list.search_placeholder')}
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  sx={{ flex: 1, fontSize: 14, color: md.onSurface }}
                />
              </Box>
            </Box>

            {/* Filter chips */}
            <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', alignItems: 'center', px: 2.5, pb: 2 }}>
              <Chip
                label={t('demo:list.filter_all', { count: nodes.length })}
                variant={filter === 'all' ? 'filled' : 'outlined'}
                icon={filter === 'all' ? <CheckIcon sx={{ fontSize: 16 }} /> : undefined}
                onClick={() => setFilter('all')}
              />
              <Chip
                label={t('demo:list.filter_online', { count: onlineCount })}
                variant={filter === 'online' ? 'filled' : 'outlined'}
                icon={filter === 'online' ? <CheckIcon sx={{ fontSize: 16 }} /> : undefined}
                onClick={() => setFilter('online')}
              />
              <Chip
                label={t('demo:list.filter_offline', { count: offlineCount })}
                variant={filter === 'offline' ? 'filled' : 'outlined'}
                icon={filter === 'offline' ? <CheckIcon sx={{ fontSize: 16 }} /> : undefined}
                onClick={() => setFilter('offline')}
              />
              <Box sx={{ width: '1px', height: 24, bgcolor: md.outlineVariant, mx: 0.5 }} />
              {(['hk', 'jp', 'sg', 'us'] as const).map(r => (
                <Chip key={r} label={t(`demo:regions.${r}`)} variant="outlined" />
              ))}
            </Box>

            {/* List */}
            <Box>
              {filtered.map(n => (
                <Box
                  key={n.id}
                  sx={{
                    display: 'flex', alignItems: 'center', gap: 2,
                    px: 2.5, py: 2,
                    borderTop: `1px solid ${md.outlineVariant}`,
                    transition: 'background-color .15s',
                    '&:hover': { bgcolor: alpha(md.onSurface, 0.04) },
                  }}
                >
                  <Box sx={{
                    width: 40, height: 40, borderRadius: '50%',
                    display: 'grid', placeItems: 'center', flexShrink: 0,
                    bgcolor: n.online ? md.primaryContainer : md.surfaceContainerHigh,
                    color: n.online ? md.onPrimaryContainer : md.onSurfaceVariant,
                  }}>
                    <DnsIcon sx={{ fontSize: 22 }} />
                  </Box>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Typography sx={{ fontSize: 16, fontWeight: 500 }}>{n.name}</Typography>
                      <Box sx={{
                        fontSize: 11, fontWeight: 500, px: 1, py: '2px',
                        borderRadius: '4px', letterSpacing: '.3px',
                        bgcolor: n.online ? md.tertiaryContainer : md.errorContainer,
                        color: n.online ? md.onTertiaryContainer : md.onErrorContainer,
                      }}>
                        {t(n.online ? 'common:status.online' : 'common:status.offline')}
                      </Box>
                    </Box>
                    <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant, mt: 0.25 }}>
                      {regionLabel(n)} · {n.protocol}
                    </Typography>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 1, maxWidth: 360 }}>
                      <LinearProgress
                        variant="determinate"
                        value={n.load}
                        sx={{
                          flex: 1, height: 6, borderRadius: 3,
                          bgcolor: md.surfaceContainerHighest,
                          '& .MuiLinearProgress-bar': { bgcolor: md.primary, borderRadius: 3 },
                        }}
                      />
                      <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant }}>{t('common:status.load')} {n.load}%</Typography>
                    </Box>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexShrink: 0 }}>
                    <Button variant="outlined" size="small">{t('common:actions.details')}</Button>
                    <Switch checked={n.enabled} onChange={() => toggleNode(n.id)} />
                  </Box>
                </Box>
              ))}
              {filtered.length === 0 && (
                <Box sx={{ py: 4, textAlign: 'center', color: md.onSurfaceVariant }}>{t('demo:list.empty')}</Box>
              )}
            </Box>
          </Card>

          {/* Outlined tip card */}
          <Card sx={{ border: `1px solid ${md.outlineVariant}`, bgcolor: md.surface }}>
            <CardContent sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', p: 2.5, '&:last-child': { pb: 2.5 } }}>
              <Box sx={{
                width: 40, height: 40, borderRadius: '50%',
                display: 'grid', placeItems: 'center', flexShrink: 0,
                bgcolor: md.primaryContainer, color: md.onPrimaryContainer,
              }}>
                <LightbulbOutlinedIcon sx={{ fontSize: 22 }} />
              </Box>
              <Box>
                <Typography sx={{ fontWeight: 500, mb: 0.5 }}>{t('demo:tip.title')}</Typography>
                <Typography variant="body2">{t('demo:tip.body')}</Typography>
              </Box>
            </CardContent>
          </Card>
        </Box>
      </Box>

      {/* Extended FAB */}
      <Fab
        variant="extended"
        sx={{
          position: 'fixed', right: 32, bottom: 32,
          height: 56, borderRadius: '16px',
          bgcolor: md.primaryContainer, color: md.onPrimaryContainer,
          boxShadow: '0 4px 8px 3px rgba(0,0,0,.15),0 1px 3px rgba(0,0,0,.30)',
          '&:hover': { bgcolor: md.primaryContainer },
        }}
        onClick={() => setSnackMsg(t('demo:snack.create_dialog'))}
      >
        <AddIcon sx={{ mr: 1.5 }} />
        {t('demo:fab.create_node')}
      </Fab>

      <Snackbar
        open={!!snackMsg}
        autoHideDuration={2400}
        onClose={() => setSnackMsg(null)}
        message={snackMsg}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
    </Box>
  )
}

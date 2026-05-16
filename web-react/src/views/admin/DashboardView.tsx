import { lazy, Suspense, useEffect, useState } from 'react'
import {
  Box,
  Card,
  CardContent,
  CircularProgress,
  Link as MuiLink,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  useTheme,
} from '@mui/material'
import { Link as RouterLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import GroupIcon from '@mui/icons-material/Group'
import DnsIcon from '@mui/icons-material/Dns'
import WorkspacesIcon from '@mui/icons-material/Workspaces'

import { listUsers } from '@/api/users'
import { listNodes } from '@/api/nodes'
import { listGroups } from '@/api/groups'
import { topTraffic, trafficHistory, type TrafficHistoryItem, type TrafficRow } from '@/api/traffic'
import type { M3Tokens } from '@/theme'

const TrafficChart = lazy(() => import('@/components/TrafficChart'))

function formatBytes(n: number): string {
  if (n === 0) return '0'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v.toFixed(2)} ${units[u]}`
}

interface MetricCardProps {
  labelKey: string
  value: number | string
  Icon: typeof DnsIcon
  tone: 'primary' | 'secondary' | 'tertiary'
  loading: boolean
}

function toneContainer(md: M3Tokens, tone: MetricCardProps['tone']) {
  switch (tone) {
    case 'secondary': return { bg: md.secondaryContainer, fg: md.onSecondaryContainer }
    case 'tertiary': return { bg: md.tertiaryContainer, fg: md.onTertiaryContainer }
    default: return { bg: md.primaryContainer, fg: md.onPrimaryContainer }
  }
}

function MetricCard({ labelKey, value, Icon, tone, loading }: MetricCardProps) {
  const theme = useTheme()
  const md = theme.palette.md
  const t = toneContainer(md, tone)
  const { t: tr } = useTranslation('admin')
  return (
    <Card sx={{ bgcolor: md.surfaceContainerHighest }}>
      <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2, p: 2.5, '&:last-child': { pb: 2.5 } }}>
        <Box sx={{
          width: 56, height: 56, borderRadius: '12px',
          display: 'grid', placeItems: 'center', flexShrink: 0,
          bgcolor: t.bg, color: t.fg,
        }}>
          <Icon sx={{ fontSize: 28 }} />
        </Box>
        <Box>
          <Typography sx={{ fontSize: 13, color: md.onSurfaceVariant }}>{tr(labelKey)}</Typography>
          {loading
            ? <CircularProgress size={20} sx={{ mt: 0.5 }} />
            : <Typography sx={{ fontSize: 32, fontWeight: 500, lineHeight: 1.1, mt: 0.25 }}>{value}</Typography>}
        </Box>
      </CardContent>
    </Card>
  )
}

export default function DashboardView() {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation('admin')

  const [loading, setLoading] = useState(true)
  const [userCount, setUserCount] = useState(0)
  const [nodeCount, setNodeCount] = useState(0)
  const [groupCount, setGroupCount] = useState(0)
  const [topUsers, setTopUsers] = useState<TrafficRow[]>([])
  const [trend, setTrend] = useState<TrafficHistoryItem[]>([])
  const [trendLoading, setTrendLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [u, n, g, top] = await Promise.all([
          listUsers({ page: 1, page_size: 1 }),
          listNodes(),
          listGroups(),
          topTraffic(5).catch(() => []),
        ])
        if (cancelled) return
        setUserCount(u.total)
        setNodeCount(n.length)
        setGroupCount(g.items.length)
        setTopUsers(top)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    async function loadTrend() {
      try {
        const since = new Date()
        since.setHours(0, 0, 0, 0)
        since.setDate(since.getDate() - 6)
        const sStr = `${since.getFullYear()}-${String(since.getMonth() + 1).padStart(2, '0')}-${String(since.getDate()).padStart(2, '0')}`
        const today = new Date()
        const tStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
        const res = await trafficHistory({ period: 'day', since: sStr, until: tStr })
        if (!cancelled) setTrend(res.items)
      } catch { /* ignore */ }
      finally { if (!cancelled) setTrendLoading(false) }
    }
    void load()
    void loadTrend()
    return () => { cancelled = true }
  }, [])

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" sx={{ mb: 3 }}>{t('dashboard.title')}</Typography>

      <Box sx={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: 2,
        mb: 3,
      }}>
        <MetricCard labelKey="dashboard.metric.users" value={userCount} Icon={GroupIcon} tone="primary" loading={loading} />
        <MetricCard labelKey="dashboard.metric.nodes" value={nodeCount} Icon={DnsIcon} tone="tertiary" loading={loading} />
        <MetricCard labelKey="dashboard.metric.groups" value={groupCount} Icon={WorkspacesIcon} tone="secondary" loading={loading} />
      </Box>

      {/* 7-day traffic trend */}
      <Card sx={{ mb: 3, p: 2.5, bgcolor: md.surfaceContainerLow, boxShadow: '0 1px 2px rgba(0,0,0,.3),0 1px 3px 1px rgba(0,0,0,.15)' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1.5 }}>
          <Typography variant="h6">{t('dashboard.trend_title', { defaultValue: '近 7 天流量趋势' })}</Typography>
          <MuiLink component={RouterLink} to="/admin/traffic" underline="hover" sx={{ fontSize: 13, color: md.primary }}>
            {t('dashboard.traffic.view_all')} →
          </MuiLink>
        </Box>
        <Suspense fallback={<Box sx={{ height: 280, display: 'grid', placeItems: 'center' }}><CircularProgress size={24} /></Box>}>
          {trendLoading
            ? <Box sx={{ height: 280, display: 'grid', placeItems: 'center' }}><CircularProgress size={24} /></Box>
            : <TrafficChart items={trend} height={280} />}
        </Suspense>
      </Card>

      <Card sx={{ bgcolor: md.surfaceContainerLow, boxShadow: '0 1px 2px rgba(0,0,0,.3),0 1px 3px 1px rgba(0,0,0,.15)' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 2.5, pt: 2.5, pb: 1.5 }}>
          <Typography variant="h6">{t('dashboard.traffic.title')}</Typography>
          <MuiLink component={RouterLink} to="/admin/traffic" underline="hover" sx={{ fontSize: 13, color: md.primary }}>
            {t('dashboard.traffic.view_all')} →
          </MuiLink>
        </Box>

        {loading ? (
          <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}>
            <CircularProgress size={24} />
          </Box>
        ) : topUsers.length === 0 ? (
          <Box sx={{ py: 6, textAlign: 'center', color: md.onSurfaceVariant, fontSize: 14 }}>
            {t('dashboard.traffic.empty')}
          </Box>
        ) : (
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow sx={{ '& th': { color: md.onSurfaceVariant, fontWeight: 500, fontSize: 12, textTransform: 'uppercase', letterSpacing: '.5px', borderBottom: `1px solid ${md.outlineVariant}` } }}>
                  <TableCell sx={{ width: 60 }}>{t('dashboard.traffic.table.rank')}</TableCell>
                  <TableCell>{t('dashboard.traffic.table.upn')}</TableCell>
                  <TableCell align="right">{t('dashboard.traffic.table.period_used')}</TableCell>
                  <TableCell align="right">{t('dashboard.traffic.table.today_used')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {topUsers.map((r, i) => (
                  <TableRow key={r.user_id} hover sx={{ '& td': { borderBottom: `1px solid ${md.outlineVariant}` } }}>
                    <TableCell sx={{ color: md.onSurfaceVariant, fontVariantNumeric: 'tabular-nums' }}>{i + 1}</TableCell>
                    <TableCell sx={{ fontWeight: 500 }}>{r.upn}</TableCell>
                    <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums', fontSize: 13 }}>
                      {formatBytes(r.period_used_bytes)}
                    </TableCell>
                    <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums', fontSize: 13 }}>
                      {formatBytes(r.today_used_bytes)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>
    </Box>
  )
}

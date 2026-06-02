import { useEffect, useState } from 'react'
import { Box, CircularProgress, Typography, useTheme } from '@mui/material'
import { useTranslation } from 'react-i18next'

import { getUserNodeUsage, type UserNodeUsageRow } from '@/api/traffic'

// fmt renders a byte count compactly (single-letter unit) so three metric
// columns plus up/down sublines fit the 300px read-only column.
function fmt(n: number): string {
  if (!n) return '0'
  const units = ['B', 'K', 'M', 'G', 'T']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++ }
  return `${v >= 100 || u === 0 ? Math.round(v) : v.toFixed(1)}${units[u]}`
}

/** UserNodeUsage shows one user's usage broken down per node (lifetime /
 *  current-period / today, each split ↑up ↓down) inside the admin user-edit
 *  dialog's left read-only column. Read-only; fetched on mount. The footer
 *  total is the sum of the rows — in the normal case it equals the user's own
 *  period/lifetime figures shown above it. */
export function UserNodeUsage({ userId }: { userId: number }) {
  const { t } = useTranslation('admin')
  const md = useTheme().palette.md
  const [rows, setRows] = useState<UserNodeUsageRow[] | null>(null)

  useEffect(() => {
    let alive = true
    setRows(null)
    getUserNodeUsage(userId)
      .then(r => { if (alive) setRows(r) })
      .catch(() => { if (alive) setRows([]) })
    return () => { alive = false }
  }, [userId])

  const title = (
    <Typography sx={{ fontSize: 13, fontWeight: 600, color: md.onSurfaceVariant, mb: 0.75 }}>
      {t('users.nodeusage.title', { defaultValue: '按节点用量' })}
    </Typography>
  )

  if (rows === null) {
    return <Box sx={{ mt: 1 }}>{title}<CircularProgress size={18} /></Box>
  }
  if (rows.length === 0) {
    return (
      <Box sx={{ mt: 1 }}>
        {title}
        <Typography sx={{ fontSize: 12, color: md.onSurfaceVariant }}>
          {t('users.nodeusage.empty', { defaultValue: '该用户暂无节点' })}
        </Typography>
      </Box>
    )
  }

  const sum = rows.reduce((a, r) => ({
    lifetime: a.lifetime + r.lifetime_total_bytes,
    lifeUp: a.lifeUp + r.lifetime_up_bytes,
    lifeDown: a.lifeDown + r.lifetime_down_bytes,
    period: a.period + r.period_total_bytes,
    periodUp: a.periodUp + r.period_up_bytes,
    periodDown: a.periodDown + r.period_down_bytes,
    today: a.today + r.today_total_bytes,
    todayUp: a.todayUp + r.today_up_bytes,
    todayDown: a.todayDown + r.today_down_bytes,
  }), { lifetime: 0, lifeUp: 0, lifeDown: 0, period: 0, periodUp: 0, periodDown: 0, today: 0, todayUp: 0, todayDown: 0 })

  const grid = { display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 1fr 1fr 1fr', columnGap: 0.5, alignItems: 'baseline' } as const

  // One metric cell: prominent total over a small ↑up ↓down subline.
  const cell = (total: number, up: number, down: number) => (
    <Box sx={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
      <Box sx={{ fontSize: 12, fontWeight: 500 }}>{fmt(total)}</Box>
      <Box sx={{ fontSize: 10, color: md.onSurfaceVariant, whiteSpace: 'nowrap' }}>↑{fmt(up)} ↓{fmt(down)}</Box>
    </Box>
  )

  return (
    <Box sx={{ mt: 1 }}>
      {title}
      {/* header */}
      <Box sx={{ ...grid, mb: 0.25, fontSize: 10, color: md.onSurfaceVariant, textTransform: 'uppercase', letterSpacing: '.3px' }}>
        <span />
        <Box sx={{ textAlign: 'right' }}>{t('users.nodeusage.lifetime', { defaultValue: '累计' })}</Box>
        <Box sx={{ textAlign: 'right' }}>{t('users.nodeusage.period', { defaultValue: '本周期' })}</Box>
        <Box sx={{ textAlign: 'right' }}>{t('users.nodeusage.today', { defaultValue: '今日' })}</Box>
      </Box>
      {rows.map((r, i) => (
        <Box key={r.node_id || r.client_email || i}
          sx={{ ...grid, py: 0.5, borderTop: `1px solid ${md.outlineVariant}` }}>
          <Box sx={{ minWidth: 0, pr: 0.5 }}>
            <Typography noWrap sx={{ fontSize: 12, fontWeight: 500 }}>
              {r.display_name || r.client_email || `#${r.node_id}`}
            </Typography>
            {r.region && <Typography noWrap sx={{ fontSize: 10, color: md.onSurfaceVariant }}>{r.region}</Typography>}
          </Box>
          {cell(r.lifetime_total_bytes, r.lifetime_up_bytes, r.lifetime_down_bytes)}
          {cell(r.period_total_bytes, r.period_up_bytes, r.period_down_bytes)}
          {cell(r.today_total_bytes, r.today_up_bytes, r.today_down_bytes)}
        </Box>
      ))}
      {/* footer total */}
      {rows.length > 1 && (
        <Box sx={{ ...grid, py: 0.5, borderTop: `2px solid ${md.outlineVariant}`, fontWeight: 600 }}>
          <Box sx={{ fontSize: 12 }}>{t('users.nodeusage.total', { defaultValue: '合计' })}</Box>
          {cell(sum.lifetime, sum.lifeUp, sum.lifeDown)}
          {cell(sum.period, sum.periodUp, sum.periodDown)}
          {cell(sum.today, sum.todayUp, sum.todayDown)}
        </Box>
      )}
    </Box>
  )
}

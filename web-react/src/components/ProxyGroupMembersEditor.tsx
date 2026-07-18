import { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward'
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import DragIndicatorIcon from '@mui/icons-material/DragIndicator'
import RestartAltIcon from '@mui/icons-material/RestartAlt'
import { useTranslation } from 'react-i18next'

import {
  inspectProxyGroups,
  type ProxyGroupInspection,
  type ProxyGroupMember,
} from '@/api/rules'
import type { Group } from '@/api/types'
import { appendUniqueProxyGroupMember, proxyGroupMemberIdentity, proxyGroupMemberListsEqual, reorderProxyGroupMembers } from '@/utils/proxyGroupMembers'

type MemberMap = Record<string, ProxyGroupMember[]>

interface Props {
  content: string
  members: MemberMap
  initialMembers: MemberMap
  onChange: (members: MemberMap) => void
  previewGroups: Group[]
  onValidationChange?: (hasErrors: boolean) => void
}

type AddKind = 'node' | 'builtin' | 'proxy_group' | 'region' | 'tag' | 'remaining'
type AddOption = { value: string | number; label: string }

export default function ProxyGroupMembersEditor({ content, members, initialMembers, onChange, previewGroups, onValidationChange }: Props) {
  const theme = useTheme()
  const md = theme.palette.md
  const { t } = useTranslation(['admin', 'common'])
  const [inspection, setInspection] = useState<ProxyGroupInspection | null>(null)
  const [selected, setSelected] = useState('')
  const [previewGroupID, setPreviewGroupID] = useState(0)
  const [loading, setLoading] = useState(false)
  const [addKind, setAddKind] = useState<AddKind>('node')
  const [addValue, setAddValue] = useState<string | number | null>(null)
  const [dragIndex, setDragIndex] = useState<number | null>(null)

  const serializedMembers = useMemo(() => JSON.stringify(members), [members])
  useEffect(() => {
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      setLoading(true)
      void inspectProxyGroups({
        content,
        proxy_group_members: members,
        preview_group_id: previewGroupID || undefined,
      }, controller.signal).then(result => {
        setInspection(result)
        const hasErrors = result.issues.some(issue => issue.level === 'error')
        onValidationChange?.(hasErrors)
        if (!selected || !result.groups.some(group => group.name === selected)) {
          setSelected(result.groups[0]?.name || '')
        }
      }).catch(error => {
        if (error?.code !== 'ERR_CANCELED') onValidationChange?.(true)
      }).finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    }, 300)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [content, serializedMembers, previewGroupID]) // eslint-disable-line react-hooks/exhaustive-deps

  const current = inspection?.groups.find(group => group.name === selected)
  const currentConfigured = Object.prototype.hasOwnProperty.call(members, selected)
  const currentMembers = members[selected] || current?.default_members || []
  const currentIssues = (inspection?.issues || []).filter(issue => !issue.group || issue.group === selected)

  function commit(next: ProxyGroupMember[]) {
    if (!selected) return
    onChange({ ...members, [selected]: next })
  }

  function resetCurrent() {
    if (!selected) return
    const next = { ...members }
    delete next[selected]
    onChange(next)
  }

  function move(from: number, to: number) {
    if (to < 0 || to >= currentMembers.length || from === to) return
    commit(reorderProxyGroupMembers(currentMembers, from, to))
  }

  function remove(index: number) {
    commit(currentMembers.filter((_, i) => i !== index))
  }

  function memberKey(member: ProxyGroupMember) {
    return proxyGroupMemberIdentity(member)
  }

  function memberLabel(member: ProxyGroupMember) {
    if (member.kind === 'node') {
      const node = inspection?.nodes.find(n => n.id === member.node_id)
      return node ? node.display_name : t('admin:rules.members.missing_node', { id: member.node_id })
    }
    if (member.kind === 'node_set') {
      if (member.value === 'remaining') return t('admin:rules.members.remaining')
      if (member.value?.startsWith('region:')) return t('admin:rules.members.region_set', { value: member.value.slice(7) })
      if (member.value?.startsWith('tag:')) return t('admin:rules.members.tag_set', { value: member.value.slice(4) })
    }
    return member.value || member.kind
  }

  function issueLabel(issue: NonNullable<ProxyGroupInspection['issues']>[number]) {
    if (!issue.code) return issue.message
    return t(`admin:rules.members.issues.${issue.code}`, { ...issue.params, defaultValue: issue.message })
  }

  const addOptions = useMemo<AddOption[]>(() => {
    switch (addKind) {
      case 'node': return (inspection?.nodes || []).map(node => ({ value: node.id, label: `${node.display_name} · ${node.server_address || '—'} · ${node.region || '—'}${node.enabled ? '' : ` · ${t('admin:rules.members.disabled')}`}` }))
      case 'builtin': return (inspection?.builtins || []).map(value => ({ value, label: value }))
      case 'proxy_group': return (inspection?.groups || []).filter(group => group.name !== selected).map(group => ({ value: group.name, label: group.name }))
      case 'region': return (inspection?.regions || []).map(value => ({ value, label: value }))
      case 'tag': return (inspection?.tags || []).map(value => ({ value, label: value }))
      case 'remaining': return [{ value: 'remaining', label: t('admin:rules.members.remaining') }]
    }
  }, [addKind, inspection, selected, t])

  function addMember() {
    let member: ProxyGroupMember | null = null
    if (addKind === 'remaining') member = { kind: 'node_set', value: 'remaining' }
    else if (addValue !== null) {
      if (addKind === 'node') member = { kind: 'node', node_id: Number(addValue) }
      else if (addKind === 'region') member = { kind: 'node_set', value: `region:${addValue}` }
      else if (addKind === 'tag') member = { kind: 'node_set', value: `tag:${addValue}` }
      else member = { kind: addKind, value: String(addValue) }
    }
    if (!member) return
    const next = appendUniqueProxyGroupMember(currentMembers, member)
    if (next === currentMembers) return
    commit(next)
    setAddValue(null)
  }

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '280px minmax(0, 1fr)' }, height: { xs: 'auto', md: 560 }, minHeight: 560, border: `1px solid ${md.outlineVariant}`, borderRadius: 2, overflow: 'hidden' }}>
      <Box sx={{ borderRight: { md: `1px solid ${md.outlineVariant}` }, bgcolor: md.surfaceContainerLow, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <Box sx={{ p: 2, borderBottom: `1px solid ${md.outlineVariant}` }}>
          <Typography sx={{ fontWeight: 600 }}>{t('admin:rules.members.groups_title')}</Typography>
          <Typography variant="caption" color="text.secondary">{t('admin:rules.members.groups_hint')}</Typography>
        </Box>
        {loading && !inspection ? <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress size={22} /></Box> : (
          <List dense disablePadding sx={{ overflowY: 'auto', minHeight: 0 }}>
            {(inspection?.groups || []).map(group => (
              <ListItemButton key={group.name} selected={group.name === selected} onClick={() => setSelected(group.name)} sx={{ borderBottom: `1px solid ${md.outlineVariant}` }}>
                <ListItemText primary={group.name} secondary={group.members[0] ? memberLabel(group.members[0]) : '—'} />
                {!proxyGroupMemberListsEqual(members[group.name], initialMembers[group.name]) && (
                  <Tooltip title={t('admin:rules.members.unsaved')}>
                    <Box component="span" sx={{ width: 10, height: 10, flexShrink: 0, borderRadius: '50%', bgcolor: md.primary }} />
                  </Tooltip>
                )}
              </ListItemButton>
            ))}
          </List>
        )}
      </Box>

      <Box sx={{ p: 2.5, minWidth: 0, overflowY: 'auto' }}>
        {!current ? (
          <Box sx={{ py: 10, textAlign: 'center', color: md.onSurfaceVariant }}>{t('admin:rules.members.no_groups')}</Box>
        ) : <>
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ justifyContent: 'space-between', gap: 1, alignItems: { sm: 'center' } }}>
            <Box>
              <Typography variant="h6">{current.name}</Typography>
              <Typography variant="caption" color="text.secondary">{t('admin:rules.members.first_default')}</Typography>
            </Box>
            <Button size="small" startIcon={<RestartAltIcon />} onClick={resetCurrent} disabled={!currentConfigured}>
              {t('admin:rules.members.restore_default')}
            </Button>
          </Stack>

          <Stack spacing={1} sx={{ mt: 2 }}>
            {currentMembers.map((member, index) => (
              <Box key={`${memberKey(member)}:${index}`} draggable
                onDragStart={() => setDragIndex(index)} onDragEnd={() => setDragIndex(null)}
                onDragOver={e => e.preventDefault()} onDrop={() => { if (dragIndex !== null) move(dragIndex, index); setDragIndex(null) }}
                sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 1, border: `1px solid ${md.outlineVariant}`, borderRadius: 1.5, bgcolor: index === 0 ? md.primaryContainer : md.surfaceContainer, opacity: dragIndex === index ? 0.5 : 1 }}>
                <DragIndicatorIcon fontSize="small" sx={{ cursor: 'grab', color: md.onSurfaceVariant }} />
                <Chip size="small" label={index + 1} color={index === 0 ? 'primary' : 'default'} />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography noWrap sx={{ fontSize: 14, fontWeight: 500 }}>{memberLabel(member)}</Typography>
                  <Typography noWrap variant="caption" color="text.secondary">{memberKey(member)}</Typography>
                </Box>
                <Tooltip title={t('admin:rules.members.move_up')}><span><IconButton size="small" disabled={index === 0} onClick={() => move(index, index - 1)}><ArrowUpwardIcon fontSize="small" /></IconButton></span></Tooltip>
                <Tooltip title={t('admin:rules.members.move_down')}><span><IconButton size="small" disabled={index === currentMembers.length - 1} onClick={() => move(index, index + 1)}><ArrowDownwardIcon fontSize="small" /></IconButton></span></Tooltip>
                <IconButton size="small" color="error" onClick={() => remove(index)}><DeleteIcon fontSize="small" /></IconButton>
              </Box>
            ))}
          </Stack>

          <Box sx={{ mt: 2, p: 1.5, borderRadius: 2, bgcolor: md.surfaceContainerLow }}>
            <Typography sx={{ fontSize: 13, fontWeight: 600, mb: 1 }}>{t('admin:rules.members.add_title')}</Typography>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
              <TextField select size="small" value={addKind} onChange={e => { setAddKind(e.target.value as AddKind); setAddValue(null) }} sx={{ minWidth: 150 }}>
                <MenuItem value="node">{t('admin:rules.members.kind_node')}</MenuItem>
                <MenuItem value="remaining">{t('admin:rules.members.kind_remaining')}</MenuItem>
                <MenuItem value="region">{t('admin:rules.members.kind_region')}</MenuItem>
                <MenuItem value="tag">{t('admin:rules.members.kind_tag')}</MenuItem>
                <MenuItem value="builtin">{t('admin:rules.members.kind_builtin')}</MenuItem>
                <MenuItem value="proxy_group">{t('admin:rules.members.kind_group')}</MenuItem>
              </TextField>
              <Autocomplete<AddOption> size="small" options={addOptions} value={addOptions.find(option => option.value === addValue) || null}
                onChange={(_, option) => setAddValue(option?.value ?? null)} getOptionLabel={option => option.label}
                sx={{ flex: 1, minWidth: 180 }} renderInput={params => <TextField {...params} placeholder={t('admin:rules.members.search_placeholder')} />} />
              <Button variant="outlined" startIcon={<AddIcon />} onClick={addMember} disabled={addKind !== 'remaining' && addValue === null}>{t('admin:rules.members.add')}</Button>
            </Stack>
          </Box>

          {currentIssues.map((issue, index) => <Alert key={`${issue.message}:${index}`} severity={issue.level} sx={{ mt: 1.5 }}>{issueLabel(issue)}</Alert>)}
          <Divider sx={{ my: 2 }} />
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ justifyContent: 'space-between', gap: 1, alignItems: { sm: 'center' } }}>
            <Typography sx={{ fontWeight: 600 }}>{t('admin:rules.members.preview_title')}</Typography>
            <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
              <Typography variant="body2" sx={{ flexShrink: 0 }}>{t('admin:rules.members.preview_group_label')}</Typography>
              <TextField select size="small" value={previewGroupID} onChange={e => setPreviewGroupID(Number(e.target.value))} sx={{ minWidth: 220 }}>
                <MenuItem value={0}>{t('admin:rules.members.preview_all')}</MenuItem>
                {previewGroups.map(group => <MenuItem key={group.id} value={group.id}>{group.name}</MenuItem>)}
              </TextField>
            </Stack>
          </Stack>
          <Box sx={{ mt: 1, display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
            {current.preview.map((name, index) => <Chip key={`${name}:${index}`} size="small" color={index === 0 ? 'primary' : 'default'} label={`${index + 1}. ${name}`} />)}
            {!current.preview.length && <Typography variant="caption" color="error">{t('admin:rules.members.preview_empty')}</Typography>}
          </Box>
        </>}
      </Box>
    </Box>
  )
}

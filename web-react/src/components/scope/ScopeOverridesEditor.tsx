// ScopeOverridesEditor renders the per-field inherit/override controls for one
// scope (a group). Pure controlled component: `scope` in, `onChange` out — it
// performs no I/O. Used by the group detail Policies tab and the Settings scope
// rail. Restrict to a subset of categories via `categories`; default = all.
import { Box, FormControlLabel, Switch, TextField, Typography } from '@mui/material'
import { useTranslation } from 'react-i18next'

import { SCOPE_CATEGORIES, SCOPE_KEYS, fmtScope, type ScopeState } from './scopeOverrides'

export default function ScopeOverridesEditor({
  scope,
  onChange,
  categories,
  hideCategoryCaptions,
}: {
  scope: ScopeState
  onChange: (next: ScopeState) => void
  /** Restrict to these category ids; omit for all categories. */
  categories?: string[]
  /** Suppress the per-category caption (e.g. when the surrounding tab already
   *  names the single category being shown). */
  hideCategoryCaptions?: boolean
}) {
  const { t } = useTranslation(['admin', 'common'])
  const cats = categories
    ? SCOPE_CATEGORIES.filter(c => categories.includes(c.id))
    : SCOPE_CATEGORIES

  return (
    <>
      {cats.map(cat => {
        const keys = SCOPE_KEYS.filter(k => k.cat === cat.id && scope.overridable.includes(k.key))
        if (!keys.length) return null
        return (
          <Box key={cat.id} sx={{ mt: 1 }}>
            {!hideCategoryCaptions && (
              <Typography variant="caption" sx={{ fontWeight: 600, color: 'text.secondary' }}>
                {t(`admin:groups.scope.${cat.labelKey}`, { defaultValue: cat.def })}
              </Typography>
            )}
            {keys.map(k => {
              const st = scope.edit[k.key]
              const setEdit = (v: { on: boolean; value: string }) =>
                onChange({ ...scope, edit: { ...scope.edit, [k.key]: v } })
              return (
                <Box key={k.key} sx={{ display: 'flex', alignItems: 'center', gap: 1, minHeight: 40 }}>
                  <Box sx={{ flex: 1, fontSize: 14 }}>
                    {t(`admin:groups.scope.${k.labelKey}`, { defaultValue: k.def })}
                  </Box>
                  <FormControlLabel
                    sx={{ mr: 0 }}
                    control={
                      <Switch size="small" checked={st.on}
                        onChange={(_, c) => setEdit({ on: c, value: c ? st.value : scope.global[k.key] })} />
                    }
                    label={
                      <Typography variant="caption">
                        {st.on
                          ? t('admin:groups.scope.override', { defaultValue: '覆盖' })
                          : t('admin:groups.scope.inherit', { defaultValue: '继承' })}
                      </Typography>
                    }
                  />
                  {st.on ? (
                    k.kind === 'bool' ? (
                      <Switch size="small" checked={st.value === '1'}
                        onChange={(_, c) => setEdit({ on: true, value: c ? '1' : '0' })} />
                    ) : k.kind === 'str' ? (
                      <TextField size="small" value={st.value}
                        onChange={e => setEdit({ on: true, value: e.target.value })} sx={{ width: 180 }} />
                    ) : (
                      <TextField size="small" type="number" value={st.value}
                        inputProps={k.kind === 'float' ? { step: 'any', min: 0 } : { step: 1, min: 0 }}
                        onChange={e => setEdit({ on: true, value: e.target.value })} sx={{ width: 96 }} />
                    )
                  ) : (
                    <Typography variant="caption" color="text.secondary" sx={{ minWidth: 96, textAlign: 'right' }}>
                      {t('admin:groups.scope.global_prefix', { defaultValue: '全局' })}: {fmtScope(k.kind, scope.global[k.key])}
                    </Typography>
                  )}
                </Box>
              )
            })}
          </Box>
        )
      })}
    </>
  )
}

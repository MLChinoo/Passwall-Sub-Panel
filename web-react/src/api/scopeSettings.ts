import { client } from './client'

// Per-group setting overrides (v3.8.0). Overrides are sparse: a key present in
// `overrides` is set for this group; absent = inherit the global value. Values
// are the raw KV strings the backend decodes via each setting's descriptor
// ("1"/"0" for bools, a number string for ints).
export interface ScopeSettings {
  scope_type: string
  scope_id: number
  /** "type.name" keys this scope is allowed to override (drives the editor). */
  overridable: string[]
  /** Sparse "type.name" -> raw value for the keys this group overrides. */
  overrides: Record<string, string>
}

export async function getGroupScopeSettings(groupId: number, signal?: AbortSignal) {
  const { data } = await client.get<ScopeSettings>(`/admin/groups/${groupId}/scope-settings`, { signal })
  return data
}

/** Set (upsert) one override. `value` is the raw KV string ("1"/"0" / a number). */
export async function setGroupScopeOverride(groupId: number, type: string, name: string, value: string) {
  await client.put(`/admin/groups/${groupId}/scope-settings`, { type, name, value })
}

/** Delete one override = restore inheritance from the global value. */
export async function deleteGroupScopeOverride(groupId: number, type: string, name: string) {
  await client.delete(`/admin/groups/${groupId}/scope-settings/${type}/${name}`)
}

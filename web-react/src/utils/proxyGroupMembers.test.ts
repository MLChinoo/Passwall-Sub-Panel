import { describe, expect, it } from 'vitest'

import type { ProxyGroupMember } from '@/api/rules'
import { appendUniqueProxyGroupMember, proxyGroupMemberIdentity, proxyGroupMemberListsEqual, reorderProxyGroupMembers } from './proxyGroupMembers'

describe('proxy group member editor helpers', () => {
  const node: ProxyGroupMember = { kind: 'node', node_id: 42 }
  const direct: ProxyGroupMember = { kind: 'builtin', value: 'DIRECT' }
  const remaining: ProxyGroupMember = { kind: 'node_set', value: 'remaining' }

  it('moves a concrete node before DIRECT without mutating the source', () => {
    const source = [direct, node, remaining]
    expect(reorderProxyGroupMembers(source, 1, 0)).toEqual([node, direct, remaining])
    expect(source).toEqual([direct, node, remaining])
  })

  it('does not append a duplicate typed reference', () => {
    const source = [node]
    expect(appendUniqueProxyGroupMember(source, { kind: 'node', node_id: 42 })).toBe(source)
  })

  it('keeps node IDs and selector values distinct in identities', () => {
    expect(proxyGroupMemberIdentity(node)).toBe('node::42')
    expect(proxyGroupMemberIdentity(remaining)).toBe('node_set:remaining:0')
  })

  it('detects unsaved member changes including order and restoring defaults', () => {
    expect(proxyGroupMemberListsEqual([direct, node], [{ ...direct }, { ...node }])).toBe(true)
    expect(proxyGroupMemberListsEqual([node, direct], [direct, node])).toBe(false)
    expect(proxyGroupMemberListsEqual(undefined, undefined)).toBe(true)
    expect(proxyGroupMemberListsEqual(undefined, [])).toBe(false)
  })
})

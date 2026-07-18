import type { ProxyGroupMember } from '@/api/rules'

export function proxyGroupMemberIdentity(member: ProxyGroupMember): string {
  return `${member.kind}:${member.value || ''}:${member.node_id || 0}`
}

export function reorderProxyGroupMembers(members: ProxyGroupMember[], from: number, to: number): ProxyGroupMember[] {
  if (from === to || from < 0 || to < 0 || from >= members.length || to >= members.length) return members
  const next = [...members]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

export function appendUniqueProxyGroupMember(members: ProxyGroupMember[], member: ProxyGroupMember): ProxyGroupMember[] {
  const identity = proxyGroupMemberIdentity(member)
  if (members.some(existing => proxyGroupMemberIdentity(existing) === identity)) return members
  return [...members, member]
}

export function proxyGroupMemberListsEqual(left?: ProxyGroupMember[], right?: ProxyGroupMember[]): boolean {
  if (left === undefined || right === undefined) return left === right
  return left.length === right.length && left.every((member, index) => proxyGroupMemberIdentity(member) === proxyGroupMemberIdentity(right[index]))
}

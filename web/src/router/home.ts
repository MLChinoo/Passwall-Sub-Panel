import type { Role } from '@/api/types'

export function homeForRole(role: Role | '') {
  return role === 'admin' ? '/admin/dashboard' : '/user/me'
}

export function isAdminPath(path: string) {
  return path === '/admin' || path.startsWith('/admin/')
}

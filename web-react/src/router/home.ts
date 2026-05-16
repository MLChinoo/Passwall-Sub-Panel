import type { Role } from '@/api/types'

export function homeForRole(role: Role | ''): string {
  return role === 'admin' ? '/admin/dashboard' : '/user/me'
}

export function isAdminPath(path: string): boolean {
  return path === '/admin' || path.startsWith('/admin/')
}

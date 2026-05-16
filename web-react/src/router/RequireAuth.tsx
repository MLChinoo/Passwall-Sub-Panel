import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { selectIsAdmin, selectIsLoggedIn, useAuthStore } from '@/stores/auth'

interface Props {
  adminOnly?: boolean
}

export default function RequireAuth({ adminOnly }: Props) {
  const location = useLocation()
  const role = useAuthStore(s => s.role)

  if (!selectIsLoggedIn()) {
    return <Navigate to="/login" state={{ returnTo: location.pathname + location.search }} replace />
  }
  if (adminOnly && !selectIsAdmin({ role } as never)) {
    return <Navigate to="/user/me" replace />
  }
  return <Outlet />
}

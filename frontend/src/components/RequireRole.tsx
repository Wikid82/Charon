import React from 'react'
import { Navigate } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'

interface RequireRoleProps {
  allowed: Array<'admin' | 'user' | 'passthrough'>
  children: React.ReactNode
}

const RequireRole: React.FC<RequireRoleProps> = ({ allowed, children }) => {
  const { user } = useAuth()

  if (!user) {
    return <Navigate to="/login" replace />
  }

  if (!allowed.includes(user.role)) {
    const redirectTarget = user.role === 'passthrough' ? '/passthrough' : '/'
    return <Navigate to={redirectTarget} replace />
  }

  return children
}

export default RequireRole

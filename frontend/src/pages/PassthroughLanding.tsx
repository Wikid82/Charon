import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../hooks/useAuth'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Shield, LogOut } from 'lucide-react'

export default function PassthroughLanding() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const headingRef = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    headingRef.current?.focus()
  }, [])

  return (
    <div className="min-h-screen bg-light-bg dark:bg-dark-bg flex items-center justify-center p-4">
      <main className="w-full max-w-md" aria-labelledby="passthrough-heading">
        <Card className="p-8 text-center space-y-6">
          <div className="flex justify-center">
            <div className="p-3 bg-blue-900/20 rounded-full">
              <Shield className="h-8 w-8 text-blue-400" aria-hidden="true" />
            </div>
          </div>

          <div className="space-y-2">
            <h1
              id="passthrough-heading"
              ref={headingRef}
              tabIndex={-1}
              className="text-2xl font-bold text-gray-900 dark:text-white outline-none"
            >
              {t('passthrough.title')}
            </h1>
            {user?.name && (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {user.name}
              </p>
            )}
          </div>

          <p className="text-content-secondary">
            {t('passthrough.description')}
          </p>

          <p className="text-sm text-content-muted">
            {t('passthrough.noAccessToManagement')}
          </p>

          <Button
            onClick={logout}
            variant="secondary"
            className="w-full"
          >
            <LogOut className="h-4 w-4 mr-2" aria-hidden="true" />
            {t('auth.logout')}
          </Button>
        </Card>
      </main>
    </div>
  )
}

import { Moon, Sun } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import { useTheme } from '../hooks/useTheme'
import { Button } from './ui/Button'

export function ThemeToggle() {
  const { resolvedTheme } = useTheme()
  const navigate = useNavigate()
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => navigate('/settings/appearance')}
      title="Theme settings"
      aria-label={`Current theme: ${resolvedTheme}. Open appearance settings.`}
    >
      {resolvedTheme === 'light' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}

export function LoadingSpinner({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-4 h-4 border-2',
    md: 'w-8 h-8 border-3',
    lg: 'w-12 h-12 border-4',
  }

  return (
    <div
      className={`${sizeClasses[size]} border-blue-600 border-t-transparent rounded-full animate-spin`}
      role="status"
      aria-label="Loading"
    />
  )
}

/**
 * CharonLoader - Boat on Waves animation (Charon ferrying across the Styx)
 * Used for general proxy/configuration operations
 */
export function CharonLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Loading">
      <svg viewBox="0 0 100 100" className="w-full h-full">
        {/* Water waves */}
        <path
          d="M0,60 Q10,55 20,60 T40,60 T60,60 T80,60 T100,60"
          fill="none"
          stroke="#3b82f6"
          strokeWidth="2"
          className="animate-pulse"
        />
        <path
          d="M0,65 Q10,60 20,65 T40,65 T60,65 T80,65 T100,65"
          fill="none"
          stroke="#60a5fa"
          strokeWidth="2"
          className="animate-pulse"
          style={{ animationDelay: '0.3s' }}
        />
        <path
          d="M0,70 Q10,65 20,70 T40,70 T60,70 T80,70 T100,70"
          fill="none"
          stroke="#93c5fd"
          strokeWidth="2"
          className="animate-pulse"
          style={{ animationDelay: '0.6s' }}
        />

        {/* Boat (bobbing animation) */}
        <g className="animate-bob-boat" style={{ transformOrigin: '50% 50%' }}>
          {/* Hull */}
          <path
            d="M30,45 L30,50 Q35,55 50,55 T70,50 L70,45 Z"
            fill="#1e293b"
            stroke="#334155"
            strokeWidth="1.5"
          />
          {/* Deck */}
          <rect x="32" y="42" width="36" height="3" fill="#475569" />
          {/* Mast */}
          <line x1="50" y1="42" x2="50" y2="25" stroke="#94a3b8" strokeWidth="2" />
          {/* Sail */}
          <path
            d="M50,25 L65,30 L50,40 Z"
            fill="#e0e7ff"
            stroke="#818cf8"
            strokeWidth="1"
            className="animate-pulse-glow"
          />
          {/* Charon silhouette */}
          <circle cx="45" cy="38" r="3" fill="#334155" />
          <rect x="44" y="41" width="2" height="4" fill="#334155" />
        </g>
      </svg>
    </div>
  )
}

/**
 * CharonCoinLoader - Spinning Obol Coin animation (Payment to the Ferryman)
 * Used for authentication/login operations
 */
export function CharonCoinLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Authenticating">
      <svg viewBox="0 0 100 100" className="w-full h-full">
        {/* Outer glow */}
        <circle
          cx="50"
          cy="50"
          r="45"
          fill="none"
          stroke="#f59e0b"
          strokeWidth="1"
          opacity="0.3"
          className="animate-pulse"
        />
        <circle
          cx="50"
          cy="50"
          r="40"
          fill="none"
          stroke="#fbbf24"
          strokeWidth="1"
          opacity="0.4"
          className="animate-pulse"
          style={{ animationDelay: '0.3s' }}
        />

        {/* Spinning coin */}
        <g className="animate-spin-y" style={{ transformOrigin: '50% 50%' }}>
          {/* Coin face */}
          <ellipse
            cx="50"
            cy="50"
            rx="30"
            ry="30"
            fill="url(#goldGradient)"
            stroke="#d97706"
            strokeWidth="2"
          />

          {/* Inner circle */}
          <ellipse
            cx="50"
            cy="50"
            rx="24"
            ry="24"
            fill="none"
            stroke="#92400e"
            strokeWidth="1.5"
          />

          {/* Charon's boat symbol (simplified) */}
          <path
            d="M35,50 L40,45 L60,45 L65,50 L60,52 L40,52 Z"
            fill="#78350f"
            opacity="0.8"
          />
          <line x1="50" y1="45" x2="50" y2="38" stroke="#78350f" strokeWidth="2" />
          <path d="M50,38 L58,42 L50,46 Z" fill="#78350f" opacity="0.6" />
        </g>

        {/* Gradient definition */}
        <defs>
          <radialGradient id="goldGradient">
            <stop offset="0%" stopColor="#fcd34d" />
            <stop offset="50%" stopColor="#f59e0b" />
            <stop offset="100%" stopColor="#d97706" />
          </radialGradient>
        </defs>
      </svg>
    </div>
  )
}

/**
 * CerberusLoader - Three-Headed Guardian animation
 * Used for security operations (WAF, CrowdSec, ACL, Rate Limiting)
 */
export function CerberusLoader({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-12 h-12',
    md: 'w-20 h-20',
    lg: 'w-28 h-28',
  }

  return (
    <div className={`${sizeClasses[size]} relative`} role="status" aria-label="Security Loading">
      <svg viewBox="0 0 100 100" className="w-full h-full">
        {/* Shield background */}
        <path
          d="M50,10 L80,25 L80,50 Q80,75 50,90 Q20,75 20,50 L20,25 Z"
          fill="#7f1d1d"
          stroke="#991b1b"
          strokeWidth="2"
          className="animate-pulse"
        />

        {/* Inner shield detail */}
        <path
          d="M50,15 L75,27 L75,50 Q75,72 50,85 Q25,72 25,50 L25,27 Z"
          fill="none"
          stroke="#dc2626"
          strokeWidth="1.5"
          opacity="0.6"
        />

        {/* Three heads (simplified circles with animation) */}
        {/* Left head */}
        <g className="animate-rotate-head" style={{ transformOrigin: '35% 45%' }}>
          <circle cx="35" cy="45" r="8" fill="#dc2626" stroke="#b91c1c" strokeWidth="1.5" />
          <circle cx="33" cy="43" r="1.5" fill="#fca5a5" />
          <circle cx="37" cy="43" r="1.5" fill="#fca5a5" />
          <path d="M32,48 Q35,50 38,48" stroke="#b91c1c" strokeWidth="1" fill="none" />
        </g>

        {/* Center head (larger) */}
        <g className="animate-pulse-glow">
          <circle cx="50" cy="42" r="10" fill="#dc2626" stroke="#b91c1c" strokeWidth="1.5" />
          <circle cx="47" cy="40" r="1.5" fill="#fca5a5" />
          <circle cx="53" cy="40" r="1.5" fill="#fca5a5" />
          <path d="M46,47 Q50,50 54,47" stroke="#b91c1c" strokeWidth="1.5" fill="none" />
        </g>

        {/* Right head */}
        <g className="animate-rotate-head" style={{ transformOrigin: '65% 45%', animationDelay: '0.5s' }}>
          <circle cx="65" cy="45" r="8" fill="#dc2626" stroke="#b91c1c" strokeWidth="1.5" />
          <circle cx="63" cy="43" r="1.5" fill="#fca5a5" />
          <circle cx="67" cy="43" r="1.5" fill="#fca5a5" />
          <path d="M62,48 Q65,50 68,48" stroke="#b91c1c" strokeWidth="1" fill="none" />
        </g>

        {/* Body */}
        <ellipse cx="50" cy="65" rx="18" ry="12" fill="#7f1d1d" stroke="#991b1b" strokeWidth="1.5" />

        {/* Paws */}
        <circle cx="40" cy="72" r="4" fill="#991b1b" />
        <circle cx="50" cy="72" r="4" fill="#991b1b" />
        <circle cx="60" cy="72" r="4" fill="#991b1b" />
      </svg>
    </div>
  )
}

/**
 * ConfigReloadOverlay - Full-screen blocking overlay for Caddy configuration reloads
 *
 * Displays thematic loading animation based on operation type:
 * - 'charon' (blue): Proxy hosts, certificates, general config operations
 * - 'coin' (gold): Authentication/login operations
 * - 'cerberus' (red): Security operations (WAF, CrowdSec, ACL, Rate Limiting)
 *
 * @param message - Primary message (e.g., "Ferrying new host...")
 * @param submessage - Secondary context (e.g., "Charon is crossing the Styx")
 * @param type - Theme variant: 'charon', 'coin', or 'cerberus'
 */
export function ConfigReloadOverlay({
  message = 'Ferrying configuration...',
  submessage = 'Charon is crossing the Styx',
  type = 'charon',
}: {
  message?: string
  submessage?: string
  type?: 'charon' | 'coin' | 'cerberus'
}) {
  const Loader =
    type === 'cerberus' ? CerberusLoader :
    type === 'coin' ? CharonCoinLoader :
    CharonLoader

  const bgColor =
    type === 'cerberus' ? 'bg-red-950/90' :
    type === 'coin' ? 'bg-amber-950/90' :
    'bg-blue-950/90'

  const borderColor =
    type === 'cerberus' ? 'border-red-900/50' :
    type === 'coin' ? 'border-amber-900/50' :
    'border-blue-900/50'

  return (
    <div className="fixed inset-0 bg-slate-900/70 backdrop-blur-sm flex items-center justify-center z-50">
      <div className={`${bgColor} ${borderColor} border-2 rounded-lg p-8 flex flex-col items-center gap-4 shadow-2xl max-w-md mx-4`}>
        <Loader size="lg" />
        <div className="text-center">
          <p className="text-slate-100 text-lg font-semibold mb-1">{message}</p>
          <p className="text-slate-300 text-sm">{submessage}</p>
        </div>
      </div>
    </div>
  )
}

export function LoadingOverlay({ message = 'Loading...' }: { message?: string }) {
  return (
    <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm flex items-center justify-center z-50">
      <div className="bg-slate-800 rounded-lg p-6 flex flex-col items-center gap-4 shadow-xl">
        <LoadingSpinner size="lg" />
        <p className="text-slate-300">{message}</p>
      </div>
    </div>
  )
}

export function LoadingCard() {
  return (
    <div className="bg-slate-800 rounded-lg p-6 animate-pulse">
      <div className="h-6 bg-slate-700 rounded w-1/3 mb-4"></div>
      <div className="space-y-3">
        <div className="h-4 bg-slate-700 rounded w-full"></div>
        <div className="h-4 bg-slate-700 rounded w-5/6"></div>
        <div className="h-4 bg-slate-700 rounded w-4/6"></div>
      </div>
    </div>
  )
}

export function EmptyState({
  icon = '📦',
  title,
  description,
  action,
}: {
  icon?: string
  title: string
  description: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
      <div className="text-6xl mb-4">{icon}</div>
      <h3 className="text-xl font-semibold text-slate-200 mb-2">{title}</h3>
      <p className="text-slate-400 mb-6 max-w-md">{description}</p>
      {action}
    </div>
  )
}

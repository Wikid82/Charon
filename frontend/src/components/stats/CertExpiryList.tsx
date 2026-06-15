import { ShieldCheck } from 'lucide-react'

import { cn } from '../../utils/cn'
import { Card, CardContent, CardHeader, CardTitle, Skeleton } from '../ui'

import type { CertExpiry } from '../../api/stats'

export interface CertExpiryListProps {
  data: CertExpiry[] | undefined
  isLoading: boolean
}

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(new Date(iso))
}

function urgencyClasses(daysLeft: number): { row: string; badge: string } {
  if (daysLeft <= 7) {
    return { row: 'bg-error/5', badge: 'text-error font-semibold' }
  }
  if (daysLeft <= 30) {
    return { row: 'bg-warning/5', badge: 'text-warning font-semibold' }
  }
  return { row: '', badge: 'text-success font-semibold' }
}

export function CertExpiryList({ data, isLoading }: CertExpiryListProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <div className="rounded-lg bg-brand-500/10 p-2 text-brand-500">
            <ShieldCheck className="h-5 w-5" />
          </div>
          <CardTitle>Certificate Expiry</CardTitle>
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <div className="space-y-3 p-6 pt-0">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center justify-between gap-4">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-4 w-24" />
              </div>
            ))}
          </div>
        ) : (data ?? []).length === 0 ? (
          <p className="text-sm text-content-muted py-8 text-center px-6">
            No certificates expiring soon
          </p>
        ) : (
          <div
            role="table"
            aria-label="Certificate expiry list"
            className="divide-y divide-border"
          >
            {/* Header */}
            <div
              role="row"
              className="flex items-center gap-4 px-6 py-2 bg-surface-subtle text-xs font-medium text-content-secondary uppercase tracking-wide"
            >
              <span role="columnheader" className="flex-1">Hostname</span>
              <span role="columnheader" className="w-32 text-right">Expires</span>
              <span role="columnheader" className="w-20 text-right">Days left</span>
            </div>
            {(data ?? []).map((cert) => {
              const { row, badge } = urgencyClasses(cert.days_left)
              return (
                <div
                  key={cert.host_id}
                  role="row"
                  className={cn(
                    'flex items-center gap-4 px-6 py-3 text-sm transition-colors duration-fast',
                    row
                  )}
                >
                  <span
                    role="cell"
                    className="flex-1 truncate text-content-primary font-medium"
                    title={cert.hostname}
                  >
                    {cert.hostname}
                  </span>
                  <span role="cell" className="w-32 text-right text-content-secondary">
                    {formatDate(cert.expires_at)}
                  </span>
                  <span role="cell" className={cn('w-20 text-right tabular-nums', badge)}>
                    {cert.days_left}d
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

import { cn } from '../utils/cn';

interface ProxyGroupBadgeProps {
  group: {
    name: string;
    color: string;
  };
  className?: string;
}

export function ProxyGroupBadge({ group, className }: ProxyGroupBadgeProps) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-sm', className)}>
      <span
        className="inline-block w-2.5 h-2.5 rounded-full shrink-0"
        style={{ backgroundColor: group.color }}
        aria-hidden="true"
      />
      <span>{group.name}</span>
    </span>
  );
}

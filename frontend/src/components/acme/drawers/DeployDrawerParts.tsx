import type { ReactNode } from 'react'
import type { ButtonProps } from '../../ui/button'
import { Button } from '../../ui/button'
import { Card } from '../../ui/card'
import { cn } from '../../../lib/utils'

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
      {children}
    </p>
  )
}

export function EnabledBadge({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={cn(
        'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
        enabled
          ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          : 'bg-muted text-muted-foreground',
      )}
    >
      {enabled ? '启用' : '停用'}
    </span>
  )
}

export function AutoDeployBadge({ enabled }: { enabled: boolean }) {
  if (!enabled) return null
  return (
    <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
      自动部署
    </span>
  )
}

export function CardTitleRow({
  title,
  children,
  className,
  wrap,
}: {
  title: ReactNode
  children?: ReactNode
  className?: string
  wrap?: boolean
}) {
  return (
    <div className={cn('flex items-center gap-2', wrap && 'flex-wrap', className)}>
      <span className="truncate font-mono text-[13px] font-medium">{title}</span>
      {children}
    </div>
  )
}

export function DeployCard({
  icon,
  actions,
  align = 'center',
  children,
}: {
  icon: ReactNode
  actions: ReactNode
  align?: 'center' | 'start'
  children: ReactNode
}) {
  return (
    <Card className="px-4 py-3">
      <div
        className={cn(
          'flex flex-col gap-3 sm:flex-row sm:gap-3',
          align === 'start' ? 'sm:items-start' : 'sm:items-center',
        )}
      >
        {icon}
        <div className="min-w-0 flex-1">{children}</div>
        <CardActions>{actions}</CardActions>
      </div>
    </Card>
  )
}

export function CardActions({ children }: { children: ReactNode }) {
  return <div className="flex gap-2 sm:contents">{children}</div>
}

export function CardIconButton({
  destructive,
  className,
  ...props
}: ButtonProps & { destructive?: boolean }) {
  return (
    <Button
      {...props}
      size="sm"
      variant="outline"
      className={cn('flex-1 sm:flex-none', destructive && 'hover:text-destructive', className)}
    />
  )
}

import { Check, ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from './dropdown-menu'

export interface SelectOption<T extends string | number> {
  value: T
  label: string
}

export function Select<T extends string | number>({
  value,
  onChange,
  options,
  placeholder = '请选择',
  className,
  id,
}: {
  value: T
  onChange: (v: T) => void
  options: SelectOption<T>[]
  placeholder?: string
  className?: string
  id?: string
}) {
  const current = options.find((o) => o.value === value)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        id={id}
        className={cn(
          'flex h-9 w-full items-center justify-between gap-2 rounded-md border border-input bg-background px-3 text-[13px] outline-none transition-colors',
          'hover:bg-accent/40 focus-visible:ring-2 focus-visible:ring-ring',
          'data-[state=open]:ring-2 data-[state=open]:ring-ring',
          className,
        )}
      >
        <span className={cn('truncate', !current && 'text-muted-foreground')}>
          {current ? current.label : placeholder}
        </span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="max-h-[260px] w-[var(--radix-dropdown-menu-trigger-width)] overflow-y-auto"
      >
        {options.map((opt) => (
          <DropdownMenuItem
            key={String(opt.value)}
            onSelect={() => onChange(opt.value)}
            className="justify-between"
          >
            <span className="truncate">{opt.label}</span>
            {opt.value === value && <Check className="h-3.5 w-3.5 shrink-0" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

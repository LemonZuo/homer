import { useRef, useState } from 'react'
import { Check, ChevronDown, Search } from 'lucide-react'
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
  disabled,
  searchable,
  searchPlaceholder = '搜索…',
}: {
  value: T
  onChange: (v: T) => void
  options: SelectOption<T>[]
  placeholder?: string
  className?: string
  id?: string
  disabled?: boolean
  searchable?: boolean
  searchPlaceholder?: string
}) {
  const current = options.find((o) => o.value === value)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const filtered = searchable && query.trim()
    ? options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase()))
    : options
  return (
    <DropdownMenu
      onOpenChange={(o) => {
        if (!o) setQuery('')
      }}
    >
      <DropdownMenuTrigger
        id={id}
        disabled={disabled}
        className={cn(
          'flex h-9 w-full items-center justify-between gap-2 rounded-md border border-input bg-background px-3 text-[13px] outline-none transition-colors',
          'hover:bg-accent/40 focus-visible:ring-2 focus-visible:ring-ring',
          'data-[state=open]:ring-2 data-[state=open]:ring-ring',
          'disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-background',
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
        onOpenAutoFocus={(e: Event) => {
          if (searchable) {
            e.preventDefault()
            inputRef.current?.focus()
          }
        }}
      >
        {searchable && (
          <div className="sticky top-0 z-10 -mx-1 mb-1 border-b border-border bg-popover px-2 pb-1.5 pt-1">
            <div className="flex items-center gap-1.5 rounded-sm border border-input bg-background px-2">
              <Search className="h-3 w-3 shrink-0 text-muted-foreground" />
              <input
                ref={inputRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  // 阻止 Radix typeahead，方向键/回车/Escape 仍交给 Radix 处理
                  if (!['ArrowDown', 'ArrowUp', 'Enter', 'Escape', 'Tab'].includes(e.key)) {
                    e.stopPropagation()
                  }
                }}
                placeholder={searchPlaceholder}
                className="h-7 w-full bg-transparent text-[12px] outline-none placeholder:text-muted-foreground"
              />
            </div>
          </div>
        )}
        {filtered.length === 0 ? (
          <div className="px-2 py-3 text-center text-[12px] text-muted-foreground">
            无匹配
          </div>
        ) : (
          filtered.map((opt) => (
            <DropdownMenuItem
              key={String(opt.value)}
              onSelect={() => onChange(opt.value)}
              className="justify-between"
            >
              <span className="truncate">{opt.label}</span>
              {opt.value === value && <Check className="h-3.5 w-3.5 shrink-0" />}
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

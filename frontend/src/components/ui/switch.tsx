import { cn } from '../../lib/utils'

interface Props {
  id?: string
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
  size?: 'sm' | 'md'
}

export function Switch({ id, checked, onChange, disabled, size = 'md' }: Props) {
  const trackCls =
    size === 'sm' ? 'h-4 w-7' : 'h-5 w-9'
  const thumbCls =
    size === 'sm' ? 'h-3 w-3' : 'h-4 w-4'
  const thumbOn =
    size === 'sm' ? 'translate-x-3.5' : 'translate-x-[1.125rem]'

  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation()
        onChange(!checked)
      }}
      className={cn(
        'relative inline-flex shrink-0 items-center rounded-full transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30',
        'disabled:cursor-not-allowed disabled:opacity-50',
        trackCls,
        checked ? 'bg-primary' : 'bg-input',
      )}
    >
      <span
        className={cn(
          'inline-block transform rounded-full bg-background shadow ring-0 transition-transform',
          thumbCls,
          checked ? thumbOn : 'translate-x-0.5',
        )}
      />
    </button>
  )
}

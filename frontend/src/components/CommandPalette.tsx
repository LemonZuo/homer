import { useEffect } from 'react'
import { Command } from 'cmdk'
import { Search } from 'lucide-react'
import { tables } from '../tables'
import { getColorSet } from '../colors'
import { cn } from '../lib/utils'
import { Kbd } from './ui/kbd'

interface Props {
  open: boolean
  onClose: () => void
  onPick: (key: string) => void
  currentKey: string
}

/**
 * Cmd/Ctrl+K 触发的命令面板：cmdk 实现 fuzzy 搜索 + 键盘导航。
 */
export default function CommandPalette({ open, onClose, onPick, currentKey }: Props) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-[60] flex items-start justify-center px-4 pt-[12vh] animate-in fade-in-0 duration-150"
      onClick={onClose}
    >
      <div className="fixed inset-0 bg-black/50 backdrop-blur-[2px]" aria-hidden />
      <div
        onClick={(e) => e.stopPropagation()}
        className="relative w-full max-w-md overflow-hidden rounded-xl border border-border bg-popover shadow-2xl animate-in zoom-in-95 slide-in-from-top-2 duration-150"
      >
        <Command label="命令面板" className="flex flex-col">
          <div className="flex items-center gap-2 border-b border-border px-3.5">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <Command.Input
              autoFocus
              placeholder="搜索表名…"
              className="flex-1 bg-transparent py-3 text-[13.5px] outline-none placeholder:text-muted-foreground"
            />
            <Kbd>esc</Kbd>
          </div>
          <Command.List className="max-h-72 overflow-y-auto p-1.5">
            <Command.Empty className="px-3 py-8 text-center text-[12.5px] text-muted-foreground">
              没有匹配
            </Command.Empty>
            <Command.Group
              heading="表"
              className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-[10.5px] [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wider [&_[cmdk-group-heading]]:text-muted-foreground"
            >
              {tables.map((t) => {
                const cs = getColorSet(t.color)
                const active = t.key === currentKey
                return (
                  <Command.Item
                    key={t.key}
                    value={`${t.label} ${t.key}`}
                    onSelect={() => onPick(t.key)}
                    className={cn(
                      'flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-[13px] transition',
                      'data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground',
                    )}
                  >
                    <span className={cn('h-1.5 w-1.5 rounded-full', cs.dot)} />
                    <span className="flex-1 font-medium">{t.label}</span>
                    {active && (
                      <span className="text-[10.5px] text-muted-foreground">当前</span>
                    )}
                    <span className="font-mono text-[10.5px] text-muted-foreground">{t.key}</span>
                  </Command.Item>
                )
              })}
            </Command.Group>
          </Command.List>
          <div className="flex items-center justify-between border-t border-border bg-muted/30 px-3 py-2 text-[10.5px] text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <Kbd>↑↓</Kbd> 选择
              <span className="mx-1">·</span>
              <Kbd>↵</Kbd> 切换
            </span>
            <span>{tables.length} 张表</span>
          </div>
        </Command>
      </div>
    </div>
  )
}

import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import type { Channel, ModuleBindings, ModuleMeta } from './types'

interface BindingSectionProps {
  modules: ModuleMeta[]
  channels: Channel[]
  bindings: ModuleBindings
  activeClassName: string
  onToggle: (module: string, channelID: number) => void
}

export function BindingSection({
  modules,
  channels,
  bindings,
  activeClassName,
  onToggle,
}: BindingSectionProps) {
  return (
    <Card className="px-4 py-4">
      <div className="mb-3 text-[13px] font-medium">模块绑定</div>
      <div className="space-y-4">
        {modules.map((m) => {
          const bound = bindings[m.key] ?? []
          return (
            <div key={m.key} className="rounded-md border border-border px-3 py-3">
              <div className="mb-2 text-[12.5px] font-medium">{m.label}</div>
              {channels.length === 0 ? (
                <p className="text-[11.5px] text-muted-foreground">先创建通道再绑定</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {channels.map((ch) => {
                    const on = bound.includes(ch.id)
                    return (
                      <button
                        key={ch.id}
                        type="button"
                        onClick={() => onToggle(m.key, ch.id)}
                        className={cn(
                          'rounded-md border px-2.5 py-1 text-[12px] font-medium transition-colors',
                          on
                            ? activeClassName
                            : 'border-border bg-background text-muted-foreground hover:text-foreground',
                        )}
                      >
                        {ch.name}
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </Card>
  )
}

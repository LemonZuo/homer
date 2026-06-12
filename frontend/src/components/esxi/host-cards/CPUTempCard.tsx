import { Activity } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import { tempTone } from '../format'
import type { CPUTemperature } from '../types'
import { SectionHead } from '../ui'

export function CPUTempCard({ t }: { t: CPUTemperature }) {
  const tjmax = t.tjmax_c > 0 ? t.tjmax_c : 100
  const cores = t.cores ?? []
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Activity className="h-3.5 w-3.5" />}
        title="CPU 温度"
        suffix={
          cores.length > 0 ? (
            <span className="text-[11px] text-muted-foreground">{cores.length} 核</span>
          ) : undefined
        }
      />
      {cores.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">未拿到 CPU 温度(vsish MSR 不可读)</p>
      ) : (
        <div className="space-y-2.5">
          {cores.map((c) => {
            const tone = tempTone(c.temp_c, c.headroom_c)
            const pct = Math.max(0, Math.min(100, (c.temp_c / tjmax) * 100))
            return (
              <div key={c.id} className="flex items-center gap-2">
                <span className="w-12 shrink-0 text-[11px] tabular-nums text-muted-foreground">CPU {c.id}</span>
                <div className="relative h-2.5 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn('absolute inset-y-0 left-0 rounded-full transition-all', tone.bar)}
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <span className={cn('w-12 shrink-0 text-right text-[12px] font-semibold tabular-nums', tone.text)}>
                  {c.temp_c}°C
                </span>
                <span className="w-12 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">
                  Δ{c.headroom_c}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}

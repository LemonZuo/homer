import { Cpu } from 'lucide-react'

import { Card } from '../../ui/card'
import { fmtFreq, fmtKB } from '../format'
import type { CPUStatic } from '../types'
import { KV, SectionHead } from '../ui'

export function CPUStaticCard({ c }: { c: CPUStatic }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Cpu className="h-3.5 w-3.5" />}
        title="CPU"
        suffix={
          c.brand ? (
            <span
              className="min-w-0 truncate text-[11.5px] font-medium text-muted-foreground"
              title={c.brand}
            >
              {c.brand}
            </span>
          ) : undefined
        }
      />
      <div className="grid grid-cols-2 gap-2.5">
        <KV k="核数" v={c.cores > 0 ? c.cores : '—'} />
        <KV k="主频" v={fmtFreq(c.freq_mhz)} />
        <KV k="L2 缓存" v={fmtKB(c.l2_kb)} />
        <KV k="L3 缓存" v={fmtKB(c.l3_kb)} />
        <KV
          k="Family / Model / Step"
          v={`${c.family || '—'} / ${c.model || '—'} / ${c.stepping || '—'}`}
          mono
        />
        <KV
          k="TjMax"
          v={c.tjmax_c > 0 ? `${c.tjmax_c}°C` : '—'}
        />
      </div>
    </Card>
  )
}

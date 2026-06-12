import { cn } from '../../../lib/utils'
import type { DiskHealth } from '../types'
import { METRIC_OPTIONS, RANGE_OPTIONS } from './constants'
import { EsxiSeriesChart } from './EsxiSeriesChart'
import { useEsxiHistorySeries } from './useEsxiHistorySeries'

export function HistorySection({
  hostKind,
  hostID,
  disks,
}: {
  hostKind: string
  hostID: number
  disks?: DiskHealth[]
}) {
  const { range, setRange, metric, setMetric, series, loading } = useEsxiHistorySeries({ hostKind, hostID })

  return (
    <div className="rounded-md border border-border/60 bg-muted/30 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap gap-1">
          {METRIC_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => setMetric(o.value)}
              className={cn(
                'rounded-md border px-2 py-0.5 text-[11px] transition-colors',
                metric === o.value
                  ? 'border-[#4f89c0]/60 bg-[#4f89c0]/15 text-[#3d6e9d] dark:text-[#9bc1e0]'
                  : 'border-border bg-background text-muted-foreground hover:border-border/80 hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap gap-1">
          {RANGE_OPTIONS.map((o) => (
            <button
              key={o.value}
              type="button"
              onClick={() => setRange(o.value)}
              className={cn(
                'rounded-md border px-2 py-0.5 text-[11px] transition-colors',
                range === o.value
                  ? 'border-[#4f89c0]/60 bg-[#4f89c0]/15 text-[#3d6e9d] dark:text-[#9bc1e0]'
                  : 'border-border bg-background text-muted-foreground hover:border-border/80 hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          ))}
        </div>
      </div>
      <EsxiSeriesChart series={series} loading={loading} metric={metric} disks={disks} />
    </div>
  )
}

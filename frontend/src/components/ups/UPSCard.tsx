import { useState } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'

import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import { POWER_META } from './constants'
import { fmtBatteryType, fmtNum, fmtRuntime, fmtStaleAge, fmtTime, isStaleSample, useNowTick } from './format'
import { SeriesChart } from './history/SeriesChart'
import type { SnapshotUPS } from './types'
import { BatteryCell } from './BatteryCell'

export function UPSCard({
  ups,
  hostKind,
  hostID,
}: {
  ups: SnapshotUPS
  hostKind: string
  hostID: number
}) {
  const [expanded, setExpanded] = useState(false)
  const meta = POWER_META[ups.power_source] ?? POWER_META.unknown
  const hasData = ups.battery_percent >= 0
  const cs = getColorSet('teal')
  const now = useNowTick()
  const sampledAtMs = ups.sampled_at ? new Date(ups.sampled_at).getTime() : 0
  const isStale = isStaleSample(ups.sampled_at, now)
  const staleAge = isStale && sampledAtMs > 0 ? fmtStaleAge(now - sampledAtMs) : ''

  return (
    <Card className={cn(
      'group transition-[transform,box-shadow,border-color,opacity,filter] duration-500 ease-out hover:-translate-y-0.5',
      cs.border,
      cs.halo,
      isStale && 'opacity-60 saturate-50',
    )}>
      <div className="p-5">
        {/* 顶部 */}
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-muted-foreground">
                {ups.name}
              </span>
              {ups.mfr && (
                <span className="truncate text-[12px] text-muted-foreground">{ups.mfr}</span>
              )}
            </div>
            <div
              className="mt-1 truncate text-[15px] font-semibold tracking-tight"
              title={ups.model}
            >
              {ups.model || '(无型号)'}
            </div>
          </div>
          {isStale ? (
            <span
              className="shrink-0 rounded-full border border-border bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
              title={ups.sampled_at ? `最近采样 ${ups.sampled_at}` : '未拿到任何采样'}
            >
              <span className="inline-flex items-center gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/60" />
                已离线{staleAge && ` · ${staleAge}`}
              </span>
            </span>
          ) : (
            <span
              className={cn(
                'shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-medium',
                meta.pill,
              )}
            >
              <span className="inline-flex items-center gap-1.5">
                <span
                  className={cn(
                    'h-1.5 w-1.5 rounded-full',
                    meta.dot,
                    meta.pulse && 'animate-pulse',
                  )}
                />
                {meta.label}
              </span>
            </span>
          )}
        </div>

        {/* 主体 */}
        <div className="mt-5 flex items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <BatteryCell
              percent={ups.battery_percent}
              powerSource={ups.power_source}
              charging={/\bCHRG\b/.test((ups.raw_status ?? '').toUpperCase())}
            />
            <div>
              {hasData ? (
                <div className={cn('flex items-baseline gap-1 leading-none', meta.text)}>
                  <span className="text-[44px] font-semibold tabular-nums tracking-tight sm:text-[52px]">
                    {ups.battery_percent}
                  </span>
                  <span className="text-xl font-semibold">%</span>
                </div>
              ) : (
                <span className="text-[36px] font-semibold leading-none tabular-nums text-muted-foreground/60">
                  — —
                </span>
              )}
              <div className="mt-1.5 text-[11.5px] text-muted-foreground">剩余电量</div>
            </div>
          </div>
          <div className="text-right">
            <div className={cn('text-2xl font-semibold leading-none tabular-nums sm:text-3xl', meta.text)}>
              {fmtRuntime(ups.runtime_minutes)}
            </div>
            <div className="mt-1.5 text-[11.5px] text-muted-foreground">预估可供电</div>
          </div>
        </div>

        {/* 电池信息:类型 / 电压 / 额定电压 */}
        {(ups.battery_type || ups.battery_voltage >= 0 || ups.battery_nominal_voltage > 0) && (
          <div className="mt-5 grid grid-cols-3 gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
            <div className="min-w-0">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">电池类型</div>
              <div className="mt-0.5 leading-none" title={ups.battery_type || undefined}>
                <span className="text-[14px] font-semibold text-foreground">
                  {fmtBatteryType(ups.battery_type)}
                </span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">电池电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.battery_voltage, 1)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">额定电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.battery_nominal_voltage, 0)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
          </div>
        )}

        {/* 电气指标:输入电压 / 输出电压 / 实时功率(负载与功率重复,只留功率) */}
        {(ups.input_voltage >= 0 || ups.output_voltage >= 0 || ups.real_power >= 0) && (
          <div className="mt-3 grid grid-cols-3 gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
            <div className="min-w-0">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">输入电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.input_voltage, 1)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">输出电压</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.output_voltage, 1)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">V</span>
              </div>
            </div>
            <div className="min-w-0 border-l border-border/60 pl-3">
              <div className="text-[10.5px] uppercase tracking-wide text-muted-foreground">实时功率</div>
              <div className="mt-0.5 flex items-baseline gap-0.5 leading-none">
                <span className="text-[15px] font-semibold tabular-nums text-foreground">
                  {fmtNum(ups.real_power)}
                </span>
                <span className="text-[10.5px] text-muted-foreground">W</span>
              </div>
            </div>
          </div>
        )}

        {/* 底部信息 */}
        <div className="mt-3 flex items-center justify-between gap-2 border-t border-border/60 pt-3 text-[11.5px]">
          <div className="flex min-w-0 items-center gap-2 text-muted-foreground">
            {ups.raw_status ? (
              <span className="truncate" title={ups.raw_status}>
                状态码 <code className="rounded bg-muted px-1 py-0.5 text-[10.5px] text-foreground/80">{ups.raw_status}</code>
              </span>
            ) : (
              <span className="text-muted-foreground/70">尚未收到状态码</span>
            )}
          </div>
          <div className="shrink-0 tabular-nums text-muted-foreground">
            {fmtTime(ups.sampled_at)}
          </div>
        </div>

        {/* 展开/收起 */}
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-3 flex w-full items-center justify-center gap-1 rounded-md border border-dashed border-border py-1.5 text-[11.5px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          {expanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
          {expanded ? '收起历史' : '查看历史'}
        </button>

        <SeriesChart hostKind={hostKind} hostID={hostID} upsName={ups.name} expanded={expanded} />
      </div>
    </Card>
  )
}

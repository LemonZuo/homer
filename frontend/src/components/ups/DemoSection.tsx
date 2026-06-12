import { useMemo } from 'react'
import { X } from 'lucide-react'

import { Button } from '../ui/button'
import { DEMO_BATTERY_VARIANTS } from './constants'
import { useNowTick } from './format'
import { SummaryCard } from './SummaryCard'
import type { Snapshot, SnapshotUPS } from './types'
import { UPSCard } from './UPSCard'

export function DemoSection({ onClose }: { onClose: () => void }) {
  // 演示卡的 sampled_at 跟着 useNowTick 滴答推进:
  //   demo-offline -> now - 15min,始终展示离线样式
  //   其余卡       -> now,始终保持新鲜
  const now = useNowTick()
  const demoUpses = useMemo<SnapshotUPS[]>(
    () =>
      DEMO_BATTERY_VARIANTS.map((u) => ({
        ...u,
        sampled_at: new Date(u.name === 'demo-offline' ? now - 15 * 60 * 1000 : now).toISOString(),
      })),
    [now],
  )
  const demoSnapshots: Snapshot[] = [
    {
      host_kind: 'demo',
      host_id: -1,
      host_name: '演示机器',
      endpoint: 'demo:0',
      reachable: true,
      upses: demoUpses,
    },
  ]
  return (
    <div className="mt-10 rounded-2xl border border-dashed border-muted-foreground/30 bg-muted/20 p-4 sm:p-5">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-[14px] font-semibold">样式演示</div>
          <div className="mt-0.5 text-[11.5px] text-muted-foreground">
            非真实数据,仅展示聚合总览卡、不同电量 / 电源 / 充电状态、以及超过 10 分钟未上报的离线样式。
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={onClose}>
          <X className="mr-1 h-3.5 w-3.5" />
          退出演示
        </Button>
      </div>
      <SummaryCard snapshots={demoSnapshots} />
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        {demoUpses.map((u) => (
          <UPSCard key={u.name} ups={u} hostKind="demo" hostID={-1} />
        ))}
      </div>
    </div>
  )
}

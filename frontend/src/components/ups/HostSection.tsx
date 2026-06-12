import { AlertTriangle, Server } from 'lucide-react'

import { Card } from '../ui/card'
import type { Snapshot } from './types'
import { UPSCard } from './UPSCard'

function HostHeader({ host }: { host: Snapshot }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Server className="h-4 w-4 text-muted-foreground" />
      <span className="text-[14px] font-semibold tracking-tight">{host.host_name}</span>
      <span className="text-[12px] text-muted-foreground">{host.endpoint}</span>
      {!host.reachable && host.upses.length === 0 && (
        <span className="ml-auto flex items-center gap-1 text-[12px] text-rose-500">
          <AlertTriangle className="h-3.5 w-3.5" />
          {host.error ? '采样失败' : '尚无数据'}
        </span>
      )}
    </div>
  )
}

function HostEmptyCard({ host }: { host: Snapshot }) {
  return (
    <Card className="px-4 py-6 text-center text-[12px] text-muted-foreground">
      {host.error
        ? `采样失败:${host.error}`
        : '尚未采集到 UPS 数据(机器未装 NUT / 未绑 UPS / 等待首次采样)'}
    </Card>
  )
}

export function HostSection({ host }: { host: Snapshot }) {
  if (host.upses.length === 0) {
    return (
      <div className="space-y-3">
        <HostHeader host={host} />
        <HostEmptyCard host={host} />
      </div>
    )
  }

  return (
    <>
      {host.upses.map((u) => (
        <div key={`${host.host_kind}-${host.host_id}-${u.name}`} className="space-y-3">
          <HostHeader host={host} />
          <UPSCard ups={u} hostKind={host.host_kind} hostID={host.host_id} />
        </div>
      ))}
    </>
  )
}

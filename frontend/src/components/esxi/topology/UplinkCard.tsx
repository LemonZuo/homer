import { Network, Server } from 'lucide-react'

import { COL_W_UP, UP_CARD_H } from './constants'
import type { NIC } from './types'
import { fmtBitrate } from './utils'

export function UplinkCard({ up, nic }: { up: string; nic?: NIC }) {
  const linkUp = nic ? nic.link_status === 'Up' : false
  return (
    <div
      className="rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_UP, height: UP_CARD_H }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-sky-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-sky-600 dark:text-sky-400" />
        <span className="text-[12px] font-semibold text-foreground">
          物理适配器
        </span>
        <span className="ml-auto text-[10px] text-muted-foreground">
          {linkUp ? fmtBitrate(nic?.speed_mbps) : '未上线'}
        </span>
      </div>
      <div className="px-2.5 py-1">
        <div className="flex items-center gap-1 leading-tight">
          <Server className="h-3 w-3 shrink-0 text-muted-foreground" />
          <span
            className="truncate font-mono text-[11.5px] font-medium text-foreground"
            title={up}
          >
            {up}
          </span>
        </div>
        {nic?.mac ? (
          <div className="truncate pl-4 font-mono text-[9.5px] leading-tight text-muted-foreground">
            {nic.mac}
          </div>
        ) : null}
      </div>
    </div>
  )
}

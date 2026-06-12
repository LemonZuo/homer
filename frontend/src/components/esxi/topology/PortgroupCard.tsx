import { Box, ChevronDown, Network } from 'lucide-react'

import { COL_W_PG, PG_VM_ROW, PG_VMK_ROW, PG_VMK_SECTION } from './constants'
import type { VMInfo, VMKInfo } from './types'

export function PortgroupCard({ pg, vms, vmks }: { pg: string; vms: VMInfo[]; vmks: VMKInfo[] }) {
  const empty = vms.length === 0 && vmks.length === 0
  return (
    <div
      className="rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_PG }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-amber-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
        <span
          className="truncate text-[12px] font-semibold text-foreground"
          title={pg}
        >
          {pg}
        </span>
        <span className="ml-auto text-[10px] text-muted-foreground">
          {vms.length} 虚机
        </span>
      </div>
      {empty ? (
        <div className="px-2.5 py-1.5 text-[10.5px] text-muted-foreground">
          无虚机
        </div>
      ) : (
        <div className="pb-1.5 pt-0">
          {vmks.length > 0 ? (
            <div className={vms.length > 0 ? 'border-b border-border/70 pb-0.5' : ''}>
              <div
                className="flex items-center gap-1.5 px-2.5 text-[10.5px] font-medium text-muted-foreground"
                style={{ height: PG_VMK_SECTION }}
              >
                <ChevronDown className="h-3 w-3 shrink-0" />
                <span>VMkernel 端口 ({vmks.length})</span>
              </div>
              {vmks.map((vmk) => (
                <div
                  key={vmk.name}
                  className="flex items-center gap-1.5 px-2.5"
                  style={{ height: PG_VMK_ROW }}
                  title={[
                    vmk.ipv4 ? `${vmk.name}: ${vmk.ipv4}` : vmk.name,
                    vmk.mac ? `MAC ${vmk.mac}` : '',
                  ].filter(Boolean).join(' · ')}
                >
                  <Network className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-foreground">
                    {vmk.name}
                    {vmk.ipv4 ? `: ${vmk.ipv4}` : ''}
                  </span>
                </div>
              ))}
            </div>
          ) : null}
          {vms.map((v) => {
            const detail = [v.ip, v.mac].filter(Boolean).join(' · ') || '—'
            return (
              <div
                key={v.vmName + v.mac}
                className="flex flex-col justify-center px-2.5"
                style={{ height: PG_VM_ROW }}
                title={[
                  v.ip ? `IP ${v.ip}` : '',
                  v.mac ? `MAC ${v.mac}` : '',
                  v.teamUplink,
                ].filter(Boolean).join(' · ')}
              >
                <div className="flex items-center gap-1 leading-tight">
                  <Box className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate text-[11.5px] text-foreground">
                    {v.vmName}
                  </span>
                  {v.teamUplink ? (
                    <span className="shrink-0 font-mono text-[9.5px] text-muted-foreground">
                      {v.teamUplink}
                    </span>
                  ) : null}
                </div>
                <div className="truncate pl-4 font-mono text-[9.5px] leading-tight text-muted-foreground">
                  {detail}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

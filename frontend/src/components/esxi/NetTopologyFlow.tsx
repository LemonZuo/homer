import { useMemo, useState } from 'react'
import { ChevronDown, ChevronUp, Waypoints } from 'lucide-react'

import { Card } from '../ui/card'
import { COL_W_CH, COL_W_PG, COL_W_UP, LINE_ZONE_W, SW_GAP } from './topology/constants'
import type { NetTopology, NIC, VMInfo, VMKInfo, VMRef } from './topology/types'
import { VSwitchBlock } from './topology/VSwitchBlock'

export type { NetTopology } from './topology/types'

export function NetTopologyFlow({
  topo,
  nics,
  vms,
}: {
  topo: NetTopology
  nics?: NIC[]
  vms?: VMRef[]
}) {
  const [expanded, setExpanded] = useState(false)

  const vmIDByName = useMemo(
    () => new Map<string, number>((vms ?? []).map((v) => [v.name, v.id])),
    [vms],
  )

  const vmsByPG = useMemo(() => {
    const m = new Map<string, VMInfo[]>()
    for (const link of topo.vm_nics ?? []) {
      const key = link.vswitch + '||' + link.portgroup
      const arr = m.get(key) ?? []
      if (!arr.find((v) => v.vmName === link.vm_name)) {
        arr.push({
          vmid: vmIDByName.get(link.vm_name) ?? link.vmid,
          vmName: link.vm_name,
          teamUplink: link.team_uplink,
          mac: link.mac,
          ip: link.ip,
        })
      }
      m.set(key, arr)
    }
    for (const arr of m.values()) {
      arr.sort((a, b) => {
        if (a.vmid !== b.vmid) return a.vmid - b.vmid
        return a.vmName.localeCompare(b.vmName)
      })
    }
    return m
  }, [topo, vmIDByName])

  const vmksByPG = useMemo(() => {
    const m = new Map<string, VMKInfo[]>()
    for (const vmk of topo.vmk_ports ?? []) {
      const key = vmk.vswitch + '||' + vmk.portgroup
      const arr = m.get(key) ?? []
      if (!arr.find((v) => v.name === vmk.name)) {
        arr.push({
          name: vmk.name,
          mac: vmk.mac,
          ipv4: vmk.ipv4,
          enabled: vmk.enabled,
        })
      }
      m.set(key, arr)
    }
    return m
  }, [topo])

  const nicByName = useMemo(
    () => new Map<string, NIC>((nics ?? []).map((n) => [n.name, n])),
    [nics],
  )

  const vswitches = topo.vswitches ?? []
  const pNICCount = nics?.length ?? new Set(vswitches.flatMap((s) => s.uplinks ?? [])).size
  const vmKernelCount = topo.vmk_ports?.length ?? 0

  return (
    <Card className="px-3 py-3">
      <button
        type="button"
        className="flex w-full items-center gap-1.5 text-left"
        onClick={() => setExpanded((v) => !v)}
      >
        <Waypoints className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-[12.5px] font-medium text-foreground">网络拓扑</span>
        <span className="ml-auto rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
          {pNICCount} 物理 · {vswitches.length} vSwitch · {topo.vm_nics?.length ?? 0} vNIC · {vmKernelCount} VMkernel
        </span>
        {expanded ? (
          <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        )}
      </button>
      {expanded ? (
        <div className="mt-2 w-full overflow-x-auto rounded-md border border-border bg-muted/10 p-3">
          <div
            className="flex flex-col items-center"
            style={{ rowGap: SW_GAP, minWidth: COL_W_PG + LINE_ZONE_W * 2 + COL_W_CH + COL_W_UP }}
          >
            {vswitches.map((sw) => (
              <VSwitchBlock
                key={sw.name}
                sw={sw}
                vmsByPG={vmsByPG}
                vmksByPG={vmksByPG}
                nicByName={nicByName}
              />
            ))}
          </div>
        </div>
      ) : null}
    </Card>
  )
}

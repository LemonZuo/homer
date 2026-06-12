export interface NIC {
  name: string
  driver?: string
  mac: string
  link_status: string
  speed_mbps: number
  duplex?: string
}

export interface VSwitchInfo {
  name: string
  uplinks: string[]
  portgroups: string[]
}

export interface VMNICLink {
  vmid: number
  vm_name: string
  vswitch: string
  portgroup: string
  mac: string
  ip?: string
  team_uplink: string
}

export interface VMRef {
  id: number
  name: string
}

export interface VMKPort {
  name: string
  vswitch: string
  portgroup: string
  mac?: string
  ipv4?: string
  enabled: boolean
}

export interface NetTopology {
  vswitches: VSwitchInfo[]
  vm_nics: VMNICLink[]
  vmk_ports?: VMKPort[]
}

export type VMInfo = { vmid: number; vmName: string; teamUplink: string; mac: string; ip?: string }
export type VMKInfo = { name: string; mac?: string; ipv4?: string; enabled: boolean }

export interface SideAnchor {
  y: number
  active: boolean
}

// StripBlock:对齐 ESXi 的 portgroup-block —— 每个分组 (一个 PG 或一个 uplink) 一根独立的 strip 小条,
// 而不是整根贯穿。同一个 PG 内的多 anchor 会拼成连续条,单 anchor 会变成 ESXi 那种"突出小框"。
export interface StripBlock {
  topY: number
  bottomY: number
  anchors: SideAnchor[]
}

import { PG_EMPTY, PG_FOOTER, PG_HEADER, PG_VM_ROW, PG_VMK_ROW, PG_VMK_SECTION } from './constants'

export function pgHeight(vms: number, vmks: number) {
  if (vms === 0 && vmks === 0) return PG_EMPTY
  return (
    PG_HEADER +
    (vmks > 0 ? PG_VMK_SECTION + PG_VMK_ROW * vmks : 0) +
    PG_VM_ROW * vms +
    PG_FOOTER
  )
}

export function fmtBitrate(mbps?: number): string {
  if (mbps == null || mbps < 0) return '—'
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`
  return `${mbps} Mbps`
}

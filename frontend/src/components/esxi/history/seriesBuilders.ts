import { shortDeviceLabel } from '../format'
import type { DiskHealth, SeriesPoint } from '../types'
import { LINE_COLORS } from './constants'
import type { LineSeries } from './types'

// buildCoreLines 把按时间桶的 cpu_cores 明细转成"按核 id 分组"的多条曲线。
// 出现过的核 id 取并集排序;某桶没该核(或整桶 cpu_cores 缺失)落 null,前端跳点不连。
export function buildCoreLines(series: SeriesPoint[], ts: number[]): LineSeries[] {
  const idSet = new Set<number>()
  for (const p of series) {
    for (const c of p.cpu_cores ?? []) idSet.add(c.id)
  }
  const ids = [...idSet].sort((a, b) => a - b)
  return ids.map((id, idx) => ({
    id: `core-${id}`,
    label: `核 ${id}`,
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const c = (p.cpu_cores ?? []).find((x) => x.id === id)
      return { t: ts[i], v: c && c.temp_c >= 0 ? c.temp_c : null }
    }),
  }))
}

export function buildDiskLines(series: SeriesPoint[], ts: number[], disks?: DiskHealth[]): LineSeries[] {
  const devSet = new Set<string>()
  for (const p of series) {
    for (const d of p.disks ?? []) devSet.add(d.device)
  }
  const devs = [...devSet].sort()
  const labelByDevice = new Map((disks ?? []).map((d) => [d.device, d.model || d.type || shortDeviceLabel(d.device)]))
  return devs.map((dev, idx) => ({
    id: `disk-${dev}`,
    label: labelByDevice.get(dev) ?? shortDeviceLabel(dev),
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const d = (p.disks ?? []).find((x) => x.device === dev)
      return { t: ts[i], v: d && d.temp_c >= 0 ? d.temp_c : null }
    }),
  }))
}

// buildDiskUsageLines 按 device 维度把 disk_usage(已用字节) 转为 GiB 多线;旧桶或缺失值落 null。
export function buildDiskUsageLines(series: SeriesPoint[], ts: number[], disks?: DiskHealth[]): LineSeries[] {
  const devSet = new Set<string>()
  for (const p of series) {
    for (const d of p.disk_usage ?? []) devSet.add(d.device)
  }
  const devs = [...devSet].sort()
  const labelByDevice = new Map((disks ?? []).map((d) => [d.device, d.model || d.type || shortDeviceLabel(d.device)]))
  const GiB = 1024 ** 3
  return devs.map((dev, idx) => ({
    id: `disk-usage-${dev}`,
    label: labelByDevice.get(dev) ?? shortDeviceLabel(dev),
    color: LINE_COLORS[idx % LINE_COLORS.length],
    points: series.map((p, i) => {
      const d = (p.disk_usage ?? []).find((x) => x.device === dev)
      return { t: ts[i], v: d && d.used_bytes > 0 ? d.used_bytes / GiB : null }
    }),
  }))
}

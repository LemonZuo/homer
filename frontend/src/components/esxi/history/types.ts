export type MetricKey =
  | 'cpu_cores'
  | 'disk_per_disk'
  | 'cpu_usage'
  | 'memory_used'
  | 'disk_usage'
  | 'vm_on'
  | 'mce'

export interface MiniPoint {
  t: number
  v: number | null
}

export interface LineSeries {
  id: string
  label: string
  color: string
  points: MiniPoint[]
}

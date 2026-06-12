import type { MetricKey } from './types'

// 多线曲线的颜色色卡(色相循环);超过 10 条会循环复用。
export const LINE_COLORS = [
  'rgb(168 85 247)',
  'rgb(14 165 233)',
  'rgb(16 185 129)',
  'rgb(244 63 94)',
  'rgb(245 158 11)',
  'rgb(99 102 241)',
  'rgb(20 184 166)',
  'rgb(236 72 153)',
  'rgb(132 204 22)',
  'rgb(249 115 22)',
]

export const METRIC_OPTIONS: { value: MetricKey; label: string }[] = [
  { value: 'cpu_cores', label: 'CPU 温度' },
  { value: 'disk_per_disk', label: '磁盘温度' },
  { value: 'cpu_usage', label: 'CPU 使用量' },
  { value: 'memory_used', label: '内存使用量' },
  { value: 'disk_usage', label: '磁盘使用量' },
  { value: 'vm_on', label: '运行 VM' },
  { value: 'mce', label: 'MCE 累计' },
]

export const RANGE_OPTIONS = [
  { value: '1h', label: '1 小时' },
  { value: '6h', label: '6 小时' },
  { value: '24h', label: '24 小时' },
  { value: '3d', label: '3 天' },
  { value: '7d', label: '7 天' },
]

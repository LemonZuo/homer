import type { ComponentType } from 'react'
import {
  Activity,
  Cake,
  CalendarClock,
  Clock,
  RefreshCw,
  ShieldCheck,
  Trash2,
} from 'lucide-react'

export const JOB_META: Record<string, { icon: ComponentType<{ className?: string }>; label: string }> = {
  birthday: { icon: Cake, label: '生日提醒' },
  event: { icon: CalendarClock, label: '事项提醒' },
  'acme-renew': { icon: ShieldCheck, label: 'ACME 续期' },
  'acme-deploy-retry': { icon: RefreshCw, label: '部署失败重试' },
  'ups-sample': { icon: Activity, label: 'UPS 采样' },
  'ups-cleanup': { icon: Trash2, label: 'UPS 采样清理' },
  'esxi-sample': { icon: Activity, label: 'ESXi 采样' },
  'esxi-cleanup': { icon: Trash2, label: 'ESXi 采样清理' },
}

export const DEFAULT_JOB_META = { icon: Clock, label: '' }

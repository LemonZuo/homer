import { Server } from 'lucide-react'

import { Card } from '../../ui/card'
import { fmtBytes, fmtDateTime, fmtUptime } from '../format'
import type { HostBoot, MemoryInfo, PlatformInfo } from '../types'
import { KV, SectionHead } from '../ui'

export function PlatformCard({ p, m, boot }: { p: PlatformInfo; m?: MemoryInfo; boot?: HostBoot }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Server className="h-3.5 w-3.5" />}
        title="平台"
        suffix={
          p.esxi_version ? (
            <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
              ESXi {p.esxi_version}{p.esxi_build > 0 ? ` · build-${p.esxi_build}` : ''}
            </span>
          ) : null
        }
      />
      <div className="grid grid-cols-2 gap-2.5">
        <KV k="厂商" v={p.vendor || '—'} title={p.vendor} />
        <KV k="型号" v={p.product || '—'} title={p.product} />
        <KV k="序列号" v={p.serial || '—'} mono title={p.serial} />
        <KV
          k="IPMI"
          v={
            p.ipmi_supported ? (
              <span className="text-emerald-600 dark:text-emerald-400">支持</span>
            ) : (
              <span className="text-muted-foreground">不支持</span>
            )
          }
        />
        <KV k="总内存" v={m && m.mem_total_bytes > 0 ? fmtBytes(m.mem_total_bytes) : '—'} />
        <KV k="可用内存" v={m && m.mem_free_bytes > 0 ? fmtBytes(m.mem_free_bytes) : '—'} />
      </div>
      {boot && boot.uptime_seconds >= 0 ? <BootLine boot={boot} /> : null}
    </Card>
  )
}

function BootLine({ boot }: { boot: HostBoot }) {
  const uptimeText = fmtUptime(boot.uptime_seconds)
  const bootedText = fmtDateTime(boot.booted_at)
  return (
    <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border pt-2 text-[11.5px] text-muted-foreground">
      <span>
        运行 <span className="font-medium text-foreground">{uptimeText}</span>
      </span>
      {bootedText ? <span>启动于 {bootedText}</span> : null}
      {boot.crash_dump_count > 0 ? (
        <span
          className="rounded-md border border-rose-500/40 bg-rose-500/10 px-1.5 py-0.5 text-rose-700 dark:text-rose-300"
          title={`/var/core/ 下残留 ${boot.crash_dump_count} 个 VMkernel 崩溃转储 (PSOD 紫屏)。排查无误后可手动删除 /var/core/vmkernel-zdump.*`}
        >
          崩溃次数：{boot.crash_dump_count}
        </span>
      ) : null}
    </div>
  )
}

import { Usb } from 'lucide-react'

import { cn } from '../../../lib/utils'
import { Card } from '../../ui/card'
import type { USBState } from '../types'
import { SectionHead } from '../ui'

export function USBCard({ u }: { u: USBState }) {
  const owned = u.vm_owned ?? []
  const avail = u.available_for_passthrough ?? []
  const ctrls = u.controllers ?? []
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Usb className="h-3.5 w-3.5" />}
        title="USB"
        suffix={
          <span
            className={cn(
              'rounded-full border px-2 py-0.5 text-[11px] font-medium',
              u.arbitrator_running
                ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                : 'border-border bg-muted text-muted-foreground',
            )}
            title="usbarbitrator 服务"
          >
            arbitrator {u.arbitrator_running ? '运行中' : '已停止'}
          </span>
        }
      />
      <div className="space-y-2">
        {ctrls.length > 0 && (
          <div>
            <div className="mb-1 text-[10.5px] uppercase tracking-wide text-muted-foreground">控制器</div>
            <div className="grid grid-cols-1 gap-1 sm:grid-cols-2">
              {ctrls.map((c) => (
                <div
                  key={c.pci_addr}
                  className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-2 py-1"
                >
                  <span className="shrink-0 font-mono text-[10.5px] text-muted-foreground">{c.pci_addr}</span>
                  <span className="min-w-0 flex-1 truncate text-[11.5px] text-foreground" title={c.name}>
                    {c.name}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
        {(owned.length > 0 || avail.length > 0) && (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div>
              <div className="mb-1 text-[10.5px] uppercase tracking-wide text-muted-foreground">
                VM 已直通({owned.length})
              </div>
              {owned.length > 0 ? (
                <div className="space-y-1">
                  {owned.map((d, i) => (
                    <div
                      key={`${d.vm_id}-${d.label}-${i}`}
                      className="rounded-md border border-border/60 bg-muted/30 px-2 py-1"
                    >
                      <div className="flex min-w-0 items-center gap-1.5 text-[11.5px]">
                        <span className="min-w-0 truncate font-medium text-foreground" title={d.vm_name || `VM ${d.vm_id}`}>
                          {d.vm_name || `VM ${d.vm_id}`}
                        </span>
                        <span className="rounded bg-muted px-1 py-0.5 font-mono text-[10px] text-muted-foreground">
                          {d.label}
                        </span>
                        <span className="min-w-0 truncate font-mono text-[10.5px] text-muted-foreground" title={`path:${d.path}`}>
                          path:{d.path}
                        </span>
                      </div>
                      {d.summary && (
                        <div className="mt-0.5 truncate text-[11px] text-muted-foreground" title={d.summary}>
                          {d.summary}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="rounded-md border border-dashed border-border/60 px-2 py-2 text-center text-[11.5px] text-muted-foreground">
                  暂无
                </p>
              )}
            </div>
            <div>
              <div className="mb-1 text-[10.5px] uppercase tracking-wide text-muted-foreground">
                可直通({avail.length})
              </div>
              {avail.length > 0 ? (
                <div className="space-y-1">
                  {avail.map((d, i) => (
                    <div
                      key={`${d.bus}-${d.dev}-${i}`}
                      className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-2 py-1"
                    >
                      <span className="shrink-0 font-mono text-[10.5px] text-muted-foreground">
                        {d.bus}:{d.dev}
                      </span>
                      <span className="shrink-0 font-mono text-[10.5px] text-muted-foreground">
                        {d.vid}:{d.pid}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-[11.5px] text-foreground" title={d.name}>
                        {d.name}
                      </span>
                      {!d.enabled && (
                        <span className="shrink-0 rounded bg-muted px-1 py-0.5 text-[10px] text-muted-foreground">
                          已禁用
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="rounded-md border border-dashed border-border/60 px-2 py-2 text-center text-[11.5px] text-muted-foreground">
                  暂无
                </p>
              )}
            </div>
          </div>
        )}
        {ctrls.length === 0 && owned.length === 0 && avail.length === 0 && (
          <p className="py-2 text-center text-[11.5px] text-muted-foreground">未拿到 USB 信息</p>
        )}
      </div>
    </Card>
  )
}

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Settings2,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  Server,
  Cpu,
  HardDrive,
  Usb,
  Box,
  ShieldCheck,
  ShieldAlert,
  Activity,
  Network,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { EsxiHostsDrawer } from './esxi/EsxiHostsDrawer'
import { EsxiHostEditDialog } from './esxi/EsxiHostEditDialog'
import { EsxiCredentialsDrawer } from './esxi/EsxiCredentialsDrawer'
import { EsxiCredentialEditDialog } from './esxi/EsxiCredentialEditDialog'
import { NetTopologyFlow } from './esxi/NetTopologyFlow'
import { HistorySection } from './esxi/history/HistorySection'
import {
  diskStatusPill,
  diskUsageInfo,
  extractErr,
  fmtBitrate,
  fmtBytes,
  fmtBytesWithZero,
  fmtDateTime,
  fmtFreq,
  fmtKB,
  fmtStaleAge,
  fmtUptime,
  isStaleSample,
  tempTone,
  useNowTick,
  vmStatePill,
} from './esxi/format'
import type {
  CPUStatic,
  CPUTemperature,
  DiskHealth,
  EsxiCredential,
  EsxiHost,
  HostBoot,
  MCEHealth,
  MemoryInfo,
  NIC,
  PlatformInfo,
  Snapshot,
  USBState,
  VM,
} from './esxi/types'
import { EmptyCard, KV, SectionHead } from './esxi/ui'

// --- 子卡片 ---

function PlatformCard({ p, m, boot }: { p: PlatformInfo; m?: MemoryInfo; boot?: HostBoot }) {
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

function NICsCard({ nics }: { nics: NIC[] }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Network className="h-3.5 w-3.5" />}
        title="网卡"
        suffix={
          <span className="rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
            {nics.length} 张
          </span>
        }
      />
      <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
        {nics.map((n) => {
          const linkUp = n.link_status === 'Up'
          const adminUp = n.admin_status === 'Up'
          const linkTone = linkUp
            ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
            : 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300'
          type Pill = { label: string; tone: 'red' | 'amber' }
          const pills: Pill[] = []
          if (n.rx_errors > 0) pills.push({ label: `收错 ${n.rx_errors}`, tone: 'red' })
          if (n.tx_errors > 0) pills.push({ label: `发错 ${n.tx_errors}`, tone: 'red' })
          if (n.rx_dropped > 0) pills.push({ label: `收丢 ${n.rx_dropped}`, tone: 'amber' })
          if (n.tx_dropped > 0) pills.push({ label: `发丢 ${n.tx_dropped}`, tone: 'amber' })
          const pillCls: Record<Pill['tone'], string> = {
            red: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300',
            amber: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
          }
          return (
            <div key={n.name} className="rounded-md border border-border bg-muted/30 px-2.5 py-2">
              <div className="flex items-start gap-2">
                <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                  <span className="font-mono text-[12px] font-medium text-foreground">{n.name}</span>
                  <span className={cn('rounded-md border px-1.5 py-0.5 text-[10.5px] font-medium', linkTone)}>
                    {linkUp ? '链路 Up' : '链路 Down'}
                  </span>
                  {linkUp ? (
                    <span className="rounded-md border border-border bg-background px-1.5 py-0.5 text-[10.5px] text-muted-foreground">
                      {fmtBitrate(n.speed_mbps)}
                      {n.duplex ? ` · ${n.duplex}` : ''}
                    </span>
                  ) : null}
                  {!adminUp ? (
                    <span className="rounded-md border border-amber-500/40 bg-amber-500/10 px-1.5 py-0.5 text-[10.5px] text-amber-700 dark:text-amber-300">
                      Admin Down
                    </span>
                  ) : null}
                  {pills.map((p) => (
                    <span
                      key={p.label}
                      className={cn('rounded-md border px-1.5 py-0.5 text-[10.5px]', pillCls[p.tone])}
                    >
                      {p.label}
                    </span>
                  ))}
                </div>
                {n.description ? (
                  <span
                    className="min-w-0 max-w-[55%] truncate text-right text-[11px] text-muted-foreground"
                    title={n.description}
                  >
                    {n.description}
                  </span>
                ) : null}
              </div>
              <div className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-0.5 text-[11px] text-muted-foreground">
                <span>MAC <span className="font-mono text-foreground">{n.mac || '—'}</span></span>
                <span>驱动 <span className="text-foreground">{n.driver || '—'}</span></span>
                <span>收 <span className="text-foreground">{n.rx_bytes >= 0 ? fmtBytes(n.rx_bytes) : '—'}</span></span>
                <span>发 <span className="text-foreground">{n.tx_bytes >= 0 ? fmtBytes(n.tx_bytes) : '—'}</span></span>
              </div>
            </div>
          )
        })}
      </div>
    </Card>
  )
}

function CPUStaticCard({ c }: { c: CPUStatic }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Cpu className="h-3.5 w-3.5" />}
        title="CPU"
        suffix={
          c.brand ? (
            <span
              className="min-w-0 truncate text-[11.5px] font-medium text-muted-foreground"
              title={c.brand}
            >
              {c.brand}
            </span>
          ) : undefined
        }
      />
      <div className="grid grid-cols-2 gap-2.5">
        <KV k="核数" v={c.cores > 0 ? c.cores : '—'} />
        <KV k="主频" v={fmtFreq(c.freq_mhz)} />
        <KV k="L2 缓存" v={fmtKB(c.l2_kb)} />
        <KV k="L3 缓存" v={fmtKB(c.l3_kb)} />
        <KV
          k="Family / Model / Step"
          v={`${c.family || '—'} / ${c.model || '—'} / ${c.stepping || '—'}`}
          mono
        />
        <KV
          k="TjMax"
          v={c.tjmax_c > 0 ? `${c.tjmax_c}°C` : '—'}
        />
      </div>
    </Card>
  )
}

function CPUTempCard({ t }: { t: CPUTemperature }) {
  const tjmax = t.tjmax_c > 0 ? t.tjmax_c : 100
  const cores = t.cores ?? []
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Activity className="h-3.5 w-3.5" />}
        title="CPU 温度"
        suffix={
          cores.length > 0 ? (
            <span className="text-[11px] text-muted-foreground">{cores.length} 核</span>
          ) : undefined
        }
      />
      {cores.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">未拿到 CPU 温度(vsish MSR 不可读)</p>
      ) : (
        <div className="space-y-2.5">
          {cores.map((c) => {
            const tone = tempTone(c.temp_c, c.headroom_c)
            const pct = Math.max(0, Math.min(100, (c.temp_c / tjmax) * 100))
            return (
              <div key={c.id} className="flex items-center gap-2">
                <span className="w-12 shrink-0 text-[11px] tabular-nums text-muted-foreground">核 {c.id}</span>
                <div className="relative h-2.5 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn('absolute inset-y-0 left-0 rounded-full transition-all', tone.bar)}
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <span className={cn('w-12 shrink-0 text-right text-[12px] font-semibold tabular-nums', tone.text)}>
                  {c.temp_c}°C
                </span>
                <span className="w-12 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">
                  Δ{c.headroom_c}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}

function MCECard({ m }: { m: MCEHealth }) {
  const state = (m.state || '').toLowerCase()
  const tone =
    state === 'green'
      ? { pill: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300', dot: 'bg-emerald-500', icon: ShieldCheck, label: '健康' }
      : state === 'yellow'
        ? { pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300', dot: 'bg-amber-500', icon: ShieldAlert, label: '警告' }
        : state === 'red'
          ? { pill: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300', dot: 'bg-rose-500', icon: ShieldAlert, label: '危险' }
          : { pill: 'border-border bg-muted text-muted-foreground', dot: 'bg-muted-foreground/60', icon: ShieldCheck, label: m.state || '未知' }
  const Icon = tone.icon
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Icon className="h-3.5 w-3.5" />}
        title="MCE"
        suffix={
          <span className={cn('rounded-full border px-2 py-0.5 text-[11px] font-medium', tone.pill)}>
            <span className="inline-flex items-center gap-1.5">
              <span className={cn('h-1.5 w-1.5 rounded-full', tone.dot)} />
              {tone.label}
            </span>
          </span>
        }
      />
      <div className="grid grid-cols-2 gap-2.5">
        <KV k="可纠正错误" v={(m.corrected_total ?? 0).toLocaleString()} />
        <KV k="不可纠正" v={
          <span className={m.uncorrected_total > 0 ? 'text-rose-600 dark:text-rose-400' : undefined}>
            {(m.uncorrected_total ?? 0).toLocaleString()}
          </span>
        } />
        <KV k="EWMA / 周期" v={`${m.corrected_ewma ?? 0} / ${m.period_seconds ?? 0}s`} />
      </div>
    </Card>
  )
}

function DisksCard({ disks }: { disks: DiskHealth[] }) {
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<HardDrive className="h-3.5 w-3.5" />}
        title="磁盘"
        suffix={<span className="text-[11px] text-muted-foreground">{disks.length} 块</span>}
      />
      {disks.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">未拿到 SMART 数据</p>
      ) : (
        <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
          {disks.map((d) => {
            const p = diskStatusPill(d.status)
            const usage = diskUsageInfo(d)
            const datastores = d.datastores ?? []
            return (
              <div
                key={d.device}
                className="rounded-md border border-border/60 bg-muted/30 px-2 py-1.5"
              >
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate text-[12px] font-medium text-foreground" title={d.model || d.device}>
                        {d.model || '(无型号)'}
                      </span>
                      {d.type && (
                        <span className="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-[10px] text-muted-foreground">
                          {d.type}
                        </span>
                      )}
                    </div>
                  </div>
                  <span className="shrink-0 text-[13px] font-semibold tabular-nums text-foreground">
                    {d.temp_c >= 0 ? `${d.temp_c}°C` : '—'}
                  </span>
                  <span className={cn('shrink-0 rounded-full border px-1.5 py-0.5 text-[10.5px] font-medium', p.cls)}>
                    {p.label}
                  </span>
                </div>
                <div className="mt-1.5 flex items-center gap-2">
                  <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                    {usage.pct !== null && (
                      <div
                        className="h-full rounded-full bg-sky-500 dark:bg-sky-400"
                        style={{ width: `${usage.pct}%` }}
                      />
                    )}
                  </div>
                  <span
                    className="shrink-0 text-[10.5px] tabular-nums text-muted-foreground"
                    title={
                      usage.pct !== null
                        ? `已用 ${fmtBytesWithZero(usage.used)} / 总 ${fmtBytes(usage.capacity > 0 ? usage.capacity : usage.used + Math.max(0, usage.free))}`
                        : usage.capacity > 0
                          ? `总 ${fmtBytes(usage.capacity)}`
                          : undefined
                    }
                  >
                    {usage.label}
                  </span>
                </div>
                {(() => {
                  const hours = d.smart_power_on_hours ?? -1
                  const cycles = d.smart_power_cycle_count ?? -1
                  const wear = d.smart_media_wearout ?? -1
                  const realloc = d.smart_reallocated_sectors ?? 0
                  const pending = d.smart_pending_sector_realloc ?? 0
                  const uncorr = d.smart_uncorrectable_errors ?? 0
                  const readErr = d.smart_read_error_count ?? 0
                  const health = (d.smart_health ?? '').trim()

                  const facts: string[] = []
                  if (hours >= 0) {
                    facts.push(hours >= 8760 ? `通电 ${(hours / 8760).toFixed(1)}y` : `通电 ${hours}h`)
                  }
                  if (cycles >= 0) facts.push(`开机 ${cycles} 次`)

                  type Pill = { label: string; tone: 'red' | 'amber' | 'green' }
                  const pills: Pill[] = []
                  if (wear >= 0) {
                    const tone: Pill['tone'] = wear >= 80 ? 'green' : wear >= 60 ? 'amber' : 'red'
                    pills.push({ label: `健康 ${wear}%`, tone })
                  }
                  if (realloc > 0) pills.push({ label: `重映射 ${realloc}`, tone: realloc >= 5 ? 'red' : 'amber' })
                  if (pending > 0) pills.push({ label: `待重映射 ${pending}`, tone: 'red' })
                  if (uncorr > 0) pills.push({ label: `不可纠正 ${uncorr}`, tone: 'red' })
                  if (readErr > 0) pills.push({ label: `读错误 ${readErr}`, tone: 'amber' })
                  if (health && health !== 'OK') pills.push({ label: `SMART: ${health}`, tone: 'red' })

                  if (facts.length === 0 && pills.length === 0) return null
                  const pillCls: Record<Pill['tone'], string> = {
                    red: 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300',
                    amber: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
                    green: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
                  }
                  return (
                    <div className="mt-1 flex flex-wrap items-center gap-1 text-[10.5px] text-muted-foreground">
                      {facts.length > 0 && <span className="tabular-nums">{facts.join(' · ')}</span>}
                      {pills.map((p, i) => (
                        <span
                          key={i}
                          className={cn('rounded-full border px-1.5 py-0.5 font-medium tabular-nums', pillCls[p.tone])}
                        >
                          {p.label}
                        </span>
                      ))}
                    </div>
                  )
                })()}
                {datastores.length > 0 && (
                  <div
                    className="mt-1 truncate text-[10.5px] text-muted-foreground"
                    title={datastores.join(', ')}
                  >
                    {datastores.join(', ')}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}

function USBCard({ u }: { u: USBState }) {
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

function VMsCard({ vms }: { vms: VM[] }) {
  const on = vms.filter((v) => v.state === 'powered_on').length
  const sorted = [...vms].sort((a, b) => a.id - b.id)
  return (
    <Card className="px-3 py-3">
      <SectionHead
        icon={<Box className="h-3.5 w-3.5" />}
        title="虚拟机"
        suffix={
          <span className="text-[11px] tabular-nums text-muted-foreground">
            <span className="font-semibold text-emerald-600 dark:text-emerald-400">{on}</span>
            <span className="mx-0.5">/</span>
            <span>{vms.length}</span>
            <span className="ml-1">运行中</span>
          </span>
        }
      />
      {vms.length === 0 ? (
        <p className="py-2 text-center text-[11.5px] text-muted-foreground">没有虚拟机</p>
      ) : (
        <div className="grid grid-cols-1 gap-1 md:grid-cols-2">
          {sorted.map((v) => {
            const p = vmStatePill(v.state)
            return (
              <div key={v.id} className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-2 py-1.5">
                <span className="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-[10px] text-muted-foreground">
                  #{v.id}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[12px] font-medium text-foreground" title={v.name}>
                    {v.name}
                  </div>
                </div>
                <span className={cn('shrink-0 rounded-full border px-2 py-0.5 text-[10.5px] font-medium', p.cls)}>
                  <span className="inline-flex items-center gap-1">
                    <span className={cn('h-1.5 w-1.5 rounded-full', p.dot)} />
                    {p.label}
                  </span>
                </span>
              </div>
            )
          })}
        </div>
      )}
    </Card>
  )
}

// --- 一台主机的整张卡 ---

function HostBlock({ host }: { host: Snapshot }) {
  const [expanded, setExpanded] = useState(false)
  const now = useNowTick()
  const cs = getColorSet('esxi')
  const sampledAtMs = host.sampled_at ? new Date(host.sampled_at).getTime() : 0
  const isStale = !host.reachable || isStaleSample(host.sampled_at, now)
  const staleAge = sampledAtMs > 0 ? fmtStaleAge(now - sampledAtMs) : ''

  return (
    <Card
      className={cn(
        'group transition-[transform,box-shadow,border-color,opacity,filter] duration-500 ease-out hover:-translate-y-0.5',
        cs.border,
        cs.halo,
        isStale && 'opacity-70 saturate-50',
      )}
    >
      <div className="p-4 sm:p-5">
        {/* 头部 */}
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <Server className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-[15px] font-semibold tracking-tight">{host.host_name}</span>
            <span className="truncate font-mono text-[11.5px] text-muted-foreground">{host.endpoint}</span>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {host.sampled_at && (
              <span className="hidden text-[11.5px] text-muted-foreground sm:inline">
                最近采样 {fmtDateTime(host.sampled_at)}
              </span>
            )}
            {isStale ? (
              <span
                className="rounded-full border border-border bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
                title={host.error ? `失败:${host.error}` : '尚未拿到采样'}
              >
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/60" />
                  {host.error ? '采样失败' : '已离线'}
                  {staleAge && ` · ${staleAge}`}
                </span>
              </span>
            ) : (
              <span className="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                  在线
                </span>
              </span>
            )}
          </div>
        </div>

        {host.error && (
          <div className="mt-3 flex items-start gap-2 rounded-md border border-rose-500/30 bg-rose-500/5 px-3 py-2 text-[11.5px] text-rose-700 dark:text-rose-300">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span className="break-all">{host.error}</span>
          </div>
        )}

        {/* 卡片网格:永远渲染所有模块,缺数据的格子显示占位,避免子模块整块消失。
            完全没有任何模块数据(首次采样前)时退化成一个大的提示框。 */}
        {(host.platform || host.cpu_static || host.memory || host.cpu_temperature || host.mce_health ||
          (host.disk_health && host.disk_health.length > 0) || host.usb || host.vms ||
          host.boot || (host.nics && host.nics.length > 0)) ? (
          <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
            {host.platform
              ? <PlatformCard p={host.platform} m={host.memory} boot={host.boot} />
              : <EmptyCard icon={<Server className="h-3.5 w-3.5" />} title="平台" />}
            {host.mce_health
              ? <MCECard m={host.mce_health} />
              : <EmptyCard icon={<ShieldCheck className="h-3.5 w-3.5" />} title="MCE" />}
            {host.cpu_static
              ? <CPUStaticCard c={host.cpu_static} />
              : <EmptyCard icon={<Cpu className="h-3.5 w-3.5" />} title="CPU" />}
            {host.cpu_temperature
              ? <CPUTempCard t={host.cpu_temperature} />
              : <EmptyCard icon={<Activity className="h-3.5 w-3.5" />} title="CPU 温度" />}
            <div className="md:col-span-2">
              {host.disk_health && host.disk_health.length > 0
                ? <DisksCard disks={host.disk_health} />
                : <EmptyCard icon={<HardDrive className="h-3.5 w-3.5" />} title="磁盘" />}
            </div>
            <div className="md:col-span-2">
              {host.nics && host.nics.length > 0
                ? <NICsCard nics={host.nics} />
                : <EmptyCard icon={<Network className="h-3.5 w-3.5" />} title="网卡" />}
            </div>
            {host.net_topology &&
              ((host.net_topology.vswitches?.length ?? 0) > 0 ||
                (host.net_topology.vm_nics?.length ?? 0) > 0 ||
                (host.net_topology.vmk_ports?.length ?? 0) > 0) ? (
              <div className="md:col-span-2">
                <NetTopologyFlow topo={host.net_topology} nics={host.nics} vms={host.vms} />
              </div>
            ) : null}
            <div className="md:col-span-2">
              {host.usb
                ? <USBCard u={host.usb} />
                : <EmptyCard icon={<Usb className="h-3.5 w-3.5" />} title="USB" />}
            </div>
            {/* 虚拟机数量多时网格会被拉得不对称(USB 列被拉很长),让它独占一整行 */}
            <div className="md:col-span-2">
              {host.vms
                ? <VMsCard vms={host.vms} />
                : <EmptyCard icon={<Box className="h-3.5 w-3.5" />} title="虚拟机" />}
            </div>
          </div>
        ) : (
          !host.error && (
            <div className="mt-4 rounded-md border border-dashed border-border py-6 text-center text-[12px] text-muted-foreground">
              尚未采集到数据,等待首次采样
            </div>
          )
        )}

        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="mt-3 flex w-full items-center justify-center gap-1 rounded-md border border-dashed border-border py-1.5 text-[11.5px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          {expanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
          {expanded ? '收起历史' : '查看历史'}
        </button>
        {expanded && (
          <div className="mt-3">
            <HistorySection hostKind={host.host_kind} hostID={host.host_id} disks={host.disk_health} />
          </div>
        )}
      </div>
    </Card>
  )
}

// --- 主页面 ---

export default function Esxi() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [hostsOpen, setHostsOpen] = useState(false)
  const [hostEditOpen, setHostEditOpen] = useState(false)
  const [editingHost, setEditingHost] = useState<EsxiHost | null>(null)
  const [credsOpen, setCredsOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [editingCred, setEditingCred] = useState<EsxiCredential | null>(null)
  const [hosts, setHosts] = useState<EsxiHost[]>([])
  const [credentials, setCredentials] = useState<EsxiCredential[]>([])

  const normalize = (arr: unknown): Snapshot[] => {
    if (!Array.isArray(arr)) return []
    return arr as Snapshot[]
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/esxi/snapshot')
      setSnapshots(normalize(data?.data))
    } catch (e) {
      toast.error(extractErr(e, '加载失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const loadHosts = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/hosts')
      setHosts(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载机器失败'))
    }
  }, [])

  const loadCredentials = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/credentials')
      setCredentials(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载凭证失败'))
    }
  }, [])

  const openHostsDrawer = useCallback(() => {
    setHostsOpen(true)
    void loadHosts()
    void loadCredentials()
  }, [loadHosts, loadCredentials])

  const openCredsDrawer = useCallback(() => {
    setCredsOpen(true)
    void loadCredentials()
  }, [loadCredentials])

  const onAddHost = () => {
    setEditingHost(null)
    setHostEditOpen(true)
  }
  const onEditHost = (h: EsxiHost) => {
    setEditingHost(h)
    setHostEditOpen(true)
  }
  const onDeleteHost = async (h: EsxiHost) => {
    if (!window.confirm(`确认删除 ESXi 机器「${h.name}」?`)) return
    try {
      await api.delete(`/esxi/hosts/${h.id}`)
      toast.success('已删除')
      void loadHosts()
      void load()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }
  const onTestHost = async (h: EsxiHost) => {
    try {
      const { data } = await api.post(`/esxi/hosts/${h.id}/test`)
      const r = data?.data
      if (r?.ok) {
        toast.success(r.summary ? `连通成功 · ${r.summary}` : '连通成功')
      } else {
        toast.error(r?.error || '连通失败')
      }
    } catch (e) {
      toast.error(extractErr(e, '测试失败'))
    }
  }

  const onAddCredential = () => {
    setEditingCred(null)
    setCredEditOpen(true)
  }
  const onEditCredential = (c: EsxiCredential) => {
    setEditingCred(c)
    setCredEditOpen(true)
  }
  const onDeleteCredential = async (c: EsxiCredential) => {
    if (!window.confirm(`确认删除 ESXi 凭证「${c.name}」?`)) return
    try {
      await api.delete(`/esxi/credentials/${c.id}`)
      toast.success('已删除')
      void loadCredentials()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }

  const triggerSample = useCallback(async () => {
    setRefreshing(true)
    try {
      const { data } = await api.post('/esxi/refresh')
      setSnapshots(normalize(data?.data))
      toast.success('已触发一次采样')
    } catch (e) {
      toast.error(extractErr(e, '采样失败'))
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => {
      void load()
    })
  }, [load])

  useEffect(() => {
    const es = new EventSource('/api/esxi/stream')
    es.addEventListener('snapshot', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data)
        setSnapshots(normalize(data))
        setLoading(false)
      } catch {
        // 单帧损坏忽略
      }
    })
    return () => es.close()
  }, [])

  const cs = getColorSet('esxi')
  const empty = !loading && snapshots.length === 0
  const now = useNowTick()

  const stats = useMemo(() => {
    const hostCnt = snapshots.length
    let onlineHosts = 0
    let totalVMs = 0
    let runningVMs = 0
    let cpuPeak = -1
    for (const s of snapshots) {
      if (s.reachable && !isStaleSample(s.sampled_at, now)) onlineHosts++
      if (s.vms) {
        totalVMs += s.vms.length
        for (const v of s.vms) {
          if (v.state === 'powered_on') runningVMs++
        }
      }
      if (s.cpu_temperature?.max_c != null && s.cpu_temperature.max_c > cpuPeak) {
        cpuPeak = s.cpu_temperature.max_c
      }
    }
    return { hostCnt, onlineHosts, totalVMs, runningVMs, cpuPeak }
  }, [now, snapshots])

  const lastSampled = useMemo(() => {
    let latest = ''
    for (const s of snapshots) {
      if (s.sampled_at && (!latest || s.sampled_at > latest)) latest = s.sampled_at
    }
    return latest
  }, [snapshots])

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} aria-hidden />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">ESXi 状态</h1>
          </div>
          {stats.hostCnt > 1 && (
            <p className="mt-2 text-[12.5px] text-muted-foreground">
              {stats.hostCnt} 台机器
              {stats.onlineHosts < stats.hostCnt && (
                <span className="ml-2 inline-flex items-center gap-1 text-amber-600 dark:text-amber-400">
                  <AlertTriangle className="h-3 w-3" />
                  {stats.hostCnt - stats.onlineHosts} 台离线
                </span>
              )}
              {stats.totalVMs > 0 && (
                <span className="ml-2">· {stats.runningVMs} / {stats.totalVMs} VM 运行中</span>
              )}
              {stats.cpuPeak >= 0 && (
                <span className="ml-2">· CPU 峰值 {stats.cpuPeak}°C</span>
              )}
              {lastSampled && (
                <span className="ml-2 text-muted-foreground/70">· 最近采样 {fmtDateTime(lastSampled)}</span>
              )}
            </p>
          )}
        </div>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 sm:flex-none"
            onClick={triggerSample}
            disabled={refreshing}
          >
            {refreshing ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            )}
            立即采样
          </Button>
          <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={openHostsDrawer}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            ESXi 机器
          </Button>
        </div>
      </div>

      {loading ? (
        <Card className="px-4 py-16 text-center text-[12.5px] text-muted-foreground">
          <Loader2 className="mx-auto mb-2 h-4 w-4 animate-spin" />
          加载中
        </Card>
      ) : empty ? (
        <Card className="space-y-3 px-6 py-10 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <Server className="h-5 w-5 text-muted-foreground" />
          </div>
          <div className="text-[14px] font-medium">还没有 ESXi 机器</div>
          <p className="mx-auto max-w-md text-[12.5px] text-muted-foreground">
            点右上「ESXi 机器」新增要采样的主机;需先开放 ESXi 的 SSH(默认是关闭的)。
          </p>
          <Button variant="outline" size="sm" onClick={openHostsDrawer}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            打开 ESXi 机器
          </Button>
        </Card>
      ) : (
        <div className="space-y-6">
          {snapshots.map((s) => (
            <HostBlock key={`${s.host_kind}-${s.host_id}`} host={s} />
          ))}
        </div>
      )}

      <EsxiHostsDrawer
        open={hostsOpen}
        onOpenChange={setHostsOpen}
        hosts={hosts}
        onAdd={onAddHost}
        onEdit={onEditHost}
        onDelete={onDeleteHost}
        onTest={onTestHost}
        onManageCredentials={openCredsDrawer}
      />
      <EsxiHostEditDialog
        open={hostEditOpen}
        onOpenChange={setHostEditOpen}
        target={editingHost}
        hosts={hosts}
        credentials={credentials}
        onManageCredentials={openCredsDrawer}
        onSaved={() => {
          void loadHosts()
          void load()
        }}
      />
      <EsxiCredentialsDrawer
        open={credsOpen}
        onOpenChange={setCredsOpen}
        credentials={credentials}
        onAdd={onAddCredential}
        onEdit={onEditCredential}
        onDelete={onDeleteCredential}
      />
      <EsxiCredentialEditDialog
        open={credEditOpen}
        onOpenChange={setCredEditOpen}
        target={editingCred}
        onSaved={loadCredentials}
      />
    </div>
  )
}

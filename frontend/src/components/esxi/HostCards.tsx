import { Activity, Box, Cpu, HardDrive, Network, Server, ShieldAlert, ShieldCheck, Usb } from 'lucide-react'

import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import { diskStatusPill, diskUsageInfo, fmtBitrate, fmtBytes, fmtBytesWithZero, fmtDateTime, fmtFreq, fmtKB, fmtUptime, tempTone, vmStatePill } from './format'
import type { CPUStatic, CPUTemperature, DiskHealth, HostBoot, MCEHealth, MemoryInfo, NIC, PlatformInfo, USBState, VM } from './types'
import { KV, SectionHead } from './ui'

// --- 子卡片 ---

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

export function NICsCard({ nics }: { nics: NIC[] }) {
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

export function CPUStaticCard({ c }: { c: CPUStatic }) {
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

export function CPUTempCard({ t }: { t: CPUTemperature }) {
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

export function MCECard({ m }: { m: MCEHealth }) {
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

export function DisksCard({ disks }: { disks: DiskHealth[] }) {
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

export function VMsCard({ vms }: { vms: VM[] }) {
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

import { useState } from 'react'
import { Activity, AlertTriangle, Box, ChevronDown, ChevronUp, Cpu, HardDrive, Network, Server, ShieldCheck, Usb } from 'lucide-react'

import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Card } from '../ui/card'
import { fmtDateTime, fmtStaleAge, isStaleSample, useNowTick } from './format'
import { NetTopologyFlow } from './NetTopologyFlow'
import { HistorySection } from './history/HistorySection'
import type { Snapshot } from './types'
import { EmptyCard } from './ui'
import { CPUStaticCard, CPUTempCard, DisksCard, MCECard, NICsCard, PlatformCard, USBCard, VMsCard } from './HostCards'

// --- 一台主机的整张卡 ---

export function HostBlock({ host }: { host: Snapshot }) {
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

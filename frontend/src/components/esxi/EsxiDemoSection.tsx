import { useMemo } from 'react'
import { AlertTriangle, X } from 'lucide-react'

import { Button } from '../ui/button'
import { fmtDateTime, isStaleSample, useNowTick } from './format'
import { HostBlock } from './HostBlock'
import type { Snapshot } from './types'

// 隐藏式样式演示：主页标题左侧的状态点连点 5 次进入演示模式渲染。
// 数据完全虚构：A / B 两台在线（覆盖各卡片 + 历史曲线生成），C 台离线（采样失败样式）。
export function EsxiDemoSection({ onClose }: { onClose: () => void }) {
  const now = useNowTick()
  const demoHosts = useMemo<Snapshot[]>(
    () => [buildHealthyHost(now), buildLoadedHost(now), buildOfflineHost(now)],
    [now],
  )
  const stats = useMemo(() => computeStats(demoHosts, now), [demoHosts, now])
  const lastSampled = useMemo(() => {
    let latest = ''
    for (const s of demoHosts) {
      if (s.sampled_at && (!latest || s.sampled_at > latest)) latest = s.sampled_at
    }
    return latest
  }, [demoHosts])

  return (
    <div className="mt-10 rounded-2xl border border-dashed border-muted-foreground/30 bg-muted/20 p-4 sm:p-5">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-[14px] font-semibold">样式演示</div>
          <div className="mt-0.5 text-[11.5px] text-muted-foreground">
            非真实数据，含 2 台在线（覆盖各卡片样式 + 历史曲线按当前快照生成）与 1 台离线机器（采样失败样式）。
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
        <Button variant="ghost" size="sm" onClick={onClose}>
          <X className="mr-1 h-3.5 w-3.5" />
          退出演示
        </Button>
      </div>
      <div className="space-y-6">
        {demoHosts.map((h) => (
          <HostBlock key={`${h.host_kind}-${h.host_id}`} host={h} demo />
        ))}
      </div>
    </div>
  )
}

function computeStats(hosts: Snapshot[], now: number) {
  const hostCnt = hosts.length
  let onlineHosts = 0
  let totalVMs = 0
  let runningVMs = 0
  let cpuPeak = -1
  for (const s of hosts) {
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
}

function buildHealthyHost(now: number): Snapshot {
  const sampledAt = new Date(now).toISOString()
  const bootedAt = new Date(now - 32 * 86400_000).toISOString()
  return {
    host_kind: 'demo',
    host_id: -1,
    host_name: '演示机器 A',
    endpoint: 'demo-a:443',
    reachable: true,
    sampled_at: sampledAt,
    platform: {
      vendor: 'Supermicro',
      product: 'X10SDV-TLN4F',
      serial: 'DEMO-SN-0001',
      uuid: '00000000-0000-0000-0000-000000000000',
      ipmi_supported: true,
      esxi_version: '8.0.3',
      esxi_build: 24022510,
    },
    cpu_static: {
      brand: 'Intel(R) Xeon(R) D-1541 CPU @ 2.10GHz',
      family: 6,
      model: 86,
      stepping: 4,
      cores: 8,
      freq_mhz: 2100,
      l2_kb: 2048,
      l3_kb: 12288,
      tjmax_c: 100,
    },
    memory: {
      mem_total_bytes: 64 * 1024 ** 3,
      mem_free_bytes: 18 * 1024 ** 3,
    },
    runtime_usage: {
      cpu_used_mhz: 4480,
      cpu_capacity_mhz: 16800,
      cpu_usage_percent: 26.7,
      memory_used_bytes: 46 * 1024 ** 3,
      memory_total_bytes: 64 * 1024 ** 3,
      memory_usage_percent: 71.9,
    },
    cpu_temperature: {
      tjmax_c: 100,
      max_c: 62,
      avg_c: 55,
      cores: [
        { id: 0, temp_c: 58, headroom_c: 42 },
        { id: 1, temp_c: 55, headroom_c: 45 },
        { id: 2, temp_c: 62, headroom_c: 38 },
        { id: 3, temp_c: 53, headroom_c: 47 },
        { id: 4, temp_c: 51, headroom_c: 49 },
        { id: 5, temp_c: 54, headroom_c: 46 },
        { id: 6, temp_c: 57, headroom_c: 43 },
        { id: 7, temp_c: 52, headroom_c: 48 },
      ],
    },
    mce_health: {
      state: 'ok',
      corrected_total: 0,
      corrected_ewma: 0,
      period_seconds: 86400,
      uncorrected_total: 0,
    },
    disk_health: [
      {
        device: 't10.NVMe____Demo_SSD_1TB',
        model: 'Demo NVMe SSD 1TB',
        type: 'SSD',
        capacity_bytes: 1024 ** 4,
        used_bytes: 612 * 1024 ** 3,
        free_bytes: 412 * 1024 ** 3,
        datastores: ['datastore-ssd'],
        temp_c: 48,
        threshold_c: 70,
        status: 'ok',
        smart_health: 'OK',
        smart_power_on_hours: 12450,
        smart_power_cycle_count: 38,
        smart_reallocated_sectors: 0,
        smart_uncorrectable_errors: 0,
        smart_media_wearout: 4,
        smart_read_error_count: 0,
        smart_pending_sector_realloc: 0,
      },
      {
        device: 't10.ATA_____Demo_HDD_4TB',
        model: 'Demo HDD 4TB',
        type: 'HDD',
        capacity_bytes: 4 * 1024 ** 4,
        used_bytes: 3.1 * 1024 ** 4,
        free_bytes: 0.9 * 1024 ** 4,
        datastores: ['datastore-hdd'],
        temp_c: 56,
        threshold_c: 60,
        status: 'warning',
        smart_health: 'OK',
        smart_power_on_hours: 38211,
        smart_power_cycle_count: 142,
        smart_reallocated_sectors: 2,
        smart_uncorrectable_errors: 0,
        smart_media_wearout: 0,
        smart_read_error_count: 1,
        smart_pending_sector_realloc: 0,
      },
    ],
    usb: {
      controllers: [{ pci_addr: '0000:00:14.0', name: 'Intel xHCI Controller' }],
      arbitrator_running: true,
      available_for_passthrough: [
        { bus: 1, dev: 3, vid: '0bda', pid: '8153', name: 'Realtek USB GbE', enabled: false },
      ],
      vm_owned: [
        {
          vm_id: 12,
          vm_name: 'home-assistant',
          label: 'USB Zigbee Dongle',
          summary: 'SONOFF ZBDongle-E',
          path: 'usb.10001',
        },
      ],
    },
    vms: [
      { id: 10, name: 'router-openwrt', guest_os: 'Linux', state: 'powered_on' },
      { id: 11, name: 'nas-truenas', guest_os: 'FreeBSD', state: 'powered_on' },
      { id: 12, name: 'home-assistant', guest_os: 'Linux', state: 'powered_on' },
      { id: 13, name: 'k3s-master', guest_os: 'Linux', state: 'powered_on' },
      { id: 14, name: 'windows-test', guest_os: 'Windows', state: 'powered_off' },
      { id: 15, name: 'pbs-backup', guest_os: 'Linux', state: 'suspended' },
    ],
    boot: {
      uptime_seconds: 32 * 86400 + 4 * 3600 + 17 * 60,
      booted_at: bootedAt,
      crash_dump_count: 0,
    },
    nics: [
      {
        name: 'vmnic0',
        driver: 'ixgben',
        mac: '0c:c4:7a:00:00:01',
        mtu: 1500,
        description: 'Intel X552 10G',
        admin_status: 'up',
        link_status: 'up',
        speed_mbps: 10000,
        duplex: 'full',
        rx_bytes: 1.2e12,
        tx_bytes: 8.4e11,
        rx_errors: 0,
        tx_errors: 0,
        rx_dropped: 12,
        tx_dropped: 0,
      },
      {
        name: 'vmnic1',
        driver: 'igbn',
        mac: '0c:c4:7a:00:00:02',
        mtu: 1500,
        description: 'Intel I350 1G',
        admin_status: 'up',
        link_status: 'down',
        speed_mbps: 0,
        duplex: '',
        rx_bytes: 0,
        tx_bytes: 0,
        rx_errors: 0,
        tx_errors: 0,
        rx_dropped: 0,
        tx_dropped: 0,
      },
    ],
    net_topology: {
      vswitches: [
        { name: 'vSwitch0', uplinks: ['vmnic0'], portgroups: ['Management Network', 'VM Network'] },
        { name: 'vSwitch1', uplinks: ['vmnic1'], portgroups: ['VLAN-IoT'] },
      ],
      vm_nics: [
        { vmid: 10, vm_name: 'router-openwrt', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:01:00:0a', ip: '192.168.1.1', team_uplink: 'vmnic0' },
        { vmid: 11, vm_name: 'nas-truenas', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:01:00:0b', ip: '192.168.1.10', team_uplink: 'vmnic0' },
        { vmid: 12, vm_name: 'home-assistant', vswitch: 'vSwitch1', portgroup: 'VLAN-IoT', mac: '00:50:56:01:00:0c', ip: '192.168.20.5', team_uplink: 'vmnic1' },
        { vmid: 13, vm_name: 'k3s-master', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:01:00:0d', ip: '192.168.1.20', team_uplink: 'vmnic0' },
      ],
      vmk_ports: [
        { name: 'vmk0', vswitch: 'vSwitch0', portgroup: 'Management Network', mac: '0c:c4:7a:00:00:01', ipv4: '192.168.1.2', enabled: true },
      ],
    },
  }
}

function buildLoadedHost(now: number): Snapshot {
  const sampledAt = new Date(now).toISOString()
  const bootedAt = new Date(now - 81 * 86400_000).toISOString()
  return {
    host_kind: 'demo',
    host_id: -2,
    host_name: '演示机器 B',
    endpoint: 'demo-b:443',
    reachable: true,
    sampled_at: sampledAt,
    platform: {
      vendor: 'Dell Inc.',
      product: 'PowerEdge R740xd',
      serial: 'DEMO-SN-0002',
      uuid: '11111111-1111-1111-1111-111111111111',
      ipmi_supported: true,
      esxi_version: '7.0.3',
      esxi_build: 21686933,
    },
    cpu_static: {
      brand: 'Intel(R) Xeon(R) Silver 4210 CPU @ 2.20GHz',
      family: 6,
      model: 85,
      stepping: 7,
      cores: 20,
      freq_mhz: 2200,
      l2_kb: 1024,
      l3_kb: 14080,
      tjmax_c: 95,
    },
    memory: {
      mem_total_bytes: 192 * 1024 ** 3,
      mem_free_bytes: 38 * 1024 ** 3,
    },
    runtime_usage: {
      cpu_used_mhz: 26400,
      cpu_capacity_mhz: 44000,
      cpu_usage_percent: 60.0,
      memory_used_bytes: 154 * 1024 ** 3,
      memory_total_bytes: 192 * 1024 ** 3,
      memory_usage_percent: 80.2,
    },
    cpu_temperature: {
      tjmax_c: 95,
      max_c: 78,
      avg_c: 71,
      cores: [
        { id: 0, temp_c: 74, headroom_c: 21 },
        { id: 1, temp_c: 72, headroom_c: 23 },
        { id: 2, temp_c: 78, headroom_c: 17 },
        { id: 3, temp_c: 70, headroom_c: 25 },
        { id: 4, temp_c: 69, headroom_c: 26 },
        { id: 5, temp_c: 75, headroom_c: 20 },
        { id: 6, temp_c: 71, headroom_c: 24 },
        { id: 7, temp_c: 68, headroom_c: 27 },
        { id: 8, temp_c: 73, headroom_c: 22 },
        { id: 9, temp_c: 70, headroom_c: 25 },
        { id: 10, temp_c: 67, headroom_c: 28 },
        { id: 11, temp_c: 72, headroom_c: 23 },
        { id: 12, temp_c: 69, headroom_c: 26 },
        { id: 13, temp_c: 71, headroom_c: 24 },
        { id: 14, temp_c: 73, headroom_c: 22 },
        { id: 15, temp_c: 70, headroom_c: 25 },
        { id: 16, temp_c: 66, headroom_c: 29 },
        { id: 17, temp_c: 68, headroom_c: 27 },
        { id: 18, temp_c: 75, headroom_c: 20 },
        { id: 19, temp_c: 71, headroom_c: 24 },
      ],
    },
    mce_health: {
      state: 'warning',
      corrected_total: 17,
      corrected_ewma: 0.42,
      period_seconds: 86400,
      uncorrected_total: 0,
    },
    disk_health: [
      {
        device: 't10.NVMe____Demo_PM983_3T84',
        model: 'Samsung PM983 3.84TB',
        type: 'NVMe',
        capacity_bytes: 3.84 * 1024 ** 4,
        used_bytes: 2.6 * 1024 ** 4,
        free_bytes: 1.24 * 1024 ** 4,
        datastores: ['datastore-nvme1'],
        temp_c: 52,
        threshold_c: 70,
        status: 'ok',
        smart_health: 'OK',
        smart_power_on_hours: 24580,
        smart_power_cycle_count: 21,
        smart_reallocated_sectors: 0,
        smart_uncorrectable_errors: 0,
        smart_media_wearout: 8,
        smart_read_error_count: 0,
        smart_pending_sector_realloc: 0,
      },
      {
        device: 't10.SAS_____Demo_HUS726T6T_A',
        model: 'HGST Ultrastar 6TB',
        type: 'HDD',
        capacity_bytes: 6 * 1024 ** 4,
        used_bytes: 5.6 * 1024 ** 4,
        free_bytes: 0.4 * 1024 ** 4,
        datastores: ['datastore-hdd-raid'],
        temp_c: 63,
        threshold_c: 60,
        status: 'critical',
        smart_health: 'OK',
        smart_power_on_hours: 51220,
        smart_power_cycle_count: 89,
        smart_reallocated_sectors: 6,
        smart_uncorrectable_errors: 1,
        smart_media_wearout: 0,
        smart_read_error_count: 4,
        smart_pending_sector_realloc: 2,
      },
      {
        device: 't10.SAS_____Demo_HUS726T6T_B',
        model: 'HGST Ultrastar 6TB',
        type: 'HDD',
        capacity_bytes: 6 * 1024 ** 4,
        used_bytes: 5.4 * 1024 ** 4,
        free_bytes: 0.6 * 1024 ** 4,
        datastores: ['datastore-hdd-raid'],
        temp_c: 55,
        threshold_c: 60,
        status: 'ok',
        smart_health: 'OK',
        smart_power_on_hours: 51220,
        smart_power_cycle_count: 88,
        smart_reallocated_sectors: 0,
        smart_uncorrectable_errors: 0,
        smart_media_wearout: 0,
        smart_read_error_count: 0,
        smart_pending_sector_realloc: 0,
      },
    ],
    usb: {
      controllers: [
        { pci_addr: '0000:00:14.0', name: 'Intel Lewisburg xHCI' },
        { pci_addr: '0000:00:1d.0', name: 'Intel C620 xHCI' },
      ],
      arbitrator_running: false,
      available_for_passthrough: [],
      vm_owned: [],
    },
    vms: [
      { id: 20, name: 'gitea', guest_os: 'Linux', state: 'powered_on' },
      { id: 21, name: 'mysql-primary', guest_os: 'Linux', state: 'powered_on' },
      { id: 22, name: 'mysql-replica', guest_os: 'Linux', state: 'powered_on' },
      { id: 23, name: 'redis-cluster-1', guest_os: 'Linux', state: 'powered_on' },
      { id: 24, name: 'redis-cluster-2', guest_os: 'Linux', state: 'powered_on' },
      { id: 25, name: 'monitoring', guest_os: 'Linux', state: 'powered_on' },
      { id: 26, name: 'jenkins-agent', guest_os: 'Linux', state: 'powered_off' },
      { id: 27, name: 'gpu-train', guest_os: 'Linux', state: 'suspended' },
    ],
    boot: {
      uptime_seconds: 81 * 86400 + 11 * 3600 + 42 * 60,
      booted_at: bootedAt,
      crash_dump_count: 1,
      last_crash_at: new Date(now - 14 * 86400_000).toISOString(),
    },
    nics: [
      {
        name: 'vmnic0',
        driver: 'nmlx5_core',
        mac: 'b8:59:9f:00:00:01',
        mtu: 9000,
        description: 'Mellanox ConnectX-4 40G',
        admin_status: 'up',
        link_status: 'up',
        speed_mbps: 40000,
        duplex: 'full',
        rx_bytes: 7.8e12,
        tx_bytes: 4.2e12,
        rx_errors: 0,
        tx_errors: 0,
        rx_dropped: 38,
        tx_dropped: 0,
      },
      {
        name: 'vmnic1',
        driver: 'nmlx5_core',
        mac: 'b8:59:9f:00:00:02',
        mtu: 9000,
        description: 'Mellanox ConnectX-4 40G',
        admin_status: 'up',
        link_status: 'up',
        speed_mbps: 40000,
        duplex: 'full',
        rx_bytes: 7.4e12,
        tx_bytes: 4.0e12,
        rx_errors: 2,
        tx_errors: 0,
        rx_dropped: 41,
        tx_dropped: 0,
      },
      {
        name: 'vmnic2',
        driver: 'ixgben',
        mac: 'b8:59:9f:00:00:03',
        mtu: 1500,
        description: 'Intel X550 10G',
        admin_status: 'down',
        link_status: 'down',
        speed_mbps: 0,
        duplex: '',
        rx_bytes: 0,
        tx_bytes: 0,
        rx_errors: 0,
        tx_errors: 0,
        rx_dropped: 0,
        tx_dropped: 0,
      },
    ],
    net_topology: {
      vswitches: [
        { name: 'vSwitch0', uplinks: ['vmnic0', 'vmnic1'], portgroups: ['Management Network', 'VM Network', 'VLAN-DB'] },
        { name: 'vSwitch-iso', uplinks: [], portgroups: ['Isolated'] },
      ],
      vm_nics: [
        { vmid: 20, vm_name: 'gitea', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:02:00:14', ip: '10.0.0.20', team_uplink: 'vmnic0' },
        { vmid: 21, vm_name: 'mysql-primary', vswitch: 'vSwitch0', portgroup: 'VLAN-DB', mac: '00:50:56:02:00:15', ip: '10.0.10.21', team_uplink: 'vmnic1' },
        { vmid: 22, vm_name: 'mysql-replica', vswitch: 'vSwitch0', portgroup: 'VLAN-DB', mac: '00:50:56:02:00:16', ip: '10.0.10.22', team_uplink: 'vmnic1' },
        { vmid: 23, vm_name: 'redis-cluster-1', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:02:00:17', ip: '10.0.0.23', team_uplink: 'vmnic0' },
        { vmid: 24, vm_name: 'redis-cluster-2', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:02:00:18', ip: '10.0.0.24', team_uplink: 'vmnic0' },
        { vmid: 25, vm_name: 'monitoring', vswitch: 'vSwitch0', portgroup: 'VM Network', mac: '00:50:56:02:00:19', ip: '10.0.0.25', team_uplink: 'vmnic1' },
        { vmid: 27, vm_name: 'gpu-train', vswitch: 'vSwitch-iso', portgroup: 'Isolated', mac: '00:50:56:02:00:1b', team_uplink: '' },
      ],
      vmk_ports: [
        { name: 'vmk0', vswitch: 'vSwitch0', portgroup: 'Management Network', mac: 'b8:59:9f:00:00:01', ipv4: '10.0.0.2', enabled: true },
        { name: 'vmk1', vswitch: 'vSwitch0', portgroup: 'VLAN-DB', mac: 'b8:59:9f:00:00:11', ipv4: '10.0.10.2', enabled: true },
      ],
    },
  }
}

// 离线机器：reachable=false + 旧 sampled_at + error，HostBlock 渲染失活样式 + 采样失败 pill。
function buildOfflineHost(now: number): Snapshot {
  return {
    host_kind: 'demo',
    host_id: -3,
    host_name: '演示机器 C',
    endpoint: 'demo-c:443',
    reachable: false,
    sampled_at: new Date(now - 47 * 60_000).toISOString(),
    error: 'ssh: handshake timeout',
  }
}

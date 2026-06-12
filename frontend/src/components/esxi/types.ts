// ESXi 模块前端类型,与后端 hostDTO / credentialDTO 对齐。
export interface EsxiHost {
  id: number
  name: string
  endpoint: string
  auth_source: 'inline' | 'credential'
  credential_id: number
  username: string
  auth_type: 'password' | 'key'
  bastion_id: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface EsxiCredential {
  id: number
  name: string
  username: string
  auth_type: 'password' | 'key'
  ref_count: number
  created_at: string
  updated_at: string
}

// HostInput 是 POST / PUT /esxi/hosts 的 body,与 Go 端 esximon.HostInput 一致。
export interface EsxiHostInput {
  id?: number
  name: string
  endpoint: string
  auth_source: 'inline' | 'credential'
  credential_id: number
  username: string
  auth_type: 'password' | 'key'
  password: string
  private_key: string
  passphrase: string
  bastion_id: number
  enabled?: boolean
}

export interface EsxiCredentialInput {
  id?: number
  name: string
  username: string
  auth_type: 'password' | 'key'
  password: string
  private_key: string
  passphrase: string
}

export function authLabel(t: string): string {
  return t === 'key' ? '证书' : '密码'
}

// 后端 snapshot 数据形态,与 Go esximon.Snapshot 对齐。

export interface PlatformInfo {
  vendor: string
  product: string
  serial: string
  uuid: string
  ipmi_supported: boolean
  esxi_version: string
  esxi_build: number
}

export interface CPUStatic {
  brand: string
  family: number
  model: number
  stepping: number
  cores: number
  freq_mhz: number
  l2_kb: number
  l3_kb: number
  tjmax_c: number
}

export interface MemoryInfo {
  mem_total_bytes: number
  mem_free_bytes: number
}

export interface RuntimeUsage {
  cpu_used_mhz: number
  cpu_capacity_mhz: number
  cpu_usage_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  memory_usage_percent: number
}

export interface CPUCore {
  id: number
  temp_c: number
  headroom_c: number
}

export interface CPUTemperature {
  tjmax_c: number
  cores: CPUCore[]
  max_c: number
  avg_c: number
}

export interface MCEHealth {
  state: string
  corrected_total: number
  corrected_ewma: number
  period_seconds: number
  uncorrected_total: number
}

export interface DiskHealth {
  device: string
  model: string
  type: string
  capacity_bytes?: number
  used_bytes?: number
  free_bytes?: number
  datastores?: string[]
  temp_c: number
  threshold_c: number
  status: string
  smart_health?: string
  smart_power_on_hours?: number
  smart_power_cycle_count?: number
  smart_reallocated_sectors?: number
  smart_uncorrectable_errors?: number
  smart_media_wearout?: number
  smart_read_error_count?: number
  smart_pending_sector_realloc?: number
}

export interface USBController {
  pci_addr: string
  name: string
}

export interface USBPassthroughDevice {
  bus: number
  dev: number
  vid: string
  pid: string
  name: string
  enabled: boolean
}

export interface USBVMOwned {
  vm_id: number
  vm_name: string
  label: string
  summary: string
  path: string
}

export interface USBState {
  controllers: USBController[]
  arbitrator_running: boolean
  available_for_passthrough: USBPassthroughDevice[]
  vm_owned: USBVMOwned[]
}

export interface VM {
  id: number
  name: string
  guest_os: string
  state: string
}

export interface Snapshot {
  host_kind: string
  host_id: number
  host_name: string
  endpoint: string
  reachable: boolean
  error?: string
  sampled_at?: string
  platform?: PlatformInfo
  cpu_static?: CPUStatic
  memory?: MemoryInfo
  runtime_usage?: RuntimeUsage
  cpu_temperature?: CPUTemperature
  mce_health?: MCEHealth
  disk_health?: DiskHealth[]
  usb?: USBState
  vms?: VM[]
  boot?: HostBoot
  nics?: NIC[]
  net_topology?: NetTopology
}

export interface HostBoot {
  uptime_seconds: number
  booted_at: string
  crash_dump_count: number
  last_crash_at?: string
}

export interface NIC {
  name: string
  driver: string
  mac: string
  mtu: number
  description: string
  admin_status: string
  link_status: string
  speed_mbps: number
  duplex: string
  rx_bytes: number
  tx_bytes: number
  rx_errors: number
  tx_errors: number
  rx_dropped: number
  tx_dropped: number
}

export interface VSwitchInfo {
  name: string
  uplinks: string[]
  portgroups: string[]
}

export interface VMNICLink {
  vmid: number
  vm_name: string
  vswitch: string
  portgroup: string
  mac: string
  ip?: string
  team_uplink: string
}

export interface VMKPort {
  name: string
  vswitch: string
  portgroup: string
  mac: string
  ipv4?: string
  enabled: boolean
}

export interface NetTopology {
  vswitches: VSwitchInfo[]
  vm_nics: VMNICLink[]
  vmk_ports?: VMKPort[]
}

export interface CoreTempPoint {
  id: number
  temp_c: number
}

export interface DiskTempPoint {
  device: string
  temp_c: number
}

export interface DiskUsagePoint {
  device: string
  used_bytes: number
  capacity_bytes: number
}

export interface SeriesPoint {
  bucket_start: string
  cpu_max_c: number
  cpu_avg_c: number
  disk_max_c: number
  cpu_usage_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  memory_usage_percent: number
  mce_corrected_total: number
  mce_uncorrected_total: number
  vm_powered_on: number
  cpu_cores?: CoreTempPoint[]
  disks?: DiskTempPoint[]
  disk_usage?: DiskUsagePoint[]
}

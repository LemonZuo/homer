export type QueryType = 1 | 2
export type SimSlot = 1 | 2

export interface SmsSendForm {
  simSlot: SimSlot
  phones: string
  content: string
}

// auth_mode 与 SmsForwarder Android「客户端安全措施」一致
export type AuthMode = 0 | 1 | 2 | 3

export const AUTH_MODES: { value: AuthMode; label: string }[] = [
  { value: 0, label: '无（明文）' },
  { value: 1, label: '签名（HmacSHA256）' },
  { value: 2, label: 'RSA' },
  { value: 3, label: 'SM4' },
]

export interface Forwarder {
  id: number
  name: string
  server_url: string
  auth_mode: AuthMode
  sign_key: string
  rsa_public_key: string
  sm4_key: string
  timeout_seconds: number
  enabled: boolean
}

// SmsForwarder /sms/query 返回字段（pppscn/SmsForwarder Wiki 附录2）：
// name / number / content / date(ms) / type(1接收 2发送) / sim_id(0=SIM1 1=SIM2 -1未知) / sub_id
export interface SmsItem {
  name?: string
  number?: string
  content?: string
  date?: number
  type?: number
  sim_id?: number
  sub_id?: number
  [k: string]: any
}

export const LS_KEY = 'sms.forwarder.id'

// /config/query data 部分
export interface DeviceConfig {
  enable_api_battery_query?: boolean
  enable_api_call_query?: boolean
  enable_api_clone?: boolean
  enable_api_contact_add?: boolean
  enable_api_contact_query?: boolean
  enable_api_location?: boolean
  enable_api_sms_query?: boolean
  enable_api_sms_send?: boolean
  enable_api_wol?: boolean
  extra_device_mark?: string
  extra_sim1?: string
  extra_sim2?: string
  sim_info_list?: Record<
    string,
    {
      carrier_name?: string
      country_iso?: string
      icc_id?: string
      number?: string
      sim_slot_index?: number
      subscription_id?: number
    }
  >
  version_code?: number
  version_name?: string
}

export const CAPABILITIES: { key: keyof DeviceConfig; label: string }[] = [
  { key: 'enable_api_sms_send', label: '发短信' },
  { key: 'enable_api_sms_query', label: '查短信' },
  { key: 'enable_api_call_query', label: '查通话' },
  { key: 'enable_api_contact_query', label: '查话簿' },
  { key: 'enable_api_contact_add', label: '加联系人' },
  { key: 'enable_api_battery_query', label: '查电池' },
  { key: 'enable_api_wol', label: '远程开机' },
  { key: 'enable_api_clone', label: '一键克隆' },
  { key: 'enable_api_location', label: '位置' },
]

import type { TypeMeta } from './types'

export const FIELD_LABELS: Record<string, string> = {
  corp_id: '企业 ID (corp_id)',
  agent_id: '应用 ID (agent_id)',
  secret: '应用 Secret',
  tag_id: '标签 ID (tag_id)',
  api_key: 'Resend API Key',
  from: '发件地址 (from)',
  to: '收件地址 (to)',
  url: 'Webhook URL',
  server: 'Server 地址',
  device_key: '设备 Key (device_key)',
  topic: 'Topic',
  token: 'Access Token（可选）',
}

export function parseConfig(s: string): Record<string, string> {
  try {
    const o = JSON.parse(s || '{}')
    return o && typeof o === 'object' ? o : {}
  } catch {
    return {}
  }
}

export function typeLabel(types: TypeMeta[], type: string) {
  return types.find((x) => x.type === type)?.label ?? type
}

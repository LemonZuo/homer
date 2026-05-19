import type { ProviderSchema } from '../types'

const DOMAIN_RE =
  /^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/

export function isValidDomain(s: string): boolean {
  const v = s.trim().toLowerCase()
  if (v.length === 0 || v.length > 253) return false
  return DOMAIN_RE.test(v)
}

// 仅保留后端实际链接的 5 个 DNS provider；新增 provider 时需同步
// internal/acme/lego_client.go 的 newProviderByName。
export const PROVIDER_SCHEMAS: ProviderSchema[] = [
  {
    key: 'alidns',
    label: '阿里云 DNS (alidns)',
    required: ['ALICLOUD_ACCESS_KEY', 'ALICLOUD_SECRET_KEY'],
    optional: ['ALICLOUD_REGION_ID', 'ALICLOUD_SECURITY_TOKEN'],
  },
  {
    key: 'tencentcloud',
    label: '腾讯云 DNS (tencentcloud)',
    required: ['TENCENTCLOUD_SECRET_ID', 'TENCENTCLOUD_SECRET_KEY'],
    optional: ['TENCENTCLOUD_REGION'],
  },
  {
    key: 'dnspod',
    label: 'DNSPod 旧版 (dnspod)',
    required: ['DNSPOD_API_KEY'],
  },
  {
    key: 'huaweicloud',
    label: '华为云 DNS (huaweicloud)',
    required: ['HUAWEICLOUD_ACCESS_KEY_ID', 'HUAWEICLOUD_SECRET_ACCESS_KEY', 'HUAWEICLOUD_REGION'],
  },
  {
    key: 'cloudflare',
    label: 'Cloudflare (cloudflare)',
    required: ['CLOUDFLARE_DNS_API_TOKEN'],
    optional: ['CLOUDFLARE_ZONE_API_TOKEN'],
  },
]

export function getProviderSchema(key: string): ProviderSchema | undefined {
  return PROVIDER_SCHEMAS.find((p) => p.key === key)
}

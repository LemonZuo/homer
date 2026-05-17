import type {
  DeployConfig,
  DeployTarget,
  ProviderSchema,
  SafelineDeployConfig,
  SafelineTarget,
  SSHDeployConfig,
  SSHTarget,
} from './types'

export function fmtDate(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function fmtDateTime(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function daysUntil(s?: string | null): number | null {
  if (!s) return null
  const d = new Date(s)
  if (isNaN(d.getTime())) return null
  return Math.ceil((d.getTime() - Date.now()) / 86400000)
}

export function safeParseJSON(s: string): Record<string, any> {
  try {
    const v = JSON.parse(s || '{}')
    return typeof v === 'object' && v !== null ? v : {}
  } catch {
    return {}
  }
}

export function safeParseEnvs(s: string): Record<string, string> {
  try {
    const v = JSON.parse(s || '{}')
    return typeof v === 'object' && v !== null ? v : {}
  } catch {
    return {}
  }
}

export function parseSSHEndpoint(endpoint: string): { host: string; port: number } {
  const value = endpoint.trim()
  const match = value.match(/^(.*):(\d+)$/)
  if (!match) return { host: value, port: 22 }
  return { host: match[1], port: Number(match[2]) || 22 }
}

export function deployTargetToSSH(t: DeployTarget): SSHTarget {
  const auth = safeParseJSON(t.auth_json)
  const cfg = safeParseJSON(t.config_json)
  const endpoint = parseSSHEndpoint(t.endpoint)
  return {
    id: t.id,
    name: t.name,
    host: endpoint.host,
    port: endpoint.port,
    auth_source: String(auth.auth_source ?? 'inline') === 'credential' ? 'credential' : 'inline',
    credential_id: Number(auth.credential_id ?? 0) || 0,
    username: String(auth.username ?? ''),
    auth_type: String(auth.auth_type ?? 'password'),
    password: String(auth.password ?? ''),
    private_key: String(auth.private_key ?? ''),
    passphrase: String(auth.passphrase ?? ''),
    enabled: t.enabled,
    bastion_target_id: Number(cfg.bastion_target_id ?? 0) || 0,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

export function deployTargetToSafeline(t: DeployTarget): SafelineTarget {
  const auth = safeParseJSON(t.auth_json)
  const cfg = safeParseJSON(t.config_json)
  return {
    id: t.id,
    name: t.name,
    base_url: t.endpoint,
    api_token: String(auth.api_token ?? ''),
    skip_tls_verify: Boolean(cfg.skip_tls_verify),
    enabled: t.enabled,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

export function splitDeployTargets(rows: DeployTarget[]) {
  return {
    ssh: rows.filter((t) => t.kind === 'ssh').map(deployTargetToSSH),
    safeline: rows.filter((t) => t.kind === 'safeline').map(deployTargetToSafeline),
  }
}

export function sshTargetToDeployTarget(t: SSHTarget): DeployTarget {
  const credential = t.auth_source === 'credential'
  const auth = credential
    ? { auth_source: 'credential', credential_id: t.credential_id }
    : {
        auth_source: 'inline',
        username: t.username,
        auth_type: t.auth_type,
        password: t.password,
        private_key: t.private_key,
        passphrase: t.passphrase,
      }
  const cfg = t.bastion_target_id && t.bastion_target_id > 0
    ? { bastion_target_id: t.bastion_target_id }
    : {}
  return {
    id: t.id,
    name: t.name,
    kind: 'ssh',
    endpoint: `${t.host}:${t.port || 22}`,
    auth_json: JSON.stringify(auth),
    config_json: JSON.stringify(cfg),
    enabled: t.enabled,
  }
}

export function safelineTargetToDeployTarget(t: SafelineTarget): DeployTarget {
  return {
    id: t.id,
    name: t.name,
    kind: 'safeline',
    endpoint: t.base_url,
    auth_json: JSON.stringify({ api_token: t.api_token }),
    config_json: JSON.stringify({ skip_tls_verify: t.skip_tls_verify }),
    enabled: t.enabled,
  }
}

export function deployConfigToSSH(c: DeployConfig): SSHDeployConfig {
  const cfg = safeParseJSON(c.config_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    cert_path: String(cfg.cert_path ?? ''),
    key_path: String(cfg.key_path ?? ''),
    chain_path: String(cfg.chain_path ?? ''),
    fullchain_path: String(cfg.fullchain_path ?? ''),
    deploy_command: String(cfg.deploy_command ?? ''),
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
    created_at: c.created_at ?? '',
    updated_at: c.updated_at ?? '',
  }
}

export function deployConfigToSafeline(c: DeployConfig): SafelineDeployConfig {
  const cfg = safeParseJSON(c.config_json)
  const state = safeParseJSON(c.state_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    cert_id: Number(state.cert_id ?? 0) || 0,
    cert_type: Number(cfg.cert_type ?? 2) || 2,
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
    created_at: c.created_at ?? '',
    updated_at: c.updated_at ?? '',
  }
}

export function splitDeployConfigs(rows: DeployConfig[]) {
  return {
    ssh: rows.filter((c) => c.kind === 'ssh').map(deployConfigToSSH),
    safeline: rows.filter((c) => c.kind === 'safeline').map(deployConfigToSafeline),
  }
}

export function sshConfigToDeployConfig(c: SSHDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'ssh',
    name: c.name,
    config_json: JSON.stringify({
      cert_path: c.cert_path,
      key_path: c.key_path,
      chain_path: c.chain_path,
      fullchain_path: c.fullchain_path,
      deploy_command: c.deploy_command,
    }),
    state_json: '{}',
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

export function safelineConfigToDeployConfig(c: SafelineDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'safeline',
    name: c.name,
    config_json: JSON.stringify({ cert_type: c.cert_type || 2 }),
    state_json: JSON.stringify({ cert_id: c.cert_id || 0 }),
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

export const STATUS_STYLE: Record<string, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  failed: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
}

export const STATUS_LABEL: Record<string, string> = {
  pending: '待运行',
  running: '运行中',
  success: '成功',
  failed: '失败',
}

export const KIND_LABEL: Record<string, string> = {
  issue: '签发',
  renew: '续期',
  revoke: '吊销',
  upload_cas: '上传 CAS',
  deploy_ssh: '部署 SSH',
  deploy_safeline: '部署雷池',
}

export const TASK_PAGE_SIZES = [5, 10, 20, 50, 100]
export const TASK_PAGE_SIZE_KEY = 'acme.taskPageSize'

export function readTaskPageSize(): number {
  const v = Number(localStorage.getItem(TASK_PAGE_SIZE_KEY))
  return TASK_PAGE_SIZES.includes(v) ? v : TASK_PAGE_SIZES[0]
}

// 校验 DNS 名：可选 `*.` 通配符前缀 + 至少两段 label，label 不超过 63 字符
const DOMAIN_RE =
  /^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/

export function isValidDomain(s: string): boolean {
  const v = s.trim().toLowerCase()
  if (v.length === 0 || v.length > 253) return false
  return DOMAIN_RE.test(v)
}

export function caLabel(ca: string) {
  switch (ca) {
    case 'letsencrypt':
      return "Let's Encrypt"
    case 'zerossl':
      return 'ZeroSSL'
    case 'custom':
      return '自定义'
    default:
      return ca || '未知'
  }
}

export function authLabel(authType: string) {
  return authType === 'key' ? '证书' : '密码'
}

export function targetByID(targets: SSHTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

export function targetSummary(t?: SSHTarget) {
  if (!t) return 'SSH 机器不存在'
  const who = t.auth_source === 'credential' ? '凭证' : t.username || '未配置用户'
  return `${t.name} · ${who}@${t.host}:${t.port || 22}`
}

export function configTitle(cfg: SSHDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
}

export function configPrimaryPath(cfg: SSHDeployConfig) {
  return cfg.fullchain_path || cfg.cert_path || cfg.key_path || '未配置路径'
}

export function safelineTargetByID(targets: SafelineTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

export function safelineTargetSummary(t?: SafelineTarget) {
  if (!t) return '雷池实例不存在'
  return `${t.name} · ${t.base_url}`
}

export function safelineConfigTitle(cfg: SafelineDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
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

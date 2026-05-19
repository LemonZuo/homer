import type {
  CASDeployConfig,
  CASTarget,
  FnOSDeployConfig,
  FnOSTarget,
  SafelineDeployConfig,
  SafelineTarget,
  SSHDeployConfig,
  SSHTarget,
} from '../types'

export const STATUS_STYLE: Record<string, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  failed: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  retrying: 'bg-orange-500/10 text-orange-600 dark:text-orange-400',
}

export const STATUS_LABEL: Record<string, string> = {
  pending: '待运行',
  running: '运行中',
  success: '成功',
  failed: '失败',
  retrying: '重试中',
}

export const KIND_LABEL: Record<string, string> = {
  issue: '签发',
  renew: '续期',
  revoke: '吊销',
  upload_cas: '上传 CAS',
  deploy_ssh: '部署 SSH',
  deploy_safeline: '部署雷池',
  deploy_upload_cas: '部署 CAS',
  deploy_fnos: '部署 fnOS',
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

export function casTargetByID(targets: CASTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

export function casTargetSummary(t?: CASTarget) {
  if (!t) return '阿里云 CAS 实例不存在'
  return `${t.name} · ${t.access_key_id || '未配置 AK'}`
}

export function casConfigTitle(cfg: CASDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
}

export function fnosTargetByID(targets: FnOSTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

export function fnosTargetSummary(t?: FnOSTarget) {
  if (!t) return 'fnOS 实例不存在'
  const who = t.auth_source === 'credential' ? '凭证' : t.username || '未配置用户'
  return `${t.name} · ${who}@${t.host}:${t.port || 22}`
}

export function fnosConfigTitle(cfg: FnOSDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
}

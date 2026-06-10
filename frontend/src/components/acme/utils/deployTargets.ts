import type {
  CASTarget,
  DeployTarget,
  FnOSTarget,
  SafelineTarget,
  SSHTarget,
} from '../types'
import { safeParseJSON } from './json'

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
    bastion_id: Number(cfg.bastion_id ?? 0) || 0,
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

export function deployTargetToCAS(t: DeployTarget): CASTarget {
  const auth = safeParseJSON(t.auth_json)
  return {
    id: t.id,
    name: t.name,
    access_key_id: String(auth.access_key_id ?? ''),
    access_key_secret: String(auth.access_key_secret ?? ''),
    enabled: t.enabled,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

export function deployTargetToFnOS(t: DeployTarget): FnOSTarget {
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
    bastion_id: Number(cfg.bastion_id ?? 0) || 0,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

export function splitDeployTargets(rows: DeployTarget[]) {
  return {
    ssh: rows.filter((t) => t.kind === 'ssh').map(deployTargetToSSH),
    safeline: rows.filter((t) => t.kind === 'safeline').map(deployTargetToSafeline),
    cas: rows.filter((t) => t.kind === 'upload_cas').map(deployTargetToCAS),
    fnos: rows.filter((t) => t.kind === 'fnos').map(deployTargetToFnOS),
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
  const cfg = t.bastion_id && t.bastion_id > 0
    ? { bastion_id: t.bastion_id }
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

export function casTargetToDeployTarget(t: CASTarget): DeployTarget {
  return {
    id: t.id,
    name: t.name,
    kind: 'upload_cas',
    endpoint: '',
    auth_json: JSON.stringify({
      access_key_id: t.access_key_id,
      access_key_secret: t.access_key_secret,
    }),
    config_json: '{}',
    enabled: t.enabled,
  }
}

export function fnosTargetToDeployTarget(t: FnOSTarget): DeployTarget {
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
  const cfg = t.bastion_id && t.bastion_id > 0
    ? { bastion_id: t.bastion_id }
    : {}
  return {
    id: t.id,
    name: t.name,
    kind: 'fnos',
    endpoint: `${t.host}:${t.port || 22}`,
    auth_json: JSON.stringify(auth),
    config_json: JSON.stringify(cfg),
    enabled: t.enabled,
  }
}

import type {
  CASDeployConfig,
  DeployConfig,
  FnOSDeployConfig,
  SafelineDeployConfig,
  SSHDeployConfig,
} from '../types'
import { safeParseJSON } from './json'

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

export function deployConfigToCAS(c: DeployConfig): CASDeployConfig {
  const state = safeParseJSON(c.state_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    cert_id: Number(state.cert_id ?? 0) || 0,
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
    created_at: c.created_at ?? '',
    updated_at: c.updated_at ?? '',
  }
}

export function deployConfigToFnOS(c: DeployConfig): FnOSDeployConfig {
  const cfg = safeParseJSON(c.config_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    domain_override: String(cfg.domain_override ?? ''),
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
    cas: rows.filter((c) => c.kind === 'upload_cas').map(deployConfigToCAS),
    fnos: rows.filter((c) => c.kind === 'fnos').map(deployConfigToFnOS),
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

export function casConfigToDeployConfig(c: CASDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'upload_cas',
    name: c.name,
    config_json: '{}',
    state_json: JSON.stringify({ cert_id: c.cert_id || 0 }),
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

export function fnosConfigToDeployConfig(c: FnOSDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'fnos',
    name: c.name,
    config_json: JSON.stringify({ domain_override: c.domain_override }),
    state_json: '{}',
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

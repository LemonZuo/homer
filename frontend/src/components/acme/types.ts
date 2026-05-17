export interface Domain {
  id: number
  main_domain: string
  san_domains: string
  account_id: number
  provider: string
  san_providers?: string
  cas_enabled?: boolean
  enabled: boolean
  created_at: string
  updated_at: string
  not_before?: string
  not_after?: string
  cas_cert_id?: number
  cert_status?: string
  revoked_at?: string
  issued_at?: string
}

export interface AcmeAccount {
  id: number
  name: string
  ca: 'letsencrypt' | 'zerossl' | 'custom' | string
  directory_url: string
  email: string
  eab_kid: string
  eab_hmac: string
  enabled: boolean
  created_at: string
  updated_at: string
  ref_count: number
}

export interface Credential {
  id: number
  provider: string
  envs_json: string
  created_at: string
  updated_at: string
  ref_count: number
}

export interface DeployTarget {
  id: number
  name: string
  kind: 'ssh' | 'safeline' | string
  endpoint: string
  auth_json: string
  config_json: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface SSHCredential {
  id: number
  name: string
  username: string
  auth_type: 'password' | 'key' | string
  password: string
  private_key: string
  passphrase: string
  created_at?: string
  updated_at?: string
  ref_count?: number
}

export interface SSHTarget {
  id: number
  name: string
  host: string
  port: number
  auth_source: 'inline' | 'credential' | string
  credential_id: number
  username: string
  auth_type: 'password' | 'key' | string
  password: string
  private_key: string
  passphrase: string
  enabled: boolean
  bastion_target_id?: number
  created_at?: string
  updated_at?: string
}

export interface SSHDeployConfig {
  id: number
  domain_id: number
  target_id: number
  name: string
  cert_path: string
  key_path: string
  chain_path: string
  fullchain_path: string
  deploy_command: string
  auto_deploy: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface DeployConfig {
  id: number
  domain_id: number
  target_id: number
  kind: 'ssh' | 'safeline' | string
  name: string
  config_json: string
  state_json: string
  auto_deploy: boolean
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface SafelineTarget {
  id: number
  name: string
  base_url: string
  api_token: string
  skip_tls_verify: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface SafelineDeployConfig {
  id: number
  domain_id: number
  target_id: number
  name: string
  cert_id: number
  cert_type: number
  auto_deploy: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface Task {
  id: number
  domain_id: number
  main_domain: string
  kind: string
  status: string
  started_at: string
  finished_at: string | null
  log_text: string
  error_msg: string
  attempt?: number
  max_attempt?: number
  next_retry_at?: string | null
}

export interface ProviderSchema {
  key: string
  label: string
  required: string[]
  optional?: string[]
}

export interface EnvPair {
  key: string
  value: string
}

// ESXi 模块前端类型,与后端 hostDTO / credentialDTO 对齐。
export interface EsxiHost {
  id: number
  name: string
  endpoint: string
  auth_source: 'inline' | 'credential'
  credential_id: number
  username: string
  auth_type: 'password' | 'key'
  bastion_host_id: number
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
  bastion_host_id: number
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

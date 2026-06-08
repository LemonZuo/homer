// UPS 模块前端类型,与后端 hostDTO / credentialDTO 对齐。
export interface UpsHost {
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

export interface UpsCredential {
  id: number
  name: string
  username: string
  auth_type: 'password' | 'key'
  ref_count: number
  created_at: string
  updated_at: string
}

// HostInput 是 POST / PUT /ups/hosts 的 body,与 Go 端 upsmon.HostInput 一致。
// 编辑时如果不动认证,可以让 password / private_key 留空(后端只在 inline 模式下校验)。
export interface UpsHostInput {
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

// CredentialInput 是 POST / PUT /ups/credentials 的 body。
export interface UpsCredentialInput {
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

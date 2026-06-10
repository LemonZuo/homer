import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Textarea } from '../../ui/textarea'
import { Label } from '../../ui/label'
import { Switch } from '../../ui/switch'
import { Segmented } from '../../ui/segmented'
import { Select } from '../../ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { FnOSTarget, SSHCredential, SSHTarget } from '../types'
import { sshTargetToDeployTarget } from '../utils'

type AuthMode = 'password' | 'key' | 'credential'

export function SSHTargetEditDialog({
  open,
  onOpenChange,
  target,
  credentials,
  sshTargets,
  fnosTargets,
  onManageCredentials,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SSHTarget | null
  credentials: SSHCredential[]
  sshTargets: SSHTarget[]
  fnosTargets: FnOSTarget[]
  onManageCredentials: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('22')
  const [authMode, setAuthMode] = useState<AuthMode>('password')
  const [credentialID, setCredentialID] = useState(0)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [bastionID, setBastionID] = useState(0)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setHost(target?.host ?? '')
    setPort(String(target?.port || 22))
    if (target?.auth_source === 'credential') {
      setAuthMode('credential')
    } else {
      setAuthMode(target?.auth_type === 'key' ? 'key' : 'password')
    }
    setCredentialID(target?.credential_id ?? 0)
    setUsername(target?.username ?? '')
    setPassword(target?.password ?? '')
    setPrivateKey(target?.private_key ?? '')
    setPassphrase(target?.passphrase ?? '')
    setEnabled(target?.enabled ?? true)
    setBastionID(target?.bastion_id ?? 0)
  }, [open, target])

  const bastionCandidates = [
    ...sshTargets.map((t) => ({ ...t, kind_label: 'SSH' })),
    ...fnosTargets.map((t) => ({ ...t, kind_label: 'fnOS' })),
  ]

  // 单跳：候选必须是 启用 + 不是自己 + 自身不再依赖跳板（避免链）
  const bastionOptions = bastionCandidates
    .filter((t) => t.enabled && t.id !== (target?.id ?? 0) && !(t.bastion_id && t.bastion_id > 0))
    .map((t) => ({ value: t.id, label: `${t.kind_label} · ${t.name} · ${t.host}:${t.port || 22}` }))

  // 当前机器已被别人当跳板？若是，则本机不能再设置自己的跳板，否则链就破了
  const upstreamRef = target?.id
    ? bastionCandidates.find((t) => t.bastion_id === target.id)
    : undefined
  const bastionLocked = !!upstreamRef

  const save = async () => {
    const portNum = Number(port)
    const credential = authMode === 'credential'
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      host: host.trim(),
      port: portNum,
      auth_source: credential ? 'credential' : 'inline',
      credential_id: credential ? credentialID : 0,
      username: credential ? '' : username.trim(),
      auth_type: credential ? 'password' : authMode,
      password: credential ? '' : password.trim(),
      private_key: credential ? '' : privateKey.trim(),
      passphrase: credential ? '' : passphrase.trim(),
      enabled,
      bastion_id: !bastionLocked && bastionID > 0 ? bastionID : 0,
      created_at: target?.created_at ?? '',
      updated_at: target?.updated_at ?? '',
    }
    if (!form.name) {
      toast.error('目标名称必填')
      return
    }
    if (!form.host) {
      toast.error('SSH 主机必填')
      return
    }
    if (!Number.isInteger(portNum) || portNum <= 0 || portNum > 65535) {
      toast.error('SSH 端口无效')
      return
    }
    if (credential) {
      if (!form.credential_id) {
        toast.error('请选择登录凭证')
        return
      }
    } else {
      if (!form.username) {
        toast.error('SSH 用户名必填')
        return
      }
      if (authMode === 'password' && !form.password) {
        toast.error('密码认证需要填写密码')
        return
      }
      if (authMode === 'key' && !form.private_key) {
        toast.error('证书模式需要填写私钥')
        return
      }
    }
    const payload = sshTargetToDeployTarget(form)
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/deploy/targets/${target.id}`, payload)
      } else {
        await api.post('/acme/deploy/targets', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] overflow-y-auto p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{target ? '编辑 SSH 机器' : '新增 SSH 机器'}</DialogTitle>
          <DialogDescription>
            只保存机器连接和认证信息；证书路径和部署命令在每次部署时填写
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-name">目标名称</Label>
            <Input
              id="ssh-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-2">
            <div className="grid gap-1.5 sm:col-span-2">
              <Label htmlFor="ssh-host">主机</Label>
              <Input
                id="ssh-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="example.com / 10.0.0.1"
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="ssh-port">端口</Label>
              <Input
                id="ssh-port"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label>认证方式</Label>
            <Segmented<AuthMode>
              value={authMode}
              onChange={setAuthMode}
              options={[
                { value: 'password', label: '密码模式' },
                { value: 'key', label: '证书模式（私钥）' },
                { value: 'credential', label: '登录凭证' },
              ]}
            />
          </div>
          {authMode === 'credential' ? (
            <div className="grid gap-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="ssh-credential">登录凭证</Label>
                <button
                  type="button"
                  className="text-[11.5px] text-primary hover:underline"
                  onClick={onManageCredentials}
                >
                  管理登录凭证
                </button>
              </div>
              <Select
                id="ssh-credential"
                value={credentialID}
                onChange={setCredentialID}
                placeholder="请选择登录凭证"
                options={credentials.map((c) => ({
                  value: c.id,
                  label: `${c.name}（${c.username} · ${c.auth_type === 'key' ? '证书' : '密码'}）`,
                }))}
              />
              {credentials.length === 0 && (
                <p className="text-[11.5px] text-muted-foreground">
                  还没有登录凭证，点「管理登录凭证」创建
                </p>
              )}
            </div>
          ) : (
            <div className="grid gap-1.5">
              <Label htmlFor="ssh-user">用户名</Label>
              <Input
                id="ssh-user"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          {authMode === 'password' && (
            <div className="grid gap-1.5">
              <Label htmlFor="ssh-password">密码</Label>
              <Input
                id="ssh-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="new-password"
                data-lpignore="true"
                data-1p-ignore="true"
              />
            </div>
          )}
          {authMode === 'key' && (
            <>
              <div className="grid gap-1.5">
                <Label htmlFor="ssh-private-key">私钥</Label>
                <Textarea
                  id="ssh-private-key"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                  autoComplete="off"
                  data-lpignore="true"
                  data-1p-ignore="true"
                  className="min-h-[140px] font-mono text-[11.5px]"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="ssh-passphrase">私钥口令（可选）</Label>
                <Input
                  id="ssh-passphrase"
                  type="password"
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  autoComplete="new-password"
                  data-lpignore="true"
                  data-1p-ignore="true"
                />
              </div>
            </>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-bastion">跳板机（可选）</Label>
            <Select
              id="ssh-bastion"
              value={bastionLocked ? 0 : bastionID}
              onChange={setBastionID}
              placeholder="直连，不经过跳板机"
              disabled={bastionLocked}
              searchable
              searchPlaceholder="按名称 / 主机搜索"
              options={[
                { value: 0, label: '直连，不经过跳板机' },
                ...bastionOptions,
              ]}
            />
            {bastionLocked && (
              <p className="text-[11.5px] text-muted-foreground">
                本机已被 {upstreamRef?.name} 用作跳板，不能再为自己设置跳板
              </p>
            )}
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="ssh-enabled">启用</Label>
            <Switch id="ssh-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter className="[&>button]:flex-1 sm:[&>button]:flex-none">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

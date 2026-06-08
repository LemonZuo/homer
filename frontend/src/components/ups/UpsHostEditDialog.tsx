import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../api'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Textarea } from '../ui/textarea'
import { Label } from '../ui/label'
import { Switch } from '../ui/switch'
import { Segmented } from '../ui/segmented'
import { Select } from '../ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import type { UpsCredential, UpsHost, UpsHostInput } from './types'

type AuthMode = 'password' | 'key' | 'credential'

function splitEndpoint(endpoint: string): { host: string; port: number } {
  const ep = (endpoint ?? '').trim()
  if (!ep) return { host: '', port: 22 }
  const m = ep.match(/^\[(.+)\]:(\d+)$/) // IPv6
  if (m) return { host: m[1], port: Number(m[2]) || 22 }
  const idx = ep.lastIndexOf(':')
  if (idx > 0 && !ep.includes('::')) {
    const host = ep.slice(0, idx)
    const port = Number(ep.slice(idx + 1))
    if (Number.isInteger(port) && port > 0) return { host, port }
  }
  return { host: ep, port: 22 }
}

function joinEndpoint(host: string, port: number): string {
  const h = host.trim()
  if (!h) return ''
  return `${h}:${port}`
}

export function UpsHostEditDialog({
  open,
  onOpenChange,
  target,
  hosts,
  credentials,
  onManageCredentials,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: UpsHost | null
  hosts: UpsHost[]
  credentials: UpsCredential[]
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
    const { host: h, port: p } = splitEndpoint(target?.endpoint ?? '')
    setName(target?.name ?? '')
    setHost(h)
    setPort(String(p || 22))
    if (target?.auth_source === 'credential') {
      setAuthMode('credential')
    } else {
      setAuthMode(target?.auth_type === 'key' ? 'key' : 'password')
    }
    setCredentialID(target?.credential_id ?? 0)
    setUsername(target?.username ?? '')
    setPassword('')
    setPrivateKey('')
    setPassphrase('')
    setEnabled(target?.enabled ?? true)
    setBastionID(target?.bastion_host_id ?? 0)
  }, [open, target])

  // 单跳:候选必须是启用 + 不是自己 + 自身不再依赖跳板(避免链)
  const bastionOptions = hosts
    .filter((t) => t.enabled && t.id !== (target?.id ?? 0) && !(t.bastion_host_id && t.bastion_host_id > 0))
    .map((t) => ({ value: t.id, label: `${t.name} · ${t.endpoint}` }))

  // 当前机器已被别人当跳板?若是,本机不能再设置自己的跳板
  const upstreamRef = target?.id ? hosts.find((t) => t.bastion_host_id === target.id) : undefined
  const bastionLocked = !!upstreamRef

  const save = async () => {
    const portNum = Number(port)
    const isCredential = authMode === 'credential'
    if (!name.trim()) {
      toast.error('机器名称必填')
      return
    }
    if (!host.trim()) {
      toast.error('主机必填')
      return
    }
    if (!Number.isInteger(portNum) || portNum <= 0 || portNum > 65535) {
      toast.error('端口无效')
      return
    }
    if (isCredential) {
      if (!credentialID) {
        toast.error('请选择登录凭证')
        return
      }
    } else {
      if (!username.trim()) {
        toast.error('SSH 用户名必填')
        return
      }
      if (!target?.id) {
        if (authMode === 'password' && !password.trim()) {
          toast.error('密码认证需要填写密码')
          return
        }
        if (authMode === 'key' && !privateKey.trim()) {
          toast.error('证书模式需要填写私钥')
          return
        }
      }
    }

    const payload: UpsHostInput = {
      id: target?.id ?? 0,
      name: name.trim(),
      endpoint: joinEndpoint(host, portNum),
      auth_source: isCredential ? 'credential' : 'inline',
      credential_id: isCredential ? credentialID : 0,
      username: isCredential ? '' : username.trim(),
      auth_type: isCredential ? 'password' : authMode,
      password: isCredential ? '' : password.trim(),
      private_key: isCredential ? '' : privateKey.trim(),
      passphrase: isCredential ? '' : passphrase.trim(),
      bastion_host_id: !bastionLocked && bastionID > 0 ? bastionID : 0,
      enabled,
    }

    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/ups/hosts/${target.id}`, payload)
      } else {
        await api.post('/ups/hosts', payload)
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
          <DialogTitle>{target ? '编辑 UPS 机器' : '新增 UPS 机器'}</DialogTitle>
          <DialogDescription>
            机器要先装好 NUT(nut-client),后端通过 SSH 拨号执行 upsc 拉取状态
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="ups-name">机器名称</Label>
            <Input
              id="ups-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-2">
            <div className="grid gap-1.5 sm:col-span-2">
              <Label htmlFor="ups-host">主机</Label>
              <Input
                id="ups-host"
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
              <Label htmlFor="ups-port">端口</Label>
              <Input
                id="ups-port"
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
                { value: 'key', label: '证书模式(私钥)' },
                { value: 'credential', label: '登录凭证' },
              ]}
            />
          </div>
          {authMode === 'credential' ? (
            <div className="grid gap-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="ups-credential">登录凭证</Label>
                <button
                  type="button"
                  className="text-[11.5px] text-primary hover:underline"
                  onClick={onManageCredentials}
                >
                  管理登录凭证
                </button>
              </div>
              <Select
                id="ups-credential"
                value={credentialID}
                onChange={setCredentialID}
                placeholder="请选择登录凭证"
                options={credentials.map((c) => ({
                  value: c.id,
                  label: `${c.name}(${c.username} · ${c.auth_type === 'key' ? '证书' : '密码'})`,
                }))}
              />
              {credentials.length === 0 && (
                <p className="text-[11.5px] text-muted-foreground">
                  还没有登录凭证,点「管理登录凭证」创建
                </p>
              )}
            </div>
          ) : (
            <div className="grid gap-1.5">
              <Label htmlFor="ups-user">用户名</Label>
              <Input
                id="ups-user"
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
              <Label htmlFor="ups-password">密码</Label>
              <Input
                id="ups-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={target?.id ? '留空不修改' : ''}
                autoComplete="new-password"
                data-lpignore="true"
                data-1p-ignore="true"
              />
            </div>
          )}
          {authMode === 'key' && (
            <>
              <div className="grid gap-1.5">
                <Label htmlFor="ups-private-key">私钥</Label>
                <Textarea
                  id="ups-private-key"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  placeholder={target?.id ? '留空不修改' : '-----BEGIN OPENSSH PRIVATE KEY-----'}
                  autoComplete="off"
                  data-lpignore="true"
                  data-1p-ignore="true"
                  className="min-h-[140px] font-mono text-[11.5px]"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="ups-passphrase">私钥口令(可选)</Label>
                <Input
                  id="ups-passphrase"
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
            <Label htmlFor="ups-bastion">跳板机(可选)</Label>
            <Select
              id="ups-bastion"
              value={bastionLocked ? 0 : bastionID}
              onChange={setBastionID}
              placeholder="直连,不经过跳板机"
              disabled={bastionLocked}
              searchable
              searchPlaceholder="按名称 / 主机搜索"
              options={[
                { value: 0, label: '直连,不经过跳板机' },
                ...bastionOptions,
              ]}
            />
            {bastionLocked && (
              <p className="text-[11.5px] text-muted-foreground">
                本机已被 {upstreamRef?.name} 用作跳板,不能再为自己设置跳板
              </p>
            )}
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="ups-enabled">启用</Label>
            <Switch id="ups-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

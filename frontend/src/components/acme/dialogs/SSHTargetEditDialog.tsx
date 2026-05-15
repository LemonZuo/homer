import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Textarea } from '../../ui/textarea'
import { Label } from '../../ui/label'
import { Switch } from '../../ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { SSHTarget } from '../types'
import { sshTargetToDeployTarget } from '../utils'

export function SSHTargetEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SSHTarget | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('22')
  const [username, setUsername] = useState('')
  const [authType, setAuthType] = useState<'password' | 'key'>('password')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setHost(target?.host ?? '')
    setPort(String(target?.port || 22))
    setUsername(target?.username ?? '')
    setAuthType(target?.auth_type === 'key' ? 'key' : 'password')
    setPassword(target?.password ?? '')
    setPrivateKey(target?.private_key ?? '')
    setPassphrase(target?.passphrase ?? '')
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const portNum = Number(port)
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      host: host.trim(),
      port: portNum,
      username: username.trim(),
      auth_type: authType,
      password: password.trim(),
      private_key: privateKey.trim(),
      passphrase: passphrase.trim(),
      enabled,
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
    if (!form.username) {
      toast.error('SSH 用户名必填')
      return
    }
    if (!Number.isInteger(portNum) || portNum <= 0 || portNum > 65535) {
      toast.error('SSH 端口无效')
      return
    }
    if (authType === 'password' && !form.password) {
      toast.error('密码认证需要填写密码')
      return
    }
    if (authType === 'key' && !form.private_key) {
      toast.error('证书模式需要填写私钥')
      return
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
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-auth">认证方式</Label>
            <select
              id="ssh-auth"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={authType}
              onChange={(e) => setAuthType(e.target.value === 'key' ? 'key' : 'password')}
            >
              <option value="password">密码模式</option>
              <option value="key">证书模式（私钥）</option>
            </select>
          </div>
          {authType === 'password' ? (
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
          ) : (
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

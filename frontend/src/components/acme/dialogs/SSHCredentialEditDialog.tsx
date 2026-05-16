import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Textarea } from '../../ui/textarea'
import { Label } from '../../ui/label'
import { Segmented } from '../../ui/segmented'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { SSHCredential } from '../types'

export function SSHCredentialEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SSHCredential | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [authType, setAuthType] = useState<'password' | 'key'>('password')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setUsername(target?.username ?? '')
    setAuthType(target?.auth_type === 'key' ? 'key' : 'password')
    setPassword(target?.password ?? '')
    setPrivateKey(target?.private_key ?? '')
    setPassphrase(target?.passphrase ?? '')
  }, [open, target])

  const save = async () => {
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      username: username.trim(),
      auth_type: authType,
      password: password.trim(),
      private_key: privateKey.trim(),
      passphrase: passphrase.trim(),
    }
    if (!form.name) {
      toast.error('凭证名称必填')
      return
    }
    if (!form.username) {
      toast.error('登录用户名必填')
      return
    }
    if (authType === 'password' && !form.password) {
      toast.error('密码模式需要填写登录密码')
      return
    }
    if (authType === 'key' && !form.private_key) {
      toast.error('秘钥模式需要填写私钥')
      return
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/ssh-credentials/${target.id}`, form)
      } else {
        await api.post('/acme/ssh-credentials', form)
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
          <DialogTitle>{target ? '编辑登录凭证' : '创建登录凭证'}</DialogTitle>
          <DialogDescription>
            凭证保存一份登录身份，可被多台 SSH 机器复用
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="cred-name">凭证名称</Label>
            <Input
              id="cred-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="cred-user">用户名</Label>
            <Input
              id="cred-user"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>认证方式</Label>
            <Segmented
              value={authType}
              onChange={setAuthType}
              options={[
                { value: 'password', label: '密码模式' },
                { value: 'key', label: '秘钥模式（私钥）' },
              ]}
            />
          </div>
          {authType === 'password' ? (
            <div className="grid gap-1.5">
              <Label htmlFor="cred-password">密码</Label>
              <Input
                id="cred-password"
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
                <Label htmlFor="cred-private-key">私钥</Label>
                <Textarea
                  id="cred-private-key"
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
                <Label htmlFor="cred-passphrase">私钥口令（可选）</Label>
                <Input
                  id="cred-passphrase"
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

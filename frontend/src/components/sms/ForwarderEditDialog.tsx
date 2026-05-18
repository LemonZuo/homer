import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../api'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Textarea } from '../ui/textarea'
import { Label } from '../ui/label'
import { Select } from '../ui/select'
import { Switch } from '../ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/dialog'
import { AUTH_MODES, type AuthMode, type Forwarder } from './types'

export function ForwarderEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Forwarder | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [serverURL, setServerURL] = useState('')
  const [authMode, setAuthMode] = useState<AuthMode>(1)
  const [signKey, setSignKey] = useState('')
  const [rsaPublicKey, setRSAPublicKey] = useState('')
  const [sm4Key, setSM4Key] = useState('')
  const [timeoutSeconds, setTimeoutSeconds] = useState<number | ''>(30)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setServerURL(target?.server_url ?? '')
    setAuthMode(target?.auth_mode ?? 1)
    setSignKey(target?.sign_key ?? '')
    setRSAPublicKey(target?.rsa_public_key ?? '')
    setSM4Key(target?.sm4_key ?? '')
    setTimeoutSeconds(target?.timeout_seconds ?? 30)
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const timeout = typeof timeoutSeconds === 'number' ? timeoutSeconds : 0
    const payload = {
      name: name.trim(),
      server_url: serverURL.trim().replace(/\/+$/, ''),
      auth_mode: authMode,
      sign_key: signKey.trim(),
      rsa_public_key: rsaPublicKey.trim(),
      sm4_key: sm4Key.trim(),
      timeout_seconds: timeout > 0 ? timeout : 30,
      enabled,
    }
    if (!payload.name) return toast.error('名称必填')
    if (!payload.server_url) return toast.error('服务端地址必填')
    if (authMode === 1 && !payload.sign_key) return toast.error('签名模式需填签名密钥')
    if (authMode === 2 && !payload.rsa_public_key) return toast.error('RSA 模式需填服务端公钥')
    if (authMode === 3) {
      if (!payload.sm4_key) return toast.error('SM4 模式需填密钥')
      if (!/^[0-9a-fA-F]{32}$/.test(payload.sm4_key)) {
        return toast.error('SM4 密钥需为 32 位 hex（16 字节）')
      }
    }
    if (payload.timeout_seconds < 1 || payload.timeout_seconds > 300) {
      return toast.error('超时秒数需在 1 ~ 300 之间')
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/sms/forwarders/${target.id}`, payload)
      } else {
        await api.post('/sms/forwarders', payload)
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
          <DialogTitle>{target ? '编辑转发器' : '新增转发器'}</DialogTitle>
          <DialogDescription>
            服务端「客户端安全措施」需选「签名校验」，密钥与此处一致
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="fw-name">名称</Label>
            <Input
              id="fw-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如 家里旧手机 / 备用网关"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fw-url">服务端地址</Label>
            <Input
              id="fw-url"
              value={serverURL}
              onChange={(e) => setServerURL(e.target.value)}
              placeholder="http://192.168.1.100:5000"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fw-auth">客户端安全措施</Label>
            <Select<number>
              id="fw-auth"
              value={authMode}
              onChange={(v) => setAuthMode(v as AuthMode)}
              options={AUTH_MODES.map((m) => ({ value: m.value, label: m.label }))}
            />
            <p className="text-[11px] text-muted-foreground">
              需与服务端「设置 - 客户端安全措施」一致
            </p>
          </div>
          {authMode === 1 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-sign">签名密钥</Label>
              <Input
                id="fw-sign"
                value={signKey}
                onChange={(e) => setSignKey(e.target.value)}
                className="font-mono text-[12px]"
              />
            </div>
          )}
          {authMode === 2 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-rsa">RSA 公钥</Label>
              <Textarea
                id="fw-rsa"
                rows={4}
                value={rsaPublicKey}
                onChange={(e) => setRSAPublicKey(e.target.value)}
                placeholder="服务端 RSA 公钥，X.509/SPKI DER 的 Base64（不含 PEM 头尾）"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          {authMode === 3 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-sm4">SM4 密钥</Label>
              <Input
                id="fw-sm4"
                value={sm4Key}
                onChange={(e) => setSM4Key(e.target.value)}
                placeholder="32 位 hex（16 字节）"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="fw-timeout">请求超时（秒）</Label>
            <Input
              id="fw-timeout"
              type="number"
              min={1}
              max={300}
              value={timeoutSeconds}
              onChange={(e) => {
                const v = e.target.value
                setTimeoutSeconds(v === '' ? '' : Number(v))
              }}
              placeholder="默认 30，旧机器可调大"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="fw-enabled">启用</Label>
            <Switch id="fw-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Label } from '../../ui/label'
import { Select } from '../../ui/select'
import { Switch } from '../../ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { AcmeAccount } from '../types'

export function AccountEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: AcmeAccount | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [ca, setCA] = useState('letsencrypt')
  const [directoryURL, setDirectoryURL] = useState('')
  const [email, setEmail] = useState('')
  const [eabKID, setEABKID] = useState('')
  const [eabHMAC, setEABHMAC] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setCA(target?.ca ?? 'letsencrypt')
    setDirectoryURL(target?.directory_url ?? '')
    setEmail(target?.email ?? '')
    setEABKID(target?.eab_kid ?? '')
    setEABHMAC(target?.eab_hmac ?? '')
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const payload = {
      name: name.trim(),
      ca,
      directory_url: directoryURL.trim(),
      email: email.trim(),
      eab_kid: eabKID.trim(),
      eab_hmac: eabHMAC.trim(),
      enabled,
    }
    if (!payload.name) {
      toast.error('账号名称必填')
      return
    }
    if (!payload.email) {
      toast.error('邮箱必填')
      return
    }
    if (ca === 'custom' && !payload.directory_url) {
      toast.error('自定义 CA 需要 directory URL')
      return
    }
    if (ca === 'zerossl' && (!payload.eab_kid || !payload.eab_hmac)) {
      toast.error('ZeroSSL 需要 EAB KID 与 EAB HMAC')
      return
    }
    if ((payload.eab_kid && !payload.eab_hmac) || (!payload.eab_kid && payload.eab_hmac)) {
      toast.error('EAB KID 与 EAB HMAC 需要同时填写')
      return
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/accounts/${target.id}`, payload)
      } else {
        await api.post('/acme/accounts', payload)
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
          <DialogTitle>{target ? '编辑 CA 账号' : '新增 CA 账号'}</DialogTitle>
          <DialogDescription>
            账号配置保存到数据库；签发时按域名选择的账号注册或复用本地账号私钥
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="account-name">账号名称</Label>
            <Input
              id="account-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如 zerossl-main / letsencrypt"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="account-ca">CA</Label>
            <Select<string>
              id="account-ca"
              value={ca}
              onChange={setCA}
              options={[
                { value: 'letsencrypt', label: "Let's Encrypt" },
                { value: 'zerossl', label: 'ZeroSSL' },
                { value: 'custom', label: '自定义 ACME directory' },
              ]}
            />
          </div>
          {ca === 'custom' && (
            <div className="grid gap-1.5">
              <Label htmlFor="account-dir">Directory URL</Label>
              <Input
                id="account-dir"
                value={directoryURL}
                onChange={(e) => setDirectoryURL(e.target.value)}
                placeholder="https://acme.example.com/directory"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="account-email">邮箱</Label>
            <Input
              id="account-email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@example.com"
              className="font-mono text-[12px]"
            />
          </div>
          {(ca === 'zerossl' || ca === 'custom') && (
            <>
              <div className="grid gap-1.5">
                <Label htmlFor="account-eab-kid">
                  EAB KID{ca === 'zerossl' ? '' : '（可选）'}
                </Label>
                <Input
                  id="account-eab-kid"
                  value={eabKID}
                  onChange={(e) => setEABKID(e.target.value)}
                  className="font-mono text-[12px]"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="account-eab-hmac">
                  EAB HMAC{ca === 'zerossl' ? '' : '（可选）'}
                </Label>
                <Input
                  id="account-eab-hmac"
                  value={eabHMAC}
                  onChange={(e) => setEABHMAC(e.target.value)}
                  className="font-mono text-[12px]"
                />
              </div>
            </>
          )}
          <div className="flex items-center justify-between">
            <Label htmlFor="account-enabled">启用</Label>
            <Switch
              id="account-enabled"
              checked={enabled}
              onChange={(v) => setEnabled(v)}
            />
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

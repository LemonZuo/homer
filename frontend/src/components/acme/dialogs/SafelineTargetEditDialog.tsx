import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
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
import type { SafelineTarget } from '../types'
import { safelineTargetToDeployTarget } from '../utils'

export function SafelineTargetEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SafelineTarget | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [apiToken, setAPIToken] = useState('')
  const [skipTLSVerify, setSkipTLSVerify] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setBaseURL(target?.base_url ?? '')
    setAPIToken(target?.api_token ?? '')
    setSkipTLSVerify(target?.skip_tls_verify ?? false)
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      base_url: baseURL.trim().replace(/\/+$/, ''),
      api_token: apiToken.trim(),
      skip_tls_verify: skipTLSVerify,
      enabled,
      created_at: target?.created_at ?? '',
      updated_at: target?.updated_at ?? '',
    }
    if (!form.name) {
      toast.error('实例名称必填')
      return
    }
    if (!form.base_url) {
      toast.error('雷池地址必填')
      return
    }
    if (!form.api_token) {
      toast.error('API Token 必填')
      return
    }
    const payload = safelineTargetToDeployTarget(form)
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
          <DialogTitle>{target ? '编辑雷池实例' : '新增雷池实例'}</DialogTitle>
          <DialogDescription>
            地址填写管理端根地址，例如 https://waf.example.com:9443
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-name">实例名称</Label>
            <Input
              id="safeline-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-base-url">雷池地址</Label>
            <Input
              id="safeline-base-url"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder="https://waf.example.com:9443"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-token">API Token</Label>
            <Input
              id="safeline-token"
              type="password"
              value={apiToken}
              onChange={(e) => setAPIToken(e.target.value)}
              autoComplete="new-password"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-skip-tls">跳过 TLS 校验</Label>
            <Switch id="safeline-skip-tls" checked={skipTLSVerify} onChange={(v) => setSkipTLSVerify(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-enabled">启用</Label>
            <Switch id="safeline-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

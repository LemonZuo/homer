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
import type { CASTarget } from '../types'
import { casTargetToDeployTarget } from '../utils'

export function CASTargetEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: CASTarget | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [accessKeyID, setAccessKeyID] = useState('')
  const [accessKeySecret, setAccessKeySecret] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setAccessKeyID(target?.access_key_id ?? '')
    setAccessKeySecret(target?.access_key_secret ?? '')
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      access_key_id: accessKeyID.trim(),
      access_key_secret: accessKeySecret.trim(),
      enabled,
      created_at: target?.created_at ?? '',
      updated_at: target?.updated_at ?? '',
    }
    if (!form.name) {
      toast.error('实例名称必填')
      return
    }
    if (!form.access_key_id || !form.access_key_secret) {
      toast.error('AccessKeyId / AccessKeySecret 必填')
      return
    }
    const payload = casTargetToDeployTarget(form)
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
          <DialogTitle>{target ? '编辑阿里云 CAS 实例' : '新增阿里云 CAS 实例'}</DialogTitle>
          <DialogDescription>
            RAM 子账号需有数字证书管理服务（CAS）写权限。AK/SK 仅用于 CAS 调用。
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="cas-name">实例名称</Label>
            <Input
              id="cas-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="cas-ak">AccessKeyId</Label>
            <Input
              id="cas-ak"
              value={accessKeyID}
              onChange={(e) => setAccessKeyID(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="cas-sk">AccessKeySecret</Label>
            <Input
              id="cas-sk"
              type="password"
              value={accessKeySecret}
              onChange={(e) => setAccessKeySecret(e.target.value)}
              autoComplete="new-password"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="cas-enabled">启用</Label>
            <Switch id="cas-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

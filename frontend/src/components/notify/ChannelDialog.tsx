import { useEffect, useMemo, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../api'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
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
import type { Channel, TypeMeta } from './types'
import { FIELD_LABELS, parseConfig } from './utils'

interface ChannelDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target: Channel | null
  types: TypeMeta[]
  onSaved: () => void
}

export function ChannelDialog({
  open,
  onOpenChange,
  target,
  types,
  onSaved,
}: ChannelDialogProps) {
  const [name, setName] = useState('')
  const [type, setType] = useState('')
  const [cfg, setCfg] = useState<Record<string, string>>({})
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setType(target?.type ?? types[0]?.type ?? '')
    setCfg(target ? parseConfig(target.config_json) : {})
    setEnabled(target?.enabled ?? true)
  }, [open, target, types])

  const fields = useMemo(
    () => types.find((t) => t.type === type)?.fields ?? [],
    [types, type],
  )

  const save = async () => {
    const nm = name.trim()
    if (!nm) return toast.error('通道名称必填')
    if (!type) return toast.error('请选择通道类型')
    const cleaned: Record<string, string> = {}
    for (const f of fields) cleaned[f] = (cfg[f] ?? '').trim()
    const payload = {
      name: nm,
      type,
      config_json: JSON.stringify(cleaned),
      enabled,
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/notify/channels/${target.id}`, payload)
      } else {
        await api.post('/notify/channels', payload)
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
          <DialogTitle>{target ? '编辑通道' : '新增通道'}</DialogTitle>
          <DialogDescription>配置一个通知通道，可在下方绑定到各模块</DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="ch-name">通道名称</Label>
            <Input
              id="ch-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如 生日企微 / 告警邮件"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="ch-type">通道类型</Label>
            <Select
              id="ch-type"
              value={type}
              onChange={(v) => {
                setType(v)
                setCfg({})
              }}
              options={types.map((t) => ({ value: t.type, label: t.label }))}
              placeholder="请选择通道类型"
            />
          </div>
          {fields.map((f) => (
            <div key={f} className="grid gap-1.5">
              <Label htmlFor={`ch-${f}`}>{FIELD_LABELS[f] ?? f}</Label>
              <Input
                id={`ch-${f}`}
                value={cfg[f] ?? ''}
                onChange={(e) => setCfg((c) => ({ ...c, [f]: e.target.value }))}
                className="font-mono text-[12px]"
              />
            </div>
          ))}
          <div className="flex items-center justify-between">
            <Label htmlFor="ch-enabled">启用</Label>
            <Switch id="ch-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

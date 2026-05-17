import { useCallback, useEffect, useMemo, useState } from 'react'
import { Edit3, Loader2, Plus, Send, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Select } from './ui/select'
import { Switch } from './ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './ui/alert-dialog'
import { cn } from '../lib/utils'

interface Channel {
  id: number
  name: string
  type: string
  config_json: string
  enabled: boolean
  ref_count: number
}

interface ModuleMeta {
  key: string
  label: string
}

interface TypeMeta {
  type: string
  label: string
  fields: string[]
}

const FIELD_LABELS: Record<string, string> = {
  corp_id: '企业 ID (corp_id)',
  agent_id: '应用 ID (agent_id)',
  secret: '应用 Secret',
  tag_id: '标签 ID (tag_id)',
  api_key: 'Resend API Key',
  from: '发件地址 (from)',
  to: '收件地址 (to)',
  url: 'Webhook URL',
}

function parseConfig(s: string): Record<string, string> {
  try {
    const o = JSON.parse(s || '{}')
    return o && typeof o === 'object' ? o : {}
  } catch {
    return {}
  }
}

function ChannelDialog({
  open,
  onOpenChange,
  target,
  types,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Channel | null
  types: TypeMeta[]
  onSaved: () => void
}) {
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

export default function NotifyPage() {
  const cs = getColorSet('rose')

  const [modules, setModules] = useState<ModuleMeta[]>([])
  const [types, setTypes] = useState<TypeMeta[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [bindings, setBindings] = useState<Record<string, number[]>>({})

  const [editTarget, setEditTarget] = useState<Channel | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [delTarget, setDelTarget] = useState<Channel | null>(null)
  const [testingID, setTestingID] = useState<number | null>(null)

  const typeLabel = useCallback(
    (t: string) => types.find((x) => x.type === t)?.label ?? t,
    [types],
  )

  const loadMeta = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/meta')
      setModules(data?.data?.modules ?? [])
      setTypes(data?.data?.types ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载元信息失败')
    }
  }, [])

  const loadChannels = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/channels')
      setChannels(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载通道失败')
    }
  }, [])

  const loadBindings = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/bindings')
      setBindings(data?.data ?? {})
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载绑定失败')
    }
  }, [])

  useEffect(() => {
    loadMeta()
    loadChannels()
    loadBindings()
  }, [loadMeta, loadChannels, loadBindings])

  const doDelete = useCallback(async () => {
    if (!delTarget) return
    try {
      await api.delete(`/notify/channels/${delTarget.id}`)
      toast.success('已删除')
      setDelTarget(null)
      loadChannels()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }, [delTarget, loadChannels])

  const doTest = useCallback(async (id: number) => {
    setTestingID(id)
    try {
      await api.post(`/notify/channels/${id}/test`)
      toast.success('已发送测试消息')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '测试失败')
    } finally {
      setTestingID(null)
    }
  }, [])

  const toggleBinding = useCallback(
    async (module: string, channelID: number) => {
      const cur = bindings[module] ?? []
      const next = cur.includes(channelID)
        ? cur.filter((x) => x !== channelID)
        : [...cur, channelID]
      try {
        await api.put(`/notify/bindings/${module}`, { channel_ids: next })
        setBindings((b) => ({ ...b, [module]: next }))
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '保存绑定失败')
      }
    },
    [bindings],
  )

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">通知渠道</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            统一管理通道与每个模块的绑定，改动即时生效
          </p>
        </div>
        <Button
          size="sm"
          onClick={() => {
            setEditTarget(null)
            setEditOpen(true)
          }}
        >
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          新增通道
        </Button>
      </div>

      <Card className="mb-4 px-4 py-4">
        <div className="mb-3 text-[13px] font-medium">通道</div>
        {channels.length === 0 ? (
          <p className="rounded-md border border-dashed border-border py-8 text-center text-[12px] text-muted-foreground">
            还没有通道，点击右上角「新增通道」
          </p>
        ) : (
          <div className="space-y-2">
            {channels.map((ch) => (
              <Card key={ch.id} className="px-4 py-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">{ch.name}</span>
                      <span className="rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                        {typeLabel(ch.type)}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          ch.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {ch.enabled ? '启用' : '停用'}
                      </span>
                      {ch.ref_count > 0 && (
                        <span className="shrink-0 rounded-md bg-blue-500/10 px-1.5 py-0.5 text-[11px] font-medium text-blue-600 dark:text-blue-400">
                          {ch.ref_count}
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-2 sm:contents">
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 sm:flex-none"
                      disabled={testingID === ch.id}
                      onClick={() => doTest(ch.id)}
                    >
                      {testingID === ch.id ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Send className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 sm:flex-none"
                      onClick={() => {
                        setEditTarget(ch)
                        setEditOpen(true)
                      }}
                    >
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 hover:text-destructive disabled:hover:text-current sm:flex-none"
                      disabled={ch.ref_count > 0}
                      title={ch.ref_count > 0 ? '仍被模块绑定，请先解绑' : undefined}
                      onClick={() => setDelTarget(ch)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        )}
      </Card>

      <Card className="px-4 py-4">
        <div className="mb-3 text-[13px] font-medium">模块绑定</div>
        <div className="space-y-4">
          {modules.map((m) => {
            const bound = bindings[m.key] ?? []
            return (
              <div key={m.key} className="rounded-md border border-border px-3 py-3">
                <div className="mb-2 text-[12.5px] font-medium">{m.label}</div>
                {channels.length === 0 ? (
                  <p className="text-[11.5px] text-muted-foreground">先创建通道再绑定</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {channels.map((ch) => {
                      const on = bound.includes(ch.id)
                      return (
                        <button
                          key={ch.id}
                          type="button"
                          onClick={() => toggleBinding(m.key, ch.id)}
                          className={cn(
                            'rounded-md border px-2.5 py-1 text-[12px] font-medium transition-colors',
                            on
                              ? cs.picker
                              : 'border-border bg-background text-muted-foreground hover:text-foreground',
                          )}
                        >
                          {ch.name}
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </Card>

      <ChannelDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        types={types}
        onSaved={loadChannels}
      />

      <AlertDialog open={!!delTarget} onOpenChange={(o) => !o && setDelTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除通道</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除「{delTarget?.name}」？仍被模块绑定时无法删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={doDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

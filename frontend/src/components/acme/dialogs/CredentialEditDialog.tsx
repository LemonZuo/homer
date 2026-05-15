import { useEffect, useState } from 'react'
import { Loader2, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Label } from '../../ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { Credential, EnvPair } from '../types'
import { PROVIDER_SCHEMAS, getProviderSchema, safeParseEnvs } from '../utils'

export function CredentialEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Credential | null
  onSaved: () => void
}) {
  const [provider, setProvider] = useState('')
  const [providerMode, setProviderMode] = useState<'preset' | 'custom'>('preset')
  const [pairs, setPairs] = useState<EnvPair[]>([{ key: '', value: '' }])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) {
      const p = target?.provider ?? PROVIDER_SCHEMAS[0].key
      const schema = getProviderSchema(p)
      setProvider(p)
      setProviderMode(target ? (schema ? 'preset' : 'custom') : 'preset')
      if (target) {
        const obj = safeParseEnvs(target.envs_json ?? '{}')
        const arr = Object.entries(obj).map(([key, value]) => ({ key, value: String(value) }))
        setPairs(arr.length ? arr : [{ key: '', value: '' }])
      } else {
        setPairs((schema?.required ?? []).map((k) => ({ key: k, value: '' })) || [{ key: '', value: '' }])
      }
    }
  }, [open, target])

  const schema = getProviderSchema(provider)
  const unusedOptional = schema?.optional?.filter((k) => !pairs.some((p) => p.key === k)) ?? []

  const updatePair = (i: number, patch: Partial<EnvPair>) => {
    setPairs((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)))
  }
  const addPair = (key = '') => setPairs((prev) => [...prev, { key, value: '' }])
  const removePair = (i: number) =>
    setPairs((prev) => (prev.length === 1 ? [{ key: '', value: '' }] : prev.filter((_, idx) => idx !== i)))

  const onPresetChange = (key: string) => {
    if (key === '__custom__') {
      setProviderMode('custom')
      setProvider('')
      setPairs([{ key: '', value: '' }])
      return
    }
    setProviderMode('preset')
    setProvider(key)
    const next = getProviderSchema(key)
    setPairs((next?.required ?? []).map((k) => ({ key: k, value: '' })) || [{ key: '', value: '' }])
  }

  const save = async () => {
    const p = provider.trim()
    if (!p) {
      toast.error('provider 必填')
      return
    }
    const obj: Record<string, string> = {}
    const seen = new Set<string>()
    for (const pair of pairs) {
      const k = pair.key.trim()
      if (!k) continue
      if (seen.has(k)) {
        toast.error(`重复的 key：${k}`)
        return
      }
      seen.add(k)
      obj[k] = pair.value
    }
    setSaving(true)
    try {
      const { data } = await api.post('/acme/credentials', {
        provider: p,
        envs_json: JSON.stringify(obj),
      })
      if (data?.warning) {
        toast.warning(data.warning)
      } else {
        toast.success('已保存（凭证有效）')
      }
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
          <DialogTitle>{target ? '编辑凭证' : '新增凭证'}</DialogTitle>
          <DialogDescription>
            选择 DNS provider 后会自动列出所需环境变量，填入对应取值即可；
            其他未列出的 provider 可选「自定义」手动填 key
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="cred-provider">Provider</Label>
            {target ? (
              <Input id="cred-provider" value={provider} disabled />
            ) : (
              <select
                id="cred-provider"
                className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
                value={providerMode === 'custom' ? '__custom__' : provider}
                onChange={(e) => onPresetChange(e.target.value)}
              >
                {PROVIDER_SCHEMAS.map((p) => (
                  <option key={p.key} value={p.key}>
                    {p.label}
                  </option>
                ))}
                <option value="__custom__">自定义（手动填 provider key）</option>
              </select>
            )}
            {providerMode === 'custom' && !target && (
              <Input
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                placeholder="lego provider key，如 azure / hetzner"
                className="font-mono text-[12px]"
              />
            )}
          </div>
          <div className="grid gap-1.5">
            <Label>环境变量</Label>
            <div className="space-y-2">
              {pairs.map((pair, i) => {
                const isRequired = schema?.required.includes(pair.key)
                const isFixedKey = isRequired || (schema?.optional?.includes(pair.key) ?? false)
                if (isFixedKey) {
                  return (
                    <div key={i} className="grid gap-1">
                      <div className="flex items-center justify-between">
                        <span
                          className="font-mono text-[11.5px] text-muted-foreground"
                          title={pair.key}
                        >
                          {pair.key}
                          {isRequired && <span className="ml-1 text-rose-500">*</span>}
                        </span>
                        {!isRequired && (
                          <button
                            type="button"
                            onClick={() => removePair(i)}
                            className="text-[11px] text-muted-foreground hover:text-destructive"
                          >
                            移除
                          </button>
                        )}
                      </div>
                      <Input
                        value={pair.value}
                        onChange={(e) => updatePair(i, { value: e.target.value })}
                        placeholder={isRequired ? '必填' : 'value（可选）'}
                        className="font-mono text-[12px]"
                      />
                    </div>
                  )
                }
                return (
                  <div key={i} className="flex flex-col gap-2 sm:flex-row">
                    <Input
                      value={pair.key}
                      onChange={(e) => updatePair(i, { key: e.target.value })}
                      placeholder="KEY"
                      className="flex-1 font-mono text-[12px]"
                    />
                    <Input
                      value={pair.value}
                      onChange={(e) => updatePair(i, { value: e.target.value })}
                      placeholder="value"
                      className="flex-1 font-mono text-[12px]"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      className="shrink-0 hover:text-destructive"
                      onClick={() => removePair(i)}
                      disabled={pairs.length === 1 && !pair.key && !pair.value}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                )
              })}
              {unusedOptional.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <span className="text-[11.5px] text-muted-foreground">可选：</span>
                  {unusedOptional.map((k) => (
                    <button
                      key={k}
                      type="button"
                      onClick={() => addPair(k)}
                      className="rounded-md border border-dashed border-input px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground hover:bg-muted"
                    >
                      + {k}
                    </button>
                  ))}
                </div>
              )}
              <Button size="sm" variant="outline" onClick={() => addPair()} className="w-full">
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加自定义变量
              </Button>
            </div>
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

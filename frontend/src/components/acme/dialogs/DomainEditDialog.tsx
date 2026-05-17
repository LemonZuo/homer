import { useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
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
import { cn } from '../../../lib/utils'
import { Select } from '../../ui/select'
import type { AcmeAccount, Domain } from '../types'
import { caLabel, isValidDomain } from '../utils'

export function DomainEditDialog({
  open,
  onOpenChange,
  target,
  accounts,
  providers,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Domain | null
  accounts: AcmeAccount[]
  providers: string[]
  onSaved: () => void
}) {
  const [domains, setDomains] = useState<string[]>([])
  const [draft, setDraft] = useState('')
  const [accountID, setAccountID] = useState<number>(0)
  const [provider, setProvider] = useState('')
  const [overrides, setOverrides] = useState<Record<string, string>>({})
  const [casEnabled, setCasEnabled] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [draftError, setDraftError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    if (target) {
      const main = target.main_domain ? [target.main_domain] : []
      const sans = (target.san_domains || '')
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      setDomains([...main, ...sans])
      setAccountID(target.account_id || accounts[0]?.id || 0)
      setProvider(target.provider || providers[0] || '')
      let ov: Record<string, string> = {}
      try {
        if (target.san_providers) ov = JSON.parse(target.san_providers)
      } catch {
        ov = {}
      }
      setOverrides(ov)
      setCasEnabled(!!target.cas_enabled)
      setEnabled(target.enabled)
    } else {
      setDomains([])
      setAccountID(accounts[0]?.id || 0)
      setProvider(providers[0] || '')
      setOverrides({})
      setCasEnabled(false)
      setEnabled(true)
    }
    setDraft('')
    setDraftError(null)
  }, [open, target, accounts, providers])

  const commitDraft = (): boolean => {
    const parts = draft
      .split(/[\s,;]+/)
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
    if (parts.length === 0) {
      setDraftError(null)
      return true
    }
    const invalid = parts.filter((p) => !isValidDomain(p))
    if (invalid.length > 0) {
      setDraftError(`格式不合法：${invalid.join(', ')}`)
      return false
    }
    setDomains((prev) => {
      const seen = new Set(prev)
      const merged = [...prev]
      for (const p of parts) {
        if (!seen.has(p)) {
          merged.push(p)
          seen.add(p)
        }
      }
      return merged
    })
    setDraft('')
    setDraftError(null)
    return true
  }

  const removeDomain = (i: number) =>
    setDomains((prev) => prev.filter((_, idx) => idx !== i))

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
      e.preventDefault()
      commitDraft()
    } else if (e.key === 'Backspace' && draft === '' && domains.length > 0) {
      e.preventDefault()
      setDomains((prev) => prev.slice(0, -1))
    }
  }

  const save = async () => {
    const draftParts = draft
      .split(/[\s,;]+/)
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
    const invalid = draftParts.filter((p) => !isValidDomain(p))
    if (invalid.length > 0) {
      setDraftError(`格式不合法：${invalid.join(', ')}`)
      return
    }
    const all = Array.from(new Set([...domains, ...draftParts]))
    if (all.length === 0) {
      toast.error('至少填一个域名')
      return
    }
    if (!provider) {
      toast.error('provider 必填')
      return
    }
    if (!accountID) {
      toast.error('CA 账号必填')
      return
    }
    const allSet = new Set(all)
    const sp: Record<string, string> = {}
    for (const [d, p] of Object.entries(overrides)) {
      if (allSet.has(d) && p && p !== provider) sp[d] = p
    }
    const payload = {
      main_domain: all[0],
      san_domains: all.slice(1).join(','),
      account_id: accountID,
      provider,
      san_providers: Object.keys(sp).length ? JSON.stringify(sp) : '',
      cas_enabled: casEnabled,
      enabled,
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/domains/${target.id}`, payload)
      } else {
        await api.post('/acme/domains', payload)
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
          <DialogTitle>{target ? '编辑域名' : '新增域名'}</DialogTitle>
          <DialogDescription>
            第一个域名作为主域名，其余作为 SAN；输入后回车 / 空格 / 逗号添加
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label>域名</Label>
            <div
              className="flex min-h-[36px] w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-background px-2 py-1.5 text-[13px] focus-within:ring-2 focus-within:ring-ring"
              onClick={() => inputRef.current?.focus()}
            >
              {domains.map((d, i) => (
                <span
                  key={`${d}-${i}`}
                  className={cn(
                    'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 font-mono text-[11.5px]',
                    i === 0
                      ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                      : 'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-300',
                  )}
                  title={i === 0 ? '主域名' : 'SAN'}
                >
                  {d}
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation()
                      removeDomain(i)
                    }}
                    className="text-muted-foreground hover:text-destructive"
                    aria-label={`移除 ${d}`}
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                ref={inputRef}
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value)
                  if (draftError) setDraftError(null)
                }}
                onKeyDown={onKeyDown}
                onBlur={() => commitDraft()}
                placeholder={domains.length === 0 ? 'example.com（回车添加，可继续添加 *.example.com）' : ''}
                className="min-w-[120px] flex-1 bg-transparent font-mono text-[12px] outline-none placeholder:text-muted-foreground"
              />
            </div>
            {draftError && (
              <p className="text-[11.5px] text-rose-600 dark:text-rose-400">{draftError}</p>
            )}
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="account">CA 账号</Label>
            <Select<number>
              id="account"
              value={accountID}
              onChange={setAccountID}
              placeholder="（暂无账号，请先添加）"
              options={accounts.map((a) => ({
                value: a.id,
                label: `${a.name} (${caLabel(a.ca)})`,
              }))}
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="provider">DNS Provider</Label>
            <Select<string>
              id="provider"
              value={provider}
              onChange={setProvider}
              placeholder="（暂无凭证，请先添加）"
              options={providers.map((p) => ({ value: p, label: p }))}
            />
          </div>
          {domains.length > 0 && providers.length > 1 && (
            <div className="grid gap-1.5">
              <Label>按域名指定 DNS Provider（可选）</Label>
              <p className="text-[11.5px] text-muted-foreground">
                域名跨多个 DNS 服务商时，可单独指定；选「默认」即用上方 DNS Provider。
              </p>
              <div className="divide-y divide-border overflow-hidden rounded-md border border-input">
                {domains.map((d, i) => {
                  const ov = overrides[d] && overrides[d] !== provider ? overrides[d] : ''
                  return (
                    <div
                      key={`${d}-${i}`}
                      className="flex items-center gap-2.5 px-2.5 py-2 hover:bg-muted/40"
                    >
                      <div className="flex min-w-0 flex-1 items-center gap-1.5">
                        {i === 0 && (
                          <span className="shrink-0 rounded bg-emerald-500/10 px-1 py-0.5 text-[10px] font-medium text-emerald-700 dark:text-emerald-300">
                            主
                          </span>
                        )}
                        <span className="truncate font-mono text-[12px]" title={d}>
                          {d}
                        </span>
                      </div>
                      <Select<string>
                        className={cn(
                          'h-8 w-32 shrink-0 text-[12px] sm:w-44',
                          ov
                            ? 'border-emerald-500/50 text-emerald-700 dark:text-emerald-300'
                            : 'text-muted-foreground',
                        )}
                        value={ov}
                        onChange={(v) =>
                          setOverrides((prev) => {
                            const next = { ...prev }
                            if (v) next[d] = v
                            else delete next[d]
                            return next
                          })
                        }
                        options={[
                          { value: '', label: `默认（${provider || '—'}）` },
                          ...providers.map((p) => ({ value: p, label: p })),
                        ]}
                      />
                    </div>
                  )
                })}
              </div>
            </div>
          )}
          <div className="flex items-center justify-between">
            <Label htmlFor="cas-enabled">上传到阿里云 CAS</Label>
            <Switch
              id="cas-enabled"
              checked={casEnabled}
              onChange={(v) => setCasEnabled(v)}
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="enabled">启用自动续期</Label>
            <Switch
              id="enabled"
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

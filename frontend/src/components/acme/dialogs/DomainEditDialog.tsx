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
      setEnabled(target.enabled)
    } else {
      setDomains([])
      setAccountID(accounts[0]?.id || 0)
      setProvider(providers[0] || '')
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
    const payload = {
      main_domain: all[0],
      san_domains: all.slice(1).join(','),
      account_id: accountID,
      provider,
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
                    'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 font-mono text-[11.5px]',
                    i === 0
                      ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                      : 'bg-muted text-foreground',
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
            <select
              id="account"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={accountID ? String(accountID) : ''}
              onChange={(e) => setAccountID(Number(e.target.value))}
            >
              {accounts.length === 0 && <option value="">（暂无账号，请先添加）</option>}
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} ({caLabel(a.ca)})
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="provider">DNS Provider</Label>
            <select
              id="provider"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
            >
              {providers.length === 0 && <option value="">（暂无凭证，请先添加）</option>}
              {providers.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
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

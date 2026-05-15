import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Plus,
  Edit3,
  Trash2,
  Play,
  ScrollText,
  KeyRound,
  ShieldCheck,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { avatarColor, getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
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
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'
import { cn } from '../lib/utils'

interface Domain {
  id: number
  main_domain: string
  san_domains: string
  account_id: number
  provider: string
  enabled: boolean
  created_at: string
  updated_at: string
  not_before?: string
  not_after?: string
  cas_cert_id?: number
  issued_at?: string
}

interface AcmeAccount {
  id: number
  name: string
  ca: 'letsencrypt' | 'zerossl' | 'custom' | string
  directory_url: string
  email: string
  eab_kid: string
  eab_hmac: string
  enabled: boolean
  created_at: string
  updated_at: string
}

interface Credential {
  id: number
  provider: string
  envs_json: string
  created_at: string
  updated_at: string
}

interface Task {
  id: number
  domain_id: number
  main_domain: string
  kind: string
  status: string
  started_at: string
  finished_at: string | null
  log_text: string
  error_msg: string
}

function fmtDate(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function fmtDateTime(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function daysUntil(s?: string | null): number | null {
  if (!s) return null
  const d = new Date(s)
  if (isNaN(d.getTime())) return null
  return Math.ceil((d.getTime() - Date.now()) / 86400000)
}

function FieldRow({ label, value }: { label: string; value: string }) {
  const empty = !value || !value.trim()
  return (
    <div className="flex items-center gap-3 py-1 text-[12.5px]">
      <span className="w-16 shrink-0 text-muted-foreground">{label}</span>
      {empty ? (
        <span className="min-w-0 flex-1 text-muted-foreground/70">—</span>
      ) : (
        <span
          className="min-w-0 flex-1 truncate font-mono text-[12.5px] leading-relaxed"
          title={value}
        >
          {value}
        </span>
      )}
    </div>
  )
}

const STATUS_STYLE: Record<string, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  failed: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
}

const STATUS_LABEL: Record<string, string> = {
  pending: '待运行',
  running: '运行中',
  success: '成功',
  failed: '失败',
}

export default function AcmePage() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [accounts, setAccounts] = useState<AcmeAccount[]>([])
  const [providers, setProviders] = useState<string[]>([])
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState<string | null>(null)

  const [editOpen, setEditOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Domain | null>(null)
  const [deletePending, setDeletePending] = useState<Domain | null>(null)
  const [logTaskID, setLogTaskID] = useState<number | null>(null)

  const [credDrawerOpen, setCredDrawerOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [credEditTarget, setCredEditTarget] = useState<Credential | null>(null)
  const [credDeletePending, setCredDeletePending] = useState<Credential | null>(null)
  const [accountDrawerOpen, setAccountDrawerOpen] = useState(false)
  const [accountEditOpen, setAccountEditOpen] = useState(false)
  const [accountEditTarget, setAccountEditTarget] = useState<AcmeAccount | null>(null)
  const [accountDeletePending, setAccountDeletePending] = useState<AcmeAccount | null>(null)

  const cs = getColorSet('emerald')
  const accountSummary = useMemo(() => {
    const m = new Map<number, AcmeAccount>()
    for (const a of accounts) m.set(a.id, a)
    return (id: number) => {
      const a = m.get(id)
      if (!a) return id ? `#${id}` : '未选择 CA'
      const ca = caLabel(a.ca)
      return a.name && a.name !== ca ? `${ca} / ${a.name}` : ca
    }
  }, [accounts])

  const reloadAll = useCallback(async () => {
    setLoading(true)
    try {
      const [d, p, t, c, a] = await Promise.all([
        api.get('/acme/domains'),
        api.get('/acme/providers'),
        api.get('/acme/tasks?limit=30'),
        api.get('/acme/credentials'),
        api.get('/acme/accounts'),
      ])
      setDomains(d.data?.data ?? [])
      setProviders(p.data?.data ?? [])
      setTasks(t.data?.data ?? [])
      setCredentials(c.data?.data ?? [])
      setAccounts(a.data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  const reloadAccounts = useCallback(async () => {
    try {
      const { data } = await api.get('/acme/accounts')
      setAccounts(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载 ACME 账号失败')
    }
  }, [])

  const reloadCredentials = useCallback(async () => {
    try {
      const [p, c] = await Promise.all([
        api.get('/acme/providers'),
        api.get('/acme/credentials'),
      ])
      setProviders(p.data?.data ?? [])
      setCredentials(c.data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载凭证失败')
    }
  }, [])

  const onDeleteCredential = async () => {
    const c = credDeletePending
    if (!c) return
    setCredDeletePending(null)
    try {
      await api.delete(`/acme/credentials/${c.id}`)
      toast.success('已删除')
      await reloadCredentials()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteAccount = async () => {
    const a = accountDeletePending
    if (!a) return
    setAccountDeletePending(null)
    try {
      await api.delete(`/acme/accounts/${a.id}`)
      toast.success('已删除')
      await reloadAll()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  useEffect(() => {
    reloadAll()
  }, [reloadAll])

  const reloadTasks = useCallback(async () => {
    try {
      const { data } = await api.get('/acme/tasks?limit=30')
      setTasks(data?.data ?? [])
    } catch {
      /* silent */
    }
  }, [])

  const startIssue = async (d: Domain) => {
    setBusy(`issue-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/issue`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交，任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交失败')
    } finally {
      setBusy(null)
    }
  }

  const onDelete = async () => {
    const d = deletePending
    if (!d) return
    setDeletePending(null)
    setBusy(`del-${d.id}`)
    try {
      await api.delete(`/acme/domains/${d.id}`)
      toast.success('已删除')
      await reloadAll()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-[17px] font-semibold tracking-tight">ACME 签发</h1>
          <p className="mt-0.5 text-[12.5px] text-muted-foreground">
            自动签发与续期；落盘 ./data/acme/certs/&lt;domain&gt;/ 并上传 CAS
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={reloadAll}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            )}
            刷新
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setCredDrawerOpen(true)}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            DNS 凭证
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAccountDrawerOpen(true)}
          >
            <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
            CA 账号
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setEditTarget(null)
              setEditOpen(true)
            }}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            新增域名
          </Button>
        </div>
      </div>

      <div className="mb-8 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {domains.map((d) => {
          const days = daysUntil(d.not_after)
          const expiring = days !== null && days <= 30
          const expired = days !== null && days <= 0
          const certBadge = expired
            ? { cls: 'bg-rose-500/10 text-rose-600 dark:text-rose-400', text: '已过期' }
            : expiring
              ? {
                  cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
                  text: `${days} 天到期`,
                }
              : days !== null
                ? {
                    cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                    text: `${days} 天`,
                  }
                : { cls: 'bg-muted text-muted-foreground', text: '未签发' }
          const issuing = busy === `issue-${d.id}`
          return (
            <Card
              key={d.id}
              className={cn(
                'group flex h-full flex-col overflow-hidden transition-[transform,box-shadow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform hover:-translate-y-1',
                cs.border,
                cs.halo,
              )}
            >
              <div className="flex items-center gap-3 px-4 pt-4">
                <div
                  className={cn(
                    'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[13px] font-medium text-white shadow-sm',
                    avatarColor(d.main_domain),
                  )}
                >
                  {d.main_domain.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className="truncate text-[14px] font-semibold tracking-tight"
                    title={d.main_domain}
                  >
                    {d.main_domain}
                  </div>
                  <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
                    {accountSummary(d.account_id)} · {d.provider} · {d.enabled ? '自动续期' : '已停用'}
                  </div>
                </div>
                <span
                  className={cn(
                    'shrink-0 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                    certBadge.cls,
                  )}
                >
                  {certBadge.text}
                </span>
              </div>

              <div className="mt-3 space-y-0 px-4">
                <FieldRow label="SAN" value={d.san_domains} />
                <FieldRow label="到期" value={fmtDate(d.not_after)} />
                <FieldRow label="签发" value={fmtDate(d.issued_at)} />
                <FieldRow
                  label="CAS"
                  value={d.cas_cert_id ? `cert_id ${d.cas_cert_id}` : ''}
                />
              </div>

              <div className="mt-3 flex gap-2 px-4 pb-4">
                <Button
                  size="sm"
                  variant="outline"
                  className="flex-1"
                  onClick={() => startIssue(d)}
                  disabled={busy !== null}
                >
                  {issuing ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Play className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  签发
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setEditTarget(d)
                    setEditOpen(true)
                  }}
                  disabled={busy !== null}
                >
                  <Edit3 className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="hover:text-destructive"
                  onClick={() => setDeletePending(d)}
                  disabled={busy !== null}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </Card>
          )
        })}
        {!loading && domains.length === 0 && (
          <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
            还没有域名，点击右上「新增域名」开始
          </Card>
        )}
      </div>

      <h2 className="mb-3 text-[14px] font-semibold tracking-tight">任务历史</h2>
      <div className="space-y-2">
        {tasks.map((t) => (
          <Card
            key={t.id}
            className="flex flex-wrap items-center gap-3 px-4 py-3 text-[12.5px]"
          >
            <span
              className={cn(
                'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                STATUS_STYLE[t.status] || 'bg-muted text-muted-foreground',
              )}
            >
              {STATUS_LABEL[t.status] || t.status}
            </span>
            <span className="font-mono">#{t.id}</span>
            <span className="font-medium">{t.main_domain}</span>
            <span className="text-muted-foreground">{t.kind}</span>
            <span className="ml-auto font-mono text-[11.5px] text-muted-foreground">
              {fmtDateTime(t.started_at)}
            </span>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setLogTaskID(t.id)}
            >
              <ScrollText className="mr-1.5 h-3.5 w-3.5" />
              日志
            </Button>
          </Card>
        ))}
        {!loading && tasks.length === 0 && (
          <p className="py-6 text-center text-[12.5px] text-muted-foreground">
            还没有任务
          </p>
        )}
      </div>

      <DomainEditDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        accounts={accounts.filter((a) => a.enabled || a.id === editTarget?.account_id)}
        providers={providers}
        onSaved={reloadAll}
      />

      <AlertDialog
        open={!!deletePending}
        onOpenChange={(o) => {
          if (!o) setDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除域名配置</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {deletePending?.main_domain}
              </span>{' '}
              的 ACME 配置、关联证书记录与任务流水。本地落盘的证书文件不会被删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <LogDrawer
        taskID={logTaskID}
        onClose={() => {
          setLogTaskID(null)
          // 关闭后刷新任务列表，状态可能已变
          void reloadTasks()
          void reloadAll()
        }}
      />

      <CredentialsDrawer
        open={credDrawerOpen}
        onOpenChange={setCredDrawerOpen}
        credentials={credentials}
        onAdd={() => {
          setCredEditTarget(null)
          setCredEditOpen(true)
        }}
        onEdit={(c) => {
          setCredEditTarget(c)
          setCredEditOpen(true)
        }}
        onDelete={(c) => setCredDeletePending(c)}
      />

      <CredentialEditDialog
        open={credEditOpen}
        onOpenChange={setCredEditOpen}
        target={credEditTarget}
        onSaved={reloadCredentials}
      />

      <AccountsDrawer
        open={accountDrawerOpen}
        onOpenChange={setAccountDrawerOpen}
        accounts={accounts}
        onAdd={() => {
          setAccountEditTarget(null)
          setAccountEditOpen(true)
        }}
        onEdit={(a) => {
          setAccountEditTarget(a)
          setAccountEditOpen(true)
        }}
        onDelete={(a) => setAccountDeletePending(a)}
      />

      <AccountEditDialog
        open={accountEditOpen}
        onOpenChange={setAccountEditOpen}
        target={accountEditTarget}
        onSaved={reloadAccounts}
      />

      <AlertDialog
        open={!!accountDeletePending}
        onOpenChange={(o) => {
          if (!o) setAccountDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 CA 账号</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {accountDeletePending?.name}
              </span>{' '}
              账号；已被域名引用的账号不能删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteAccount}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!credDeletePending}
        onOpenChange={(o) => {
          if (!o) setCredDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 DNS 凭证</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除 provider{' '}
              <span className="font-mono font-medium text-foreground">
                {credDeletePending?.provider}
              </span>{' '}
              的凭证；已关联该 provider 的域名将无法继续签发，请确认。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteCredential}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

interface EditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Domain | null
  accounts: AcmeAccount[]
  providers: string[]
  onSaved: () => void
}

// 校验 DNS 名：可选 `*.` 通配符前缀 + 至少两段 label，label 不超过 63 字符
const DOMAIN_RE =
  /^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/

function isValidDomain(s: string): boolean {
  const v = s.trim().toLowerCase()
  if (v.length === 0 || v.length > 253) return false
  return DOMAIN_RE.test(v)
}

function DomainEditDialog({ open, onOpenChange, target, accounts, providers, onSaved }: EditProps) {
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
    // 草稿里可能还有内容，先校验
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
      <DialogContent>
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
                className="min-w-[160px] flex-1 bg-transparent font-mono text-[12px] outline-none placeholder:text-muted-foreground"
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
        <DialogFooter>
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

function LogDrawer({ taskID, onClose }: { taskID: number | null; onClose: () => void }) {
  const [lines, setLines] = useState<string[]>([])
  const [done, setDone] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const open = taskID !== null

  useEffect(() => {
    if (taskID === null) return
    setLines([])
    setDone(null)
    const es = new EventSource(`/api/acme/tasks/${taskID}/stream`)
    es.addEventListener('log', (ev: MessageEvent) => {
      setLines((prev) => [...prev, ev.data])
    })
    es.addEventListener('done', (ev: MessageEvent) => {
      setDone(ev.data)
      es.close()
    })
    es.onerror = () => {
      es.close()
    }
    return () => es.close()
  }, [taskID])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines, done])

  const title = useMemo(() => (taskID ? `任务 #${taskID} 日志` : '日志'), [taskID])

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{title}</DrawerTitle>
          <DrawerDescription>
            {done
              ? `状态：${STATUS_LABEL[done] || done}`
              : '实时推送（SSE）—— 关闭后可在任务历史里重看完整日志'}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 overflow-auto px-4 pb-4">
          <pre className="whitespace-pre-wrap break-all rounded-lg border border-border bg-muted/40 p-3 font-mono text-[11.5px] leading-relaxed">
            {lines.length === 0 ? '（暂无日志）' : lines.join('\n')}
            <div ref={bottomRef} />
          </pre>
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface CredentialsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  credentials: Credential[]
  onAdd: () => void
  onEdit: (c: Credential) => void
  onDelete: (c: Credential) => void
}

function CredentialsDrawer({
  open,
  onOpenChange,
  credentials,
  onAdd,
  onEdit,
  onDelete,
}: CredentialsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>DNS 凭证</DrawerTitle>
          <DrawerDescription>
            按 lego provider key 维护环境变量；保存后立刻可用于签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加凭证
            </Button>
          </div>
          {credentials.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有凭证，点击「添加凭证」开始
            </p>
          ) : (
            credentials.map((c) => (
              <Card key={c.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="font-mono text-[13px] font-medium">{c.provider}</div>
                    <div
                      className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground"
                      title={c.envs_json}
                    >
                      {Object.keys(safeParseEnvs(c.envs_json)).join(', ') || '（空）'}
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onEdit(c)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(c)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

function caLabel(ca: string) {
  switch (ca) {
    case 'letsencrypt':
      return "Let's Encrypt"
    case 'zerossl':
      return 'ZeroSSL'
    case 'custom':
      return '自定义'
    default:
      return ca || '未知'
  }
}

interface AccountsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  accounts: AcmeAccount[]
  onAdd: () => void
  onEdit: (a: AcmeAccount) => void
  onDelete: (a: AcmeAccount) => void
}

function AccountsDrawer({
  open,
  onOpenChange,
  accounts,
  onAdd,
  onEdit,
  onDelete,
}: AccountsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>CA 账号</DrawerTitle>
          <DrawerDescription>
            维护 ACME CA、邮箱与 ZeroSSL EAB；域名可选择不同账号签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加账号
            </Button>
          </div>
          {accounts.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 CA 账号，点击「添加账号」开始
            </p>
          ) : (
            accounts.map((a) => (
              <Card key={a.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">
                        {a.name}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          a.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {a.enabled ? '启用' : '停用'}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                      {caLabel(a.ca)} · {a.email}
                    </div>
                    {a.ca === 'custom' && (
                      <div
                        className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                        title={a.directory_url}
                      >
                        {a.directory_url}
                      </div>
                    )}
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onEdit(a)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(a)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface AccountEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: AcmeAccount | null
  onSaved: () => void
}

function AccountEditDialog({ open, onOpenChange, target, onSaved }: AccountEditProps) {
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
      <DialogContent>
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
            <select
              id="account-ca"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={ca}
              onChange={(e) => setCA(e.target.value)}
            >
              <option value="letsencrypt">Let's Encrypt</option>
              <option value="zerossl">ZeroSSL</option>
              <option value="custom">自定义 ACME directory</option>
            </select>
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
        <DialogFooter>
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

function safeParseEnvs(s: string): Record<string, string> {
  try {
    const v = JSON.parse(s || '{}')
    return typeof v === 'object' && v !== null ? v : {}
  } catch {
    return {}
  }
}

interface CredentialEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Credential | null
  onSaved: () => void
}

interface EnvPair {
  key: string
  value: string
}

interface ProviderSchema {
  key: string
  label: string
  required: string[]
  optional?: string[]
}

// 常用 DNS provider + 对应 lego 环境变量；完整列表见 lego 文档
const PROVIDER_SCHEMAS: ProviderSchema[] = [
  {
    key: 'alidns',
    label: '阿里云 DNS (alidns)',
    required: ['ALICLOUD_ACCESS_KEY', 'ALICLOUD_SECRET_KEY'],
    optional: ['ALICLOUD_REGION_ID', 'ALICLOUD_SECURITY_TOKEN'],
  },
  {
    key: 'tencentcloud',
    label: '腾讯云 DNS (tencentcloud)',
    required: ['TENCENTCLOUD_SECRET_ID', 'TENCENTCLOUD_SECRET_KEY'],
    optional: ['TENCENTCLOUD_REGION'],
  },
  {
    key: 'dnspod',
    label: 'DNSPod 旧版 (dnspod)',
    required: ['DNSPOD_API_KEY'],
  },
  {
    key: 'huaweicloud',
    label: '华为云 DNS (huaweicloud)',
    required: ['HUAWEICLOUD_ACCESS_KEY_ID', 'HUAWEICLOUD_SECRET_ACCESS_KEY', 'HUAWEICLOUD_REGION'],
  },
  {
    key: 'cloudflare',
    label: 'Cloudflare (cloudflare)',
    required: ['CLOUDFLARE_DNS_API_TOKEN'],
    optional: ['CLOUDFLARE_ZONE_API_TOKEN'],
  },
  {
    key: 'godaddy',
    label: 'GoDaddy (godaddy)',
    required: ['GODADDY_API_KEY', 'GODADDY_API_SECRET'],
  },
  {
    key: 'gcore',
    label: 'Gcore (gcore)',
    required: ['GCORE_PERMANENT_API_TOKEN'],
  },
  {
    key: 'digitalocean',
    label: 'DigitalOcean (digitalocean)',
    required: ['DO_AUTH_TOKEN'],
  },
  {
    key: 'namecheap',
    label: 'Namecheap (namecheap)',
    required: ['NAMECHEAP_API_USER', 'NAMECHEAP_API_KEY'],
  },
  {
    key: 'gandiv5',
    label: 'Gandi v5 (gandiv5)',
    required: ['GANDIV5_PERSONAL_ACCESS_TOKEN'],
  },
  {
    key: 'route53',
    label: 'AWS Route 53 (route53)',
    required: ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_REGION'],
  },
]

function getProviderSchema(key: string): ProviderSchema | undefined {
  return PROVIDER_SCHEMAS.find((p) => p.key === key)
}

function CredentialEditDialog({ open, onOpenChange, target, onSaved }: CredentialEditProps) {
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
      <DialogContent>
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
                  <div key={i} className="flex gap-2">
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
        <DialogFooter>
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

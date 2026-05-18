import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, UploadCloud, Trash2, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { avatarColor, getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
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

interface Cert {
  id: number
  name: string
  orgName: string
  startDate: string
  endDate: string
  expired: boolean
  sans: string
  common: string
  issuer: string
  country: string
  fingerprint: string
  province: string
  city: string
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

type Pending =
  | { kind: 'deploy'; cert: Cert }
  | { kind: 'delete'; cert: Cert }

export default function CertStorePage() {
  const [certs, setCerts] = useState<Cert[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState<string | null>(null)
  const [pending, setPending] = useState<Pending | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/cas/certificates')
      setCerts(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const deploy = async (c: Cert) => {
    setBusy(`deploy-${c.id}`)
    try {
      const { data } = await api.post('/cas/deploy', { certName: c.name })
      toast.success(data?.message || '已提交')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '部署失败')
    } finally {
      setBusy(null)
    }
  }

  const remove = async (c: Cert) => {
    setBusy(`del-${c.id}`)
    try {
      await api.delete(`/cas/certificates/${c.id}`)
      toast.success('已删除')
      await load()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    } finally {
      setBusy(null)
    }
  }

  const onConfirm = () => {
    if (!pending) return
    const p = pending
    setPending(null)
    if (p.kind === 'deploy') void deploy(p.cert)
    else void remove(p.cert)
  }

  const cs = getColorSet('violet')
  const empty = !loading && certs.length === 0

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">证书管理</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            阿里云数字证书（CAS）列表、部署到 CDN 与删除
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="w-full sm:w-auto"
          onClick={load}
          disabled={loading}
        >
          {loading ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          )}
          刷新
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {certs.map((c) => {
          const deploying = busy === `deploy-${c.id}`
          const deleting = busy === `del-${c.id}`
          return (
            <Card
              key={c.id}
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
                    avatarColor(c.name || String(c.id)),
                  )}
                >
                  {(c.name || '?').charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className="truncate text-[14px] font-semibold tracking-tight"
                    title={c.name}
                  >
                    {c.name || '(未命名)'}
                  </div>
                  <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
                    {c.issuer || '—'}
                  </div>
                </div>
                <span
                  className={cn(
                    'shrink-0 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                    c.expired
                      ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                      : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                  )}
                >
                  {c.expired ? '已过期' : '有效'}
                </span>
              </div>

              <div className="mt-3 space-y-0 px-4">
                <FieldRow label="绑定域名" value={c.sans} />
                <FieldRow label="申请日期" value={c.startDate} />
                <FieldRow label="有效期限" value={c.endDate} />
                <FieldRow label="机构" value={c.orgName} />
              </div>

              <div className="mt-3 flex gap-2 px-4 pb-4">
                <Button
                  size="sm"
                  variant="outline"
                  className="flex-1"
                  onClick={() => setPending({ kind: 'deploy', cert: c })}
                  disabled={busy !== null}
                >
                  {deploying ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <UploadCloud className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  部署 CDN
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="flex-1 hover:text-destructive"
                  onClick={() => setPending({ kind: 'delete', cert: c })}
                  disabled={busy !== null}
                >
                  {deleting ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  删除
                </Button>
              </div>
            </Card>
          )
        })}
        {empty && (
          <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
            没有证书（或阿里云 CAS 未配置）
          </Card>
        )}
      </div>

      <AlertDialog
        open={!!pending}
        onOpenChange={(o) => {
          if (!o) setPending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pending?.kind === 'delete' ? '确认删除证书' : '部署证书到 CDN'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pending?.kind === 'delete' ? (
                <>
                  即将删除证书{' '}
                  <span className="font-mono font-medium text-foreground">
                    {pending.cert.name}
                  </span>
                  。该操作不可撤销。
                </>
              ) : pending ? (
                <>
                  即将把证书{' '}
                  <span className="font-mono font-medium text-foreground">
                    {pending.cert.name}
                  </span>{' '}
                  部署到所有 HTTPS 加速域名，任务将在后台异步执行。
                </>
              ) : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            {pending?.kind === 'delete' ? (
              <AlertDialogAction onClick={onConfirm}>删除</AlertDialogAction>
            ) : (
              <AlertDialogAction
                className="!bg-primary !text-primary-foreground hover:!bg-primary/90"
                onClick={onConfirm}
              >
                部署
              </AlertDialogAction>
            )}
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

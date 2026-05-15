import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, UploadCloud, Loader2 } from 'lucide-react'
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

interface Domain {
  domainName: string
  cname: string
  domainStatus: string
  sslProtocol: string
  certName: string
  gmtCreated: string
  sourceType: string
  sourceContent: string
  sourcePort: number
}

interface Cert {
  certId: number
  certName: string
  common: string
  issuer: string
  fingerprint: string
  lastTime: number
}

function statusText(s: string) {
  if (s === 'online') return '正常运行'
  if (s === 'offline') return '已停止'
  return s || '—'
}

function fmtDate(s: string) {
  if (!s) return '—'
  const d = new Date(s)
  return isNaN(d.getTime())
    ? s
    : `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function FieldRow({ label, value }: { label: string; value: string }) {
  const empty = !value || !value.trim()
  return (
    <div className="flex items-center gap-3 py-1 text-[12.5px]">
      <span className="w-14 shrink-0 text-muted-foreground">{label}</span>
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

export default function CdnPage() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [certs, setCerts] = useState<Cert[]>([])
  const [loading, setLoading] = useState(true)
  const [deploying, setDeploying] = useState<string | null>(null)
  const [pendingDeploy, setPendingDeploy] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [d, c] = await Promise.all([
        api.get('/cdn/domains'),
        api.get('/cdn/certificates'),
      ])
      setDomains(d.data?.data ?? [])
      setCerts(c.data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const deploy = async (certName: string) => {
    setDeploying(certName)
    try {
      const { data } = await api.post('/cdn/deploy', { certName })
      toast.success(data?.message || '已部署')
      await load()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '部署失败')
    } finally {
      setDeploying(null)
    }
  }

  const cs = getColorSet('sky')
  const empty = !loading && domains.length === 0

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-[17px] font-semibold tracking-tight">加速域名</h1>
          <p className="mt-0.5 text-[12.5px] text-muted-foreground">
            阿里云 CDN 域名只读视图与证书部署
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 sm:flex-none"
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
      </div>

      <div className="mb-8 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {domains.map((d) => {
          const online = d.domainStatus === 'online'
          const https = d.sslProtocol === 'on'
          const source = d.sourceContent
            ? `${d.sourceContent}${d.sourcePort ? ':' + d.sourcePort : ''}`
            : ''
          return (
            <Card
              key={d.domainName}
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
                    avatarColor(d.domainName),
                  )}
                >
                  {d.domainName.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className="truncate text-[14px] font-semibold tracking-tight"
                    title={d.domainName}
                  >
                    {d.domainName}
                  </div>
                  <div className="mt-0.5 flex items-center gap-1.5 text-[12px] text-muted-foreground">
                    <span
                      className={cn(
                        'h-1.5 w-1.5 shrink-0 rounded-full',
                        online ? 'bg-emerald-500' : 'bg-muted-foreground/40',
                      )}
                    />
                    {statusText(d.domainStatus)}
                  </div>
                </div>
                <span
                  className={cn(
                    'shrink-0 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                    https
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-muted text-muted-foreground',
                  )}
                >
                  {https ? 'HTTPS' : '无 HTTPS'}
                </span>
              </div>

              <div className="mt-3 space-y-0 px-4 pb-3">
                <FieldRow label="CNAME" value={d.cname} />
                <FieldRow label="证书" value={d.certName} />
                <FieldRow label="回源" value={source} />
                <FieldRow label="创建" value={fmtDate(d.gmtCreated)} />
              </div>
            </Card>
          )
        })}
        {empty && (
          <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
            没有加速域名（或阿里云 CDN 未配置）
          </Card>
        )}
      </div>

      <h2 className="mb-3 text-[14px] font-semibold tracking-tight">
        CDN 证书（点击部署到所有 HTTPS 域名）
      </h2>
      <div className="grid gap-3 sm:grid-cols-2">
        {certs.map((c) => (
          <Card key={`${c.certId}-${c.certName}`} className="flex items-center gap-3 p-4">
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-semibold">{c.certName}</div>
              <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                {c.common || '—'} · {c.issuer || '—'}
              </div>
            </div>
            <Button
              size="sm"
              variant="outline"
              className="shrink-0"
              onClick={() => setPendingDeploy(c.certName)}
              disabled={deploying !== null}
            >
              {deploying === c.certName ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <UploadCloud className="mr-1.5 h-3.5 w-3.5" />
              )}
              部署
            </Button>
          </Card>
        ))}
        {!loading && certs.length === 0 && (
          <p className="col-span-full py-8 text-center text-[12.5px] text-muted-foreground">
            没有可用证书（或阿里云 CDN 未配置）
          </p>
        )}
      </div>

      <AlertDialog
        open={!!pendingDeploy}
        onOpenChange={(o) => {
          if (!o) setPendingDeploy(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>部署证书到 HTTPS 域名</AlertDialogTitle>
            <AlertDialogDescription>
              即将把证书{' '}
              <span className="font-mono font-medium text-foreground">
                {pendingDeploy}
              </span>{' '}
              部署到所有开启 HTTPS 的加速域名。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="!bg-primary !text-primary-foreground hover:!bg-primary/90"
              onClick={() => {
                const name = pendingDeploy
                setPendingDeploy(null)
                if (name) void deploy(name)
              }}
            >
              部署
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

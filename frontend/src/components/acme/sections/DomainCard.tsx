import { Ban, Download, Edit3, Loader2, Play, Send, Trash2 } from 'lucide-react'
import { avatarColor, getColorSet } from '../../../colors'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import { cn } from '../../../lib/utils'
import { FieldRow } from '../FieldRow'
import { daysUntil, fmtDate } from '../utils'
import type { Domain } from '../types'

const cs = getColorSet('emerald')

export function DomainCard({
  d,
  busy,
  accountSummary,
  onIssue,
  onDeploy,
  onEdit,
  onRevoke,
  onDelete,
  onDownload,
}: {
  d: Domain
  busy: string | null
  accountSummary: (id: number) => string
  onIssue: (d: Domain) => void
  onDeploy: (d: Domain) => void
  onEdit: (d: Domain) => void
  onRevoke: (d: Domain) => void
  onDelete: (d: Domain) => void
  onDownload: (d: Domain) => void
}) {
  const days = daysUntil(d.not_after)
  const revoked = d.cert_status === 'revoked'
  const expiring = days !== null && days <= 30
  const expired = days !== null && days <= 0
  const certBadge = revoked
    ? { cls: 'bg-rose-500/10 text-rose-600 dark:text-rose-400', text: '已吊销' }
    : expired
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
  const issueLabel = revoked || days !== null ? '重签' : '签发'
  return (
    <Card
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
          <div className="mt-0.5 text-[12px] text-muted-foreground line-clamp-2 sm:line-clamp-none sm:truncate">
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
        {revoked && <FieldRow label="吊销" value={fmtDate(d.revoked_at)} />}
      </div>

      <div className="mt-3 grid grid-cols-3 gap-2 px-4 pb-4 sm:flex sm:gap-2">
        <Button
          size="sm"
          variant="outline"
          className="h-9 sm:h-8 sm:flex-1"
          onClick={() => onIssue(d)}
          disabled={busy !== null}
        >
          {issuing ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <Play className="mr-1.5 h-3.5 w-3.5" />
          )}
          {issueLabel}
        </Button>
        <Button
          size="icon"
          variant="outline"
          className="h-9 w-full sm:h-8 sm:w-8"
          onClick={() => onDeploy(d)}
          disabled={busy !== null}
          title="部署配置"
        >
          <Send className="h-3.5 w-3.5" />
        </Button>
        <Button
          size="icon"
          variant="outline"
          className="h-9 w-full sm:h-8 sm:w-8"
          onClick={() => onDownload(d)}
          disabled={busy !== null || !d.not_after}
          title={!d.not_after ? '当前域名还没有证书' : '下载证书 ZIP'}
        >
          <Download className="h-3.5 w-3.5" />
        </Button>
        <Button
          size="icon"
          variant="outline"
          className="h-9 w-full sm:h-8 sm:w-8"
          onClick={() => onEdit(d)}
          disabled={busy !== null}
          title="编辑域名"
        >
          <Edit3 className="h-3.5 w-3.5" />
        </Button>
        <Button
          size="icon"
          variant="outline"
          className="h-9 w-full hover:text-destructive sm:h-8 sm:w-8"
          onClick={() => onRevoke(d)}
          disabled={busy !== null || !d.not_after || revoked}
          title={!d.not_after ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : '吊销当前证书'}
        >
          <Ban className="h-3.5 w-3.5" />
        </Button>
        <Button
          size="icon"
          variant="outline"
          className="h-9 w-full hover:text-destructive sm:h-8 sm:w-8"
          onClick={() => onDelete(d)}
          disabled={busy !== null}
          title="删除域名"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    </Card>
  )
}

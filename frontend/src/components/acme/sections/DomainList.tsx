import { Card } from '../../ui/card'
import type { Domain } from '../types'
import { DomainCard } from './DomainCard'

interface DomainListProps {
  domains: Domain[]
  loading: boolean
  busy: string | null
  accountSummary: (id: number) => string
  onIssue: (domain: Domain) => void
  onDeploy: (domain: Domain) => void
  onEdit: (domain: Domain) => void
  onRevoke: (domain: Domain) => void
  onDelete: (domain: Domain) => void
  onDownload: (domain: Domain) => void
}

export function DomainList({
  domains,
  loading,
  busy,
  accountSummary,
  onIssue,
  onDeploy,
  onEdit,
  onRevoke,
  onDelete,
  onDownload,
}: DomainListProps) {
  return (
    <div className="mb-8 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {domains.map((d) => (
        <DomainCard
          key={d.id}
          d={d}
          busy={busy}
          accountSummary={accountSummary}
          onIssue={onIssue}
          onDeploy={onDeploy}
          onEdit={onEdit}
          onRevoke={onRevoke}
          onDelete={onDelete}
          onDownload={onDownload}
        />
      ))}
      {!loading && domains.length === 0 && (
        <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
          还没有域名，点击右上「新增域名」开始
        </Card>
      )}
    </div>
  )
}

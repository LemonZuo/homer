import { KeyRound, Loader2, Plus, RefreshCw, Server, ShieldCheck } from 'lucide-react'

import { getColorSet } from '../../colors'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'

interface AcmePageHeaderProps {
  loading: boolean
  onRefresh: () => Promise<void> | void
  onOpenCredentials: () => void
  onOpenAccounts: () => void
  onOpenDeployTargets: () => void
  onAddDomain: () => void
}

export function AcmePageHeader({
  loading,
  onRefresh,
  onOpenCredentials,
  onOpenAccounts,
  onOpenDeployTargets,
  onAddDomain,
}: AcmePageHeaderProps) {
  const cs = getColorSet('emerald')

  return (
    <>
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">证书签发</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            自动签发与续期，配置部署目标后一键分发
          </p>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:flex sm:shrink-0">
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={onRefresh}
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
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={onOpenCredentials}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            DNS 凭证
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={onOpenAccounts}
          >
            <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
            CA 账号
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={onOpenDeployTargets}
          >
            <Server className="mr-1.5 h-3.5 w-3.5" />
            部署目标
          </Button>
          <Button
            size="sm"
            className="hidden h-10 w-full sm:inline-flex sm:h-8 sm:w-auto"
            onClick={onAddDomain}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            新增域名
          </Button>
        </div>
      </div>

      <Button
        size="icon"
        onClick={onAddDomain}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增域名"
      >
        <Plus className="h-5 w-5" />
      </Button>
    </>
  )
}

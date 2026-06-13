import { ConfirmDialog } from '../ConfirmDialog'
import { DomainEditDialog } from '../dialogs/DomainEditDialog'
import { LogDrawer } from '../LogDrawer'
import type { AcmeActions } from '../useAcmeActions'
import type { AcmeUiState } from '../useAcmeUiState'
import type { AcmeAccount } from '../types'

interface DomainOverlaysProps {
  ui: AcmeUiState
  actions: AcmeActions
  accounts: AcmeAccount[]
  providers: string[]
  onSaved: () => Promise<void> | void
  onTasksReload: () => Promise<void> | void
  onDeployConfigsReload: (domainID: number) => Promise<void> | void
}

export function DomainOverlays({
  ui,
  actions,
  accounts,
  providers,
  onSaved,
  onTasksReload,
  onDeployConfigsReload,
}: DomainOverlaysProps) {
  return (
    <>
      <DomainEditDialog
        open={ui.domain.edit.open}
        onOpenChange={ui.domain.edit.setOpen}
        target={ui.domain.edit.target}
        accounts={accounts.filter((a) => a.enabled || a.id === ui.domain.edit.target?.account_id)}
        providers={providers}
        onSaved={onSaved}
      />

      <ConfirmDialog
        open={!!ui.domain.remove.pending}
        onClose={ui.domain.remove.clear}
        onConfirm={actions.deleteDomain}
        title="删除域名配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.domain.remove.pending?.main_domain}
        </span>{' '}
        的 ACME 配置、关联证书记录与任务流水。本地落盘的证书文件不会被删除。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.domain.revoke.pending}
        onClose={ui.domain.revoke.clear}
        onConfirm={() => {
          if (ui.domain.revoke.pending) void actions.startRevoke(ui.domain.revoke.pending)
        }}
        title="吊销当前证书"
        confirmText="吊销"
      >
        即将向 CA 吊销{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.domain.revoke.pending?.main_domain}
        </span>{' '}
        当前证书。吊销不可逆，且不会自动删除 CAS 证书或切换 CDN 配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.domain.reissue.pending}
        onClose={ui.domain.reissue.clear}
        onConfirm={() => {
          if (ui.domain.reissue.pending) void actions.startIssue(ui.domain.reissue.pending)
        }}
        title="重签当前证书"
        confirmText="重签"
      >
        <span className="font-mono font-medium text-foreground">
          {ui.domain.reissue.pending?.main_domain}
        </span>{' '}
        当前证书仍在有效期内。重签会向 CA 申请一张新证书并覆盖原有产物，签发额度将按 CA 规则计入。确认继续？
      </ConfirmDialog>

      <LogDrawer
        taskID={ui.log.taskID}
        onClose={() => {
          ui.log.clear()
          void onTasksReload()
          void onSaved()
          if (ui.deploy.entryDomain) void onDeployConfigsReload(ui.deploy.entryDomain.id)
        }}
      />
    </>
  )
}

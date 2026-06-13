import { ConfirmDialog } from '../ConfirmDialog'
import { CASDeployConfigEditDialog } from '../dialogs/CASDeployConfigEditDialog'
import { FnOSDeployConfigEditDialog } from '../dialogs/FnOSDeployConfigEditDialog'
import { SSHDeployConfigEditDialog } from '../dialogs/SSHDeployConfigEditDialog'
import { SafelineDeployConfigEditDialog } from '../dialogs/SafelineDeployConfigEditDialog'
import { DeployConfigsDrawer } from '../drawers/DeployConfigsDrawer'
import type { AcmeActions } from '../useAcmeActions'
import type { AcmeUiState } from '../useAcmeUiState'
import type {
  CASDeployConfig,
  CASTarget,
  FnOSDeployConfig,
  FnOSTarget,
  SSHDeployConfig,
  SSHTarget,
  SafelineDeployConfig,
  SafelineTarget,
} from '../types'

interface DeployConfigOverlaysProps {
  ui: AcmeUiState
  actions: AcmeActions
  deployConfigs: SSHDeployConfig[]
  deployConfigLoading: boolean
  safeDeployConfigs: SafelineDeployConfig[]
  safeDeployLoading: boolean
  casDeployConfigs: CASDeployConfig[]
  casDeployLoading: boolean
  fnosDeployConfigs: FnOSDeployConfig[]
  fnosDeployLoading: boolean
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  casTargets: CASTarget[]
  fnosTargets: FnOSTarget[]
  onDeployConfigsReload: (domainID: number) => Promise<void> | void
}

export function DeployConfigOverlays({
  ui,
  actions,
  deployConfigs,
  deployConfigLoading,
  safeDeployConfigs,
  safeDeployLoading,
  casDeployConfigs,
  casDeployLoading,
  fnosDeployConfigs,
  fnosDeployLoading,
  sshTargets,
  safelineTargets,
  casTargets,
  fnosTargets,
  onDeployConfigsReload,
}: DeployConfigOverlaysProps) {
  const reloadCurrentDeployConfigs = () => {
    if (ui.deploy.entryDomain) void onDeployConfigsReload(ui.deploy.entryDomain.id)
  }

  return (
    <>
      <DeployConfigsDrawer
        open={!!ui.deploy.entryDomain}
        onOpenChange={(o) => {
          if (!o) ui.deploy.closeConfigs()
        }}
        domain={ui.deploy.entryDomain}
        sshConfigs={deployConfigs}
        safelineConfigs={safeDeployConfigs}
        casConfigs={casDeployConfigs}
        fnosConfigs={fnosDeployConfigs}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        loading={deployConfigLoading || safeDeployLoading || casDeployLoading || fnosDeployLoading}
        busy={ui.busy}
        onAddSSH={ui.deploy.ssh.edit.add}
        onEditSSH={ui.deploy.ssh.edit.edit}
        onCopySSH={(cfg) => {
          ui.deploy.ssh.edit.setTarget({
            ...cfg,
            id: 0,
            name: `${cfg.name || '配置'} 副本`,
            created_at: '',
            updated_at: '',
          })
          ui.deploy.ssh.edit.setOpen(true)
        }}
        onDeleteSSH={ui.deploy.ssh.remove.setPending}
        onDeploySSH={(cfg) => void actions.startDeployConfig('ssh', cfg)}
        onAddSafeline={ui.deploy.safeline.edit.add}
        onEditSafeline={ui.deploy.safeline.edit.edit}
        onDeleteSafeline={ui.deploy.safeline.remove.setPending}
        onDeploySafeline={(cfg) => void actions.startDeployConfig('safeline', cfg)}
        onAddCAS={ui.deploy.cas.edit.add}
        onEditCAS={ui.deploy.cas.edit.edit}
        onDeleteCAS={ui.deploy.cas.remove.setPending}
        onDeployCAS={(cfg) => void actions.startDeployConfig('cas', cfg)}
        onAddFnOS={ui.deploy.fnos.edit.add}
        onEditFnOS={ui.deploy.fnos.edit.edit}
        onDeleteFnOS={ui.deploy.fnos.remove.setPending}
        onDeployFnOS={(cfg) => void actions.startDeployConfig('fnos', cfg)}
        onDeployAll={() => void actions.startDeployAllConfigs()}
      />

      <SSHDeployConfigEditDialog
        open={ui.deploy.ssh.edit.open}
        onOpenChange={ui.deploy.ssh.edit.setOpen}
        domain={ui.deploy.ssh.domain}
        config={ui.deploy.ssh.edit.target}
        targets={sshTargets}
        onSaved={reloadCurrentDeployConfigs}
      />

      <SafelineDeployConfigEditDialog
        open={ui.deploy.safeline.edit.open}
        onOpenChange={ui.deploy.safeline.edit.setOpen}
        domain={ui.deploy.safeline.domain}
        config={ui.deploy.safeline.edit.target}
        targets={safelineTargets}
        onSaved={reloadCurrentDeployConfigs}
      />

      <CASDeployConfigEditDialog
        open={ui.deploy.cas.edit.open}
        onOpenChange={ui.deploy.cas.edit.setOpen}
        domain={ui.deploy.cas.domain}
        config={ui.deploy.cas.edit.target}
        targets={casTargets}
        onSaved={reloadCurrentDeployConfigs}
      />

      <FnOSDeployConfigEditDialog
        open={ui.deploy.fnos.edit.open}
        onOpenChange={ui.deploy.fnos.edit.setOpen}
        domain={ui.deploy.fnos.domain}
        config={ui.deploy.fnos.edit.target}
        targets={fnosTargets}
        onSaved={reloadCurrentDeployConfigs}
      />

      <ConfirmDialog
        open={!!ui.deploy.ssh.remove.pending}
        onClose={ui.deploy.ssh.remove.clear}
        onConfirm={actions.onDeleteSSHDeployConfig}
        title="删除部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.deploy.ssh.remove.pending?.name || `#${ui.deploy.ssh.remove.pending?.id}`}
        </span>{' '}
        的 SSH 部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.deploy.safeline.remove.pending}
        onClose={ui.deploy.safeline.remove.clear}
        onConfirm={actions.onDeleteSafelineDeployConfig}
        title="删除雷池部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.deploy.safeline.remove.pending?.name || `#${ui.deploy.safeline.remove.pending?.id}`}
        </span>{' '}
        的雷池部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.deploy.cas.remove.pending}
        onClose={ui.deploy.cas.remove.clear}
        onConfirm={actions.onDeleteCASDeployConfig}
        title="删除阿里云 CAS 部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.deploy.cas.remove.pending?.name || `#${ui.deploy.cas.remove.pending?.id}`}
        </span>{' '}
        的 CAS 部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.deploy.fnos.remove.pending}
        onClose={ui.deploy.fnos.remove.clear}
        onConfirm={actions.onDeleteFnOSDeployConfig}
        title="删除 fnOS 部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.deploy.fnos.remove.pending?.name || `#${ui.deploy.fnos.remove.pending?.id}`}
        </span>{' '}
        的 fnOS 部署配置。
      </ConfirmDialog>
    </>
  )
}

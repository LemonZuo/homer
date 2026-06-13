import { ConfirmDialog } from '../ConfirmDialog'
import { CASTargetEditDialog } from '../dialogs/CASTargetEditDialog'
import { FnOSTargetEditDialog } from '../dialogs/FnOSTargetEditDialog'
import { SSHCredentialEditDialog } from '../dialogs/SSHCredentialEditDialog'
import { SSHTargetEditDialog } from '../dialogs/SSHTargetEditDialog'
import { SafelineTargetEditDialog } from '../dialogs/SafelineTargetEditDialog'
import { DeployTargetsEntryDrawer } from '../drawers/DeployTargetsEntryDrawer'
import { SSHCredentialsDrawer } from '../drawers/SSHCredentialsDrawer'
import type { AcmeActions } from '../useAcmeActions'
import type { AcmeUiState } from '../useAcmeUiState'
import type { CASTarget, FnOSTarget, SSHCredential, SSHTarget, SafelineTarget } from '../types'

interface DeployTargetOverlaysProps {
  ui: AcmeUiState
  actions: AcmeActions
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  casTargets: CASTarget[]
  fnosTargets: FnOSTarget[]
  sshCredentials: SSHCredential[]
  onDeployTargetsReload: () => Promise<void> | void
  onSSHCredentialsReload: () => Promise<void> | void
}

export function DeployTargetOverlays({
  ui,
  actions,
  sshTargets,
  safelineTargets,
  casTargets,
  fnosTargets,
  sshCredentials,
  onDeployTargetsReload,
  onSSHCredentialsReload,
}: DeployTargetOverlaysProps) {
  return (
    <>
      <DeployTargetsEntryDrawer
        open={ui.targets.entry.open}
        onOpenChange={ui.targets.entry.setOpen}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        onAddSSH={ui.targets.ssh.edit.add}
        onEditSSH={ui.targets.ssh.edit.edit}
        onDeleteSSH={ui.targets.ssh.remove.setPending}
        onManageCredentials={ui.sshCredentials.drawer.show}
        onTestSSH={(t) => void actions.testDeployTarget(t.id)}
        onAddSafeline={ui.targets.safeline.edit.add}
        onEditSafeline={ui.targets.safeline.edit.edit}
        onDeleteSafeline={ui.targets.safeline.remove.setPending}
        onTestSafeline={(t) => void actions.testDeployTarget(t.id)}
        onAddCAS={ui.targets.cas.edit.add}
        onEditCAS={ui.targets.cas.edit.edit}
        onDeleteCAS={ui.targets.cas.remove.setPending}
        onTestCAS={(t) => void actions.testDeployTarget(t.id)}
        onAddFnOS={ui.targets.fnos.edit.add}
        onEditFnOS={ui.targets.fnos.edit.edit}
        onDeleteFnOS={ui.targets.fnos.remove.setPending}
        onTestFnOS={(t) => void actions.testDeployTarget(t.id)}
      />

      <SSHTargetEditDialog
        open={ui.targets.ssh.edit.open}
        onOpenChange={ui.targets.ssh.edit.setOpen}
        target={ui.targets.ssh.edit.target}
        credentials={sshCredentials}
        sshTargets={sshTargets}
        fnosTargets={fnosTargets}
        onManageCredentials={ui.sshCredentials.drawer.show}
        onSaved={onDeployTargetsReload}
      />

      <SSHCredentialsDrawer
        open={ui.sshCredentials.drawer.open}
        onOpenChange={ui.sshCredentials.drawer.setOpen}
        credentials={sshCredentials}
        onAdd={ui.sshCredentials.edit.add}
        onEdit={ui.sshCredentials.edit.edit}
        onDelete={ui.sshCredentials.remove.setPending}
      />

      <SSHCredentialEditDialog
        open={ui.sshCredentials.edit.open}
        onOpenChange={ui.sshCredentials.edit.setOpen}
        target={ui.sshCredentials.edit.target}
        onSaved={onSSHCredentialsReload}
      />

      <SafelineTargetEditDialog
        open={ui.targets.safeline.edit.open}
        onOpenChange={ui.targets.safeline.edit.setOpen}
        target={ui.targets.safeline.edit.target}
        onSaved={onDeployTargetsReload}
      />

      <CASTargetEditDialog
        open={ui.targets.cas.edit.open}
        onOpenChange={ui.targets.cas.edit.setOpen}
        target={ui.targets.cas.edit.target}
        onSaved={onDeployTargetsReload}
      />

      <FnOSTargetEditDialog
        open={ui.targets.fnos.edit.open}
        onOpenChange={ui.targets.fnos.edit.setOpen}
        target={ui.targets.fnos.edit.target}
        credentials={sshCredentials}
        sshTargets={sshTargets}
        fnosTargets={fnosTargets}
        onManageCredentials={ui.sshCredentials.drawer.show}
        onSaved={onDeployTargetsReload}
      />

      <ConfirmDialog
        open={!!ui.targets.ssh.remove.pending}
        onClose={ui.targets.ssh.remove.clear}
        onConfirm={actions.onDeleteSSHTarget}
        title="删除 SSH 机器"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.targets.ssh.remove.pending?.name}
        </span>{' '}
        的部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.targets.safeline.remove.pending}
        onClose={ui.targets.safeline.remove.clear}
        onConfirm={actions.onDeleteSafelineTarget}
        title="删除雷池实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.targets.safeline.remove.pending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.targets.cas.remove.pending}
        onClose={ui.targets.cas.remove.clear}
        onConfirm={actions.onDeleteCASTarget}
        title="删除阿里云 CAS 实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.targets.cas.remove.pending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.targets.fnos.remove.pending}
        onClose={ui.targets.fnos.remove.clear}
        onConfirm={actions.onDeleteFnOSTarget}
        title="删除 fnOS 实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.targets.fnos.remove.pending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.sshCredentials.remove.pending}
        onClose={ui.sshCredentials.remove.clear}
        onConfirm={actions.onDeleteSSHCredential}
        title="删除登录凭证"
      >
        即将删除登录凭证{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.sshCredentials.remove.pending?.name}
        </span>
        ；引用了该凭证的机器将无法连接，请确认。
      </ConfirmDialog>
    </>
  )
}

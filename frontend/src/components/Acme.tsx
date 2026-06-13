import { ConfirmDialog } from './acme/ConfirmDialog'
import { AcmePageHeader } from './acme/AcmePageHeader'
import { useAcmeData } from './acme/useAcmeData'
import { useAcmeActions } from './acme/useAcmeActions'
import { useAcmeUiState } from './acme/useAcmeUiState'
import { DomainList } from './acme/sections/DomainList'
import { TaskHistory } from './acme/sections/TaskHistory'
import { DomainOverlays } from './acme/overlays/DomainOverlays'
import { AccountCredentialOverlays } from './acme/overlays/AccountCredentialOverlays'
import { DeployTargetOverlays } from './acme/overlays/DeployTargetOverlays'
import { SSHDeployConfigEditDialog } from './acme/dialogs/SSHDeployConfigEditDialog'
import { SafelineDeployConfigEditDialog } from './acme/dialogs/SafelineDeployConfigEditDialog'
import { CASDeployConfigEditDialog } from './acme/dialogs/CASDeployConfigEditDialog'
import { FnOSDeployConfigEditDialog } from './acme/dialogs/FnOSDeployConfigEditDialog'
import { DeployConfigsDrawer } from './acme/drawers/DeployConfigsDrawer'

export default function Acme() {
  const {
    domains,
    accounts,
    sshTargets,
    safelineTargets,
    casTargets,
    fnosTargets,
    sshCredentials,
    providers,
    credentials,
    loading,
    accountSummary,
    reloadAll,
    reloadAccounts,
    reloadCredentials,
    reloadDeployTargets,
    reloadSSHCredentials,
    reloadDeployConfigs,
    tasks,
    taskPage,
    taskTotal,
    taskPageSize,
    taskStatus,
    loadTasks,
    reloadTasks,
    changeTaskPageSize,
    changeTaskStatus,
    deployConfigs,
    deployConfigLoading,
    safeDeployConfigs,
    safeDeployLoading,
    casDeployConfigs,
    casDeployLoading,
    fnosDeployConfigs,
    fnosDeployLoading,
  } = useAcmeData()
  const ui = useAcmeUiState()
  const actions = useAcmeActions({
    ui,
    reloadAll,
    reloadTasks,
    reloadCredentials,
    reloadDeployTargets,
    reloadSSHCredentials,
    reloadDeployConfigs,
  })

  return (
    <div className="mx-auto max-w-5xl px-4 pb-12 pt-4 sm:px-8 sm:pb-32 sm:pt-10">
      <AcmePageHeader
        loading={loading}
        onRefresh={reloadAll}
        onOpenCredentials={ui.credentials.drawer.show}
        onOpenAccounts={ui.accounts.drawer.show}
        onOpenDeployTargets={ui.targets.entry.show}
        onAddDomain={ui.domain.edit.add}
      />

      <DomainList
        domains={domains}
        loading={loading}
        busy={ui.busy}
        accountSummary={accountSummary}
        onIssue={actions.requestIssue}
        onDeploy={actions.openDeployConfigs}
        onEdit={ui.domain.edit.edit}
        onRevoke={ui.domain.revoke.setPending}
        onDelete={ui.domain.remove.setPending}
        onDownload={actions.downloadCert}
      />

      <TaskHistory
        tasks={tasks}
        loading={loading}
        taskStatus={taskStatus}
        onStatusChange={changeTaskStatus}
        taskPage={taskPage}
        taskPageSize={taskPageSize}
        taskTotal={taskTotal}
        onGo={(p) => void loadTasks(p)}
        onPageSizeChange={changeTaskPageSize}
        onShowLog={ui.log.setTaskID}
        onRetry={(id) => void actions.retryTask(id)}
        busy={ui.busy}
      />

      <DomainOverlays
        ui={ui}
        actions={actions}
        accounts={accounts}
        providers={providers}
        onSaved={reloadAll}
        onTasksReload={reloadTasks}
        onDeployConfigsReload={reloadDeployConfigs}
      />

      <AccountCredentialOverlays
        ui={ui}
        actions={actions}
        accounts={accounts}
        credentials={credentials}
        onAccountsReload={reloadAccounts}
        onCredentialsReload={reloadCredentials}
      />

      <DeployTargetOverlays
        ui={ui}
        actions={actions}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        sshCredentials={sshCredentials}
        onDeployTargetsReload={reloadDeployTargets}
        onSSHCredentialsReload={reloadSSHCredentials}
      />

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
        onSaved={() => {
          if (ui.deploy.entryDomain) void reloadDeployConfigs(ui.deploy.entryDomain.id)
        }}
      />

      <SafelineDeployConfigEditDialog
        open={ui.deploy.safeline.edit.open}
        onOpenChange={ui.deploy.safeline.edit.setOpen}
        domain={ui.deploy.safeline.domain}
        config={ui.deploy.safeline.edit.target}
        targets={safelineTargets}
        onSaved={() => {
          if (ui.deploy.entryDomain) void reloadDeployConfigs(ui.deploy.entryDomain.id)
        }}
      />

      <CASDeployConfigEditDialog
        open={ui.deploy.cas.edit.open}
        onOpenChange={ui.deploy.cas.edit.setOpen}
        domain={ui.deploy.cas.domain}
        config={ui.deploy.cas.edit.target}
        targets={casTargets}
        onSaved={() => {
          if (ui.deploy.entryDomain) void reloadDeployConfigs(ui.deploy.entryDomain.id)
        }}
      />

      <FnOSDeployConfigEditDialog
        open={ui.deploy.fnos.edit.open}
        onOpenChange={ui.deploy.fnos.edit.setOpen}
        domain={ui.deploy.fnos.domain}
        config={ui.deploy.fnos.edit.target}
        targets={fnosTargets}
        onSaved={() => {
          if (ui.deploy.entryDomain) void reloadDeployConfigs(ui.deploy.entryDomain.id)
        }}
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

    </div>
  )
}

import { AcmePageHeader } from './acme/AcmePageHeader'
import { useAcmeData } from './acme/useAcmeData'
import { useAcmeActions } from './acme/useAcmeActions'
import { useAcmeUiState } from './acme/useAcmeUiState'
import { DomainList } from './acme/sections/DomainList'
import { TaskHistory } from './acme/sections/TaskHistory'
import { DomainOverlays } from './acme/overlays/DomainOverlays'
import { AccountCredentialOverlays } from './acme/overlays/AccountCredentialOverlays'
import { DeployTargetOverlays } from './acme/overlays/DeployTargetOverlays'
import { DeployConfigOverlays } from './acme/overlays/DeployConfigOverlays'

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

      <DeployConfigOverlays
        ui={ui}
        actions={actions}
        deployConfigs={deployConfigs}
        deployConfigLoading={deployConfigLoading}
        safeDeployConfigs={safeDeployConfigs}
        safeDeployLoading={safeDeployLoading}
        casDeployConfigs={casDeployConfigs}
        casDeployLoading={casDeployLoading}
        fnosDeployConfigs={fnosDeployConfigs}
        fnosDeployLoading={fnosDeployLoading}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        onDeployConfigsReload={reloadDeployConfigs}
      />

    </div>
  )
}

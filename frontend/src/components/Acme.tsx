import { ConfirmDialog } from './acme/ConfirmDialog'
import { AcmePageHeader } from './acme/AcmePageHeader'
import { useAcmeData } from './acme/useAcmeData'
import { useAcmeActions } from './acme/useAcmeActions'
import { useAcmeUiState } from './acme/useAcmeUiState'
import { DomainList } from './acme/sections/DomainList'
import { TaskHistory } from './acme/sections/TaskHistory'
import { DomainOverlays } from './acme/overlays/DomainOverlays'
import { SSHTargetEditDialog } from './acme/dialogs/SSHTargetEditDialog'
import { SSHCredentialEditDialog } from './acme/dialogs/SSHCredentialEditDialog'
import { SafelineTargetEditDialog } from './acme/dialogs/SafelineTargetEditDialog'
import { AccountEditDialog } from './acme/dialogs/AccountEditDialog'
import { CredentialEditDialog } from './acme/dialogs/CredentialEditDialog'
import { SSHDeployConfigEditDialog } from './acme/dialogs/SSHDeployConfigEditDialog'
import { SafelineDeployConfigEditDialog } from './acme/dialogs/SafelineDeployConfigEditDialog'
import { CASTargetEditDialog } from './acme/dialogs/CASTargetEditDialog'
import { CASDeployConfigEditDialog } from './acme/dialogs/CASDeployConfigEditDialog'
import { FnOSTargetEditDialog } from './acme/dialogs/FnOSTargetEditDialog'
import { FnOSDeployConfigEditDialog } from './acme/dialogs/FnOSDeployConfigEditDialog'
import { CredentialsDrawer } from './acme/drawers/CredentialsDrawer'
import { AccountsDrawer } from './acme/drawers/AccountsDrawer'
import { DeployTargetsEntryDrawer } from './acme/drawers/DeployTargetsEntryDrawer'
import { SSHCredentialsDrawer } from './acme/drawers/SSHCredentialsDrawer'
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

      <CredentialsDrawer
        open={ui.credentials.drawer.open}
        onOpenChange={ui.credentials.drawer.setOpen}
        credentials={credentials}
        onAdd={ui.credentials.edit.add}
        onEdit={ui.credentials.edit.edit}
        onDelete={ui.credentials.remove.setPending}
      />

      <CredentialEditDialog
        open={ui.credentials.edit.open}
        onOpenChange={ui.credentials.edit.setOpen}
        target={ui.credentials.edit.target}
        onSaved={reloadCredentials}
      />

      <AccountsDrawer
        open={ui.accounts.drawer.open}
        onOpenChange={ui.accounts.drawer.setOpen}
        accounts={accounts}
        onAdd={ui.accounts.edit.add}
        onEdit={ui.accounts.edit.edit}
        onDelete={ui.accounts.remove.setPending}
      />

      <AccountEditDialog
        open={ui.accounts.edit.open}
        onOpenChange={ui.accounts.edit.setOpen}
        target={ui.accounts.edit.target}
        onSaved={reloadAccounts}
      />

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
        onSaved={reloadDeployTargets}
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
        onSaved={reloadSSHCredentials}
      />

      <SafelineTargetEditDialog
        open={ui.targets.safeline.edit.open}
        onOpenChange={ui.targets.safeline.edit.setOpen}
        target={ui.targets.safeline.edit.target}
        onSaved={reloadDeployTargets}
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

      <CASTargetEditDialog
        open={ui.targets.cas.edit.open}
        onOpenChange={ui.targets.cas.edit.setOpen}
        target={ui.targets.cas.edit.target}
        onSaved={reloadDeployTargets}
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

      <FnOSTargetEditDialog
        open={ui.targets.fnos.edit.open}
        onOpenChange={ui.targets.fnos.edit.setOpen}
        target={ui.targets.fnos.edit.target}
        credentials={sshCredentials}
        sshTargets={sshTargets}
        fnosTargets={fnosTargets}
        onManageCredentials={ui.sshCredentials.drawer.show}
        onSaved={reloadDeployTargets}
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

      <ConfirmDialog
        open={!!ui.accounts.remove.pending}
        onClose={ui.accounts.remove.clear}
        onConfirm={actions.onDeleteAccount}
        title="删除 CA 账号"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.accounts.remove.pending?.name}
        </span>{' '}
        账号；已被域名引用的账号不能删除。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.credentials.remove.pending}
        onClose={ui.credentials.remove.clear}
        onConfirm={actions.onDeleteCredential}
        title="删除 DNS 凭证"
      >
        即将删除 provider{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.credentials.remove.pending?.provider}
        </span>{' '}
        的凭证；已关联该 provider 的域名将无法继续签发，请确认。
      </ConfirmDialog>
    </div>
  )
}

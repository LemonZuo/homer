import {
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
} from 'lucide-react'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { ConfirmDialog } from './acme/ConfirmDialog'
import { cn } from '../lib/utils'
import { useAcmeData } from './acme/useAcmeData'
import { useAcmeActions } from './acme/useAcmeActions'
import { useAcmeUiState } from './acme/useAcmeUiState'
import { LogDrawer } from './acme/LogDrawer'
import { DomainCard } from './acme/sections/DomainCard'
import { TaskHistory } from './acme/sections/TaskHistory'
import { DomainEditDialog } from './acme/dialogs/DomainEditDialog'
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
  const cs = getColorSet('emerald')

  return (
    <div className="mx-auto max-w-5xl px-4 pb-12 pt-4 sm:px-8 sm:pb-32 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">ACME 签发</h1>
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
            onClick={reloadAll}
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
            onClick={ui.credentials.drawer.show}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            DNS 凭证
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={ui.accounts.drawer.show}
          >
            <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
            CA 账号
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={ui.targets.entry.show}
          >
            <Server className="mr-1.5 h-3.5 w-3.5" />
            部署目标
          </Button>
          <Button
            size="sm"
            className="hidden h-10 w-full sm:inline-flex sm:h-8 sm:w-auto"
            onClick={ui.domain.edit.add}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            新增域名
          </Button>
        </div>
      </div>

      <Button
        size="icon"
        onClick={ui.domain.edit.add}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增域名"
      >
        <Plus className="h-5 w-5" />
      </Button>

      <div className="mb-8 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {domains.map((d) => (
          <DomainCard
            key={d.id}
            d={d}
            busy={ui.busy}
            accountSummary={accountSummary}
            onIssue={actions.startIssue}
            onDeploy={actions.openDeployConfigs}
            onEdit={ui.domain.edit.edit}
            onRevoke={ui.domain.revoke.setPending}
            onDelete={ui.domain.remove.setPending}
            onDownload={actions.downloadCert}
          />
        ))}
        {!loading && domains.length === 0 && (
          <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
            还没有域名，点击右上「新增域名」开始
          </Card>
        )}
      </div>

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

      <DomainEditDialog
        open={ui.domain.edit.open}
        onOpenChange={ui.domain.edit.setOpen}
        target={ui.domain.edit.target}
        accounts={accounts.filter((a) => a.enabled || a.id === ui.domain.edit.target?.account_id)}
        providers={providers}
        onSaved={reloadAll}
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

      <LogDrawer
        taskID={ui.log.taskID}
        onClose={() => {
          ui.log.clear()
          void reloadTasks()
          void reloadAll()
          if (ui.deploy.entryDomain) void reloadDeployConfigs(ui.deploy.entryDomain.id)
        }}
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

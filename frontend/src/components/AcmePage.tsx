import { useState } from 'react'
import {
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { ConfirmDialog } from './acme/ConfirmDialog'
import { cn } from '../lib/utils'
import type {
  AcmeAccount,
  CASDeployConfig,
  CASTarget,
  Credential,
  Domain,
  FnOSDeployConfig,
  FnOSTarget,
  SSHCredential,
  SSHDeployConfig,
  SSHTarget,
  SafelineDeployConfig,
  SafelineTarget,
} from './acme/types'
import { useAcmeData } from './acme/useAcmeData'
import { makeDeleteHandler, type DeployConfigKind } from './acme/handlers'
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

export default function AcmePage() {
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
  const [busy, setBusy] = useState<string | null>(null)

  const [editOpen, setEditOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Domain | null>(null)
  const [deletePending, setDeletePending] = useState<Domain | null>(null)
  const [revokePending, setRevokePending] = useState<Domain | null>(null)
  const [deployDomain, setDeployDomain] = useState<Domain | null>(null)
  const [deployEditOpen, setDeployEditOpen] = useState(false)
  const [deployEditTarget, setDeployEditTarget] = useState<SSHDeployConfig | null>(null)
  const [deployDeletePending, setDeployDeletePending] = useState<SSHDeployConfig | null>(null)
  const [safeDeployDomain, setSafeDeployDomain] = useState<Domain | null>(null)
  const [safeDeployEditOpen, setSafeDeployEditOpen] = useState(false)
  const [safeDeployEditTarget, setSafeDeployEditTarget] = useState<SafelineDeployConfig | null>(null)
  const [safeDeployDeletePending, setSafeDeployDeletePending] = useState<SafelineDeployConfig | null>(null)
  const [casDeployDomain, setCASDeployDomain] = useState<Domain | null>(null)
  const [casDeployEditOpen, setCASDeployEditOpen] = useState(false)
  const [casDeployEditTarget, setCASDeployEditTarget] = useState<CASDeployConfig | null>(null)
  const [casDeployDeletePending, setCASDeployDeletePending] = useState<CASDeployConfig | null>(null)
  const [fnosDeployDomain, setFnOSDeployDomain] = useState<Domain | null>(null)
  const [fnosDeployEditOpen, setFnOSDeployEditOpen] = useState(false)
  const [fnosDeployEditTarget, setFnOSDeployEditTarget] = useState<FnOSDeployConfig | null>(null)
  const [fnosDeployDeletePending, setFnOSDeployDeletePending] = useState<FnOSDeployConfig | null>(null)
  const [logTaskID, setLogTaskID] = useState<number | null>(null)

  const [credDrawerOpen, setCredDrawerOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [credEditTarget, setCredEditTarget] = useState<Credential | null>(null)
  const [credDeletePending, setCredDeletePending] = useState<Credential | null>(null)
  const [accountDrawerOpen, setAccountDrawerOpen] = useState(false)
  const [accountEditOpen, setAccountEditOpen] = useState(false)
  const [accountEditTarget, setAccountEditTarget] = useState<AcmeAccount | null>(null)
  const [accountDeletePending, setAccountDeletePending] = useState<AcmeAccount | null>(null)
  const [targetEntryOpen, setTargetEntryOpen] = useState(false)
  const [deployEntryDomain, setDeployEntryDomain] = useState<Domain | null>(null)
  const [sshEditOpen, setSSHEditOpen] = useState(false)
  const [sshEditTarget, setSSHEditTarget] = useState<SSHTarget | null>(null)
  const [sshDeletePending, setSSHDeletePending] = useState<SSHTarget | null>(null)
  const [safeEditOpen, setSafeEditOpen] = useState(false)
  const [safeEditTarget, setSafeEditTarget] = useState<SafelineTarget | null>(null)
  const [safeDeletePending, setSafeDeletePending] = useState<SafelineTarget | null>(null)
  const [casEditOpen, setCASEditOpen] = useState(false)
  const [casEditTarget, setCASEditTarget] = useState<CASTarget | null>(null)
  const [casDeletePending, setCASDeletePending] = useState<CASTarget | null>(null)
  const [fnosEditOpen, setFnOSEditOpen] = useState(false)
  const [fnosEditTarget, setFnOSEditTarget] = useState<FnOSTarget | null>(null)
  const [fnosDeletePending, setFnOSDeletePending] = useState<FnOSTarget | null>(null)
  const [sshCredDrawerOpen, setSSHCredDrawerOpen] = useState(false)
  const [sshCredEditOpen, setSSHCredEditOpen] = useState(false)
  const [sshCredEditTarget, setSSHCredEditTarget] = useState<SSHCredential | null>(null)
  const [sshCredDeletePending, setSSHCredDeletePending] = useState<SSHCredential | null>(null)

  const cs = getColorSet('emerald')

  const onDeleteSSHCredential = makeDeleteHandler({
    get: () => sshCredDeletePending,
    clear: () => setSSHCredDeletePending(null),
    url: (c) => `/acme/ssh-credentials/${c.id}`,
    reload: reloadSSHCredentials,
  })

  const onDeleteCredential = makeDeleteHandler({
    get: () => credDeletePending,
    clear: () => setCredDeletePending(null),
    url: (c) => `/acme/credentials/${c.id}`,
    reload: reloadCredentials,
  })

  const onDeleteAccount = makeDeleteHandler({
    get: () => accountDeletePending,
    clear: () => setAccountDeletePending(null),
    url: (a) => `/acme/accounts/${a.id}`,
    reload: reloadAll,
  })

  const onDeleteSSHTarget = makeDeleteHandler({
    get: () => sshDeletePending,
    clear: () => setSSHDeletePending(null),
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteSafelineTarget = makeDeleteHandler({
    get: () => safeDeletePending,
    clear: () => setSafeDeletePending(null),
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteCASTarget = makeDeleteHandler({
    get: () => casDeletePending,
    clear: () => setCASDeletePending(null),
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteFnOSTarget = makeDeleteHandler({
    get: () => fnosDeletePending,
    clear: () => setFnOSDeletePending(null),
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const reloadEntryConfigs = () =>
    deployEntryDomain ? reloadDeployConfigs(deployEntryDomain.id) : undefined

  const onDeleteSSHDeployConfig = makeDeleteHandler({
    get: () => deployDeletePending,
    clear: () => setDeployDeletePending(null),
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteSafelineDeployConfig = makeDeleteHandler({
    get: () => safeDeployDeletePending,
    clear: () => setSafeDeployDeletePending(null),
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteCASDeployConfig = makeDeleteHandler({
    get: () => casDeployDeletePending,
    clear: () => setCASDeployDeletePending(null),
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteFnOSDeployConfig = makeDeleteHandler({
    get: () => fnosDeployDeletePending,
    clear: () => setFnOSDeployDeletePending(null),
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const startIssue = async (d: Domain) => {
    setBusy(`issue-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/issue`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交，任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交失败')
    } finally {
      setBusy(null)
    }
  }

  const startRevoke = async (d: Domain) => {
    setRevokePending(null)
    setBusy(`revoke-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/revoke`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交吊销，任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交吊销失败')
    } finally {
      setBusy(null)
    }
  }

  const downloadCert = (d: Domain) => {
    const a = document.createElement('a')
    a.href = `/api/acme/domains/${d.id}/cert/download`
    a.rel = 'noopener'
    a.click()
  }

  const openDeployConfigs = (d: Domain) => {
    setDeployEntryDomain(d)
    setDeployDomain(d)
    setSafeDeployDomain(d)
    setCASDeployDomain(d)
    setFnOSDeployDomain(d)
    void reloadDeployConfigs(d.id)
  }

  const DEPLOY_META: Record<
    DeployConfigKind,
    { word: string; reloadAfter: boolean; domain: () => Domain | null }
  > = {
    ssh: { word: ' SSH 部署', reloadAfter: false, domain: () => deployDomain },
    safeline: { word: '雷池部署', reloadAfter: true, domain: () => safeDeployDomain },
    cas: { word: ' CAS 上传', reloadAfter: true, domain: () => casDeployDomain },
    fnos: { word: ' fnOS 部署', reloadAfter: true, domain: () => fnosDeployDomain },
  }

  const startDeployConfig = async (kind: DeployConfigKind, cfg: { id: number }) => {
    const meta = DEPLOY_META[kind]
    const dom = meta.domain()
    if (!dom) return
    setBusy(`deploy-${kind}-config-${cfg.id}`)
    try {
      const { data } = await api.post(`/acme/deploy/configs/${cfg.id}/deploy`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交${meta.word}，任务 #${taskID}`)
      await reloadTasks()
      if (meta.reloadAfter) await reloadDeployConfigs(dom.id)
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || `提交${meta.word}失败`)
    } finally {
      setBusy(null)
    }
  }

  const startDeployAllConfigs = async () => {
    const d = deployEntryDomain
    if (!d) return
    setBusy(`deploy-domain-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/deploy-configs/deploy`)
      const taskIDs = (data?.data?.task_ids ?? []) as number[]
      toast.success(`已提交 ${taskIDs.length} 个部署任务`)
      await reloadTasks()
      await reloadDeployConfigs(d.id)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交一键部署失败')
    } finally {
      setBusy(null)
    }
  }

  const retryTask = async (taskID: number) => {
    setBusy(`retry-${taskID}`)
    try {
      await api.post(`/acme/tasks/${taskID}/retry`)
      toast.success(`已重试任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '重试失败')
    } finally {
      setBusy(null)
    }
  }

  const onDelete = async () => {
    const d = deletePending
    if (!d) return
    setDeletePending(null)
    setBusy(`del-${d.id}`)
    try {
      await api.delete(`/acme/domains/${d.id}`)
      toast.success('已删除')
      await reloadAll()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    } finally {
      setBusy(null)
    }
  }

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
            onClick={() => setCredDrawerOpen(true)}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            DNS 凭证
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={() => setAccountDrawerOpen(true)}
          >
            <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
            CA 账号
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={() => setTargetEntryOpen(true)}
          >
            <Server className="mr-1.5 h-3.5 w-3.5" />
            部署目标
          </Button>
          <Button
            size="sm"
            className="hidden h-10 w-full sm:inline-flex sm:h-8 sm:w-auto"
            onClick={() => {
              setEditTarget(null)
              setEditOpen(true)
            }}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            新增域名
          </Button>
        </div>
      </div>

      <Button
        size="icon"
        onClick={() => {
          setEditTarget(null)
          setEditOpen(true)
        }}
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
            busy={busy}
            accountSummary={accountSummary}
            onIssue={startIssue}
            onDeploy={openDeployConfigs}
            onEdit={(dd) => {
              setEditTarget(dd)
              setEditOpen(true)
            }}
            onRevoke={setRevokePending}
            onDelete={setDeletePending}
            onDownload={downloadCert}
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
        onShowLog={setLogTaskID}
        onRetry={(id) => void retryTask(id)}
        busy={busy}
      />

      <DomainEditDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        accounts={accounts.filter((a) => a.enabled || a.id === editTarget?.account_id)}
        providers={providers}
        onSaved={reloadAll}
      />

      <ConfirmDialog
        open={!!deletePending}
        onClose={() => setDeletePending(null)}
        onConfirm={onDelete}
        title="删除域名配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {deletePending?.main_domain}
        </span>{' '}
        的 ACME 配置、关联证书记录与任务流水。本地落盘的证书文件不会被删除。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!revokePending}
        onClose={() => setRevokePending(null)}
        onConfirm={() => {
          if (revokePending) void startRevoke(revokePending)
        }}
        title="吊销当前证书"
        confirmText="吊销"
      >
        即将向 CA 吊销{' '}
        <span className="font-mono font-medium text-foreground">
          {revokePending?.main_domain}
        </span>{' '}
        当前证书。吊销不可逆，且不会自动删除 CAS 证书或切换 CDN 配置。
      </ConfirmDialog>

      <LogDrawer
        taskID={logTaskID}
        onClose={() => {
          setLogTaskID(null)
          void reloadTasks()
          void reloadAll()
          if (deployEntryDomain) void reloadDeployConfigs(deployEntryDomain.id)
        }}
      />

      <CredentialsDrawer
        open={credDrawerOpen}
        onOpenChange={setCredDrawerOpen}
        credentials={credentials}
        onAdd={() => {
          setCredEditTarget(null)
          setCredEditOpen(true)
        }}
        onEdit={(c) => {
          setCredEditTarget(c)
          setCredEditOpen(true)
        }}
        onDelete={(c) => setCredDeletePending(c)}
      />

      <CredentialEditDialog
        open={credEditOpen}
        onOpenChange={setCredEditOpen}
        target={credEditTarget}
        onSaved={reloadCredentials}
      />

      <AccountsDrawer
        open={accountDrawerOpen}
        onOpenChange={setAccountDrawerOpen}
        accounts={accounts}
        onAdd={() => {
          setAccountEditTarget(null)
          setAccountEditOpen(true)
        }}
        onEdit={(a) => {
          setAccountEditTarget(a)
          setAccountEditOpen(true)
        }}
        onDelete={(a) => setAccountDeletePending(a)}
      />

      <AccountEditDialog
        open={accountEditOpen}
        onOpenChange={setAccountEditOpen}
        target={accountEditTarget}
        onSaved={reloadAccounts}
      />

      <DeployTargetsEntryDrawer
        open={targetEntryOpen}
        onOpenChange={setTargetEntryOpen}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        onAddSSH={() => {
          setSSHEditTarget(null)
          setSSHEditOpen(true)
        }}
        onEditSSH={(t) => {
          setSSHEditTarget(t)
          setSSHEditOpen(true)
        }}
        onDeleteSSH={(t) => setSSHDeletePending(t)}
        onManageCredentials={() => setSSHCredDrawerOpen(true)}
        onTestSSH={async (t) => {
          try {
            await api.post(`/acme/ssh-targets/${t.id}/test`)
            toast.success('连接正常')
          } catch (e: any) {
            toast.error(e?.response?.data?.error || e?.message || '连接失败')
          }
        }}
        onAddSafeline={() => {
          setSafeEditTarget(null)
          setSafeEditOpen(true)
        }}
        onEditSafeline={(t) => {
          setSafeEditTarget(t)
          setSafeEditOpen(true)
        }}
        onDeleteSafeline={(t) => setSafeDeletePending(t)}
        onTestSafeline={async (t) => {
          try {
            await api.post(`/acme/deploy/targets/${t.id}/test`)
            toast.success('连接正常')
          } catch (e: any) {
            toast.error(e?.response?.data?.error || e?.message || '连接失败')
          }
        }}
        onAddCAS={() => {
          setCASEditTarget(null)
          setCASEditOpen(true)
        }}
        onEditCAS={(t) => {
          setCASEditTarget(t)
          setCASEditOpen(true)
        }}
        onDeleteCAS={(t) => setCASDeletePending(t)}
        onTestCAS={async (t) => {
          try {
            await api.post(`/acme/deploy/targets/${t.id}/test`)
            toast.success('连接正常')
          } catch (e: any) {
            toast.error(e?.response?.data?.error || e?.message || '连接失败')
          }
        }}
        onAddFnOS={() => {
          setFnOSEditTarget(null)
          setFnOSEditOpen(true)
        }}
        onEditFnOS={(t) => {
          setFnOSEditTarget(t)
          setFnOSEditOpen(true)
        }}
        onDeleteFnOS={(t) => setFnOSDeletePending(t)}
        onTestFnOS={async (t) => {
          try {
            await api.post(`/acme/fnos-targets/${t.id}/test`)
            toast.success('连接正常')
          } catch (e: any) {
            toast.error(e?.response?.data?.error || e?.message || '连接失败')
          }
        }}
      />

      <SSHTargetEditDialog
        open={sshEditOpen}
        onOpenChange={setSSHEditOpen}
        target={sshEditTarget}
        credentials={sshCredentials}
        sshTargets={sshTargets}
        fnosTargets={fnosTargets}
        onManageCredentials={() => setSSHCredDrawerOpen(true)}
        onSaved={reloadDeployTargets}
      />

      <SSHCredentialsDrawer
        open={sshCredDrawerOpen}
        onOpenChange={setSSHCredDrawerOpen}
        credentials={sshCredentials}
        onAdd={() => {
          setSSHCredEditTarget(null)
          setSSHCredEditOpen(true)
        }}
        onEdit={(c) => {
          setSSHCredEditTarget(c)
          setSSHCredEditOpen(true)
        }}
        onDelete={(c) => setSSHCredDeletePending(c)}
      />

      <SSHCredentialEditDialog
        open={sshCredEditOpen}
        onOpenChange={setSSHCredEditOpen}
        target={sshCredEditTarget}
        onSaved={reloadSSHCredentials}
      />

      <SafelineTargetEditDialog
        open={safeEditOpen}
        onOpenChange={setSafeEditOpen}
        target={safeEditTarget}
        onSaved={reloadDeployTargets}
      />

      <DeployConfigsDrawer
        open={!!deployEntryDomain}
        onOpenChange={(o) => {
          if (!o) {
            setDeployEntryDomain(null)
            setDeployDomain(null)
            setSafeDeployDomain(null)
            setCASDeployDomain(null)
            setFnOSDeployDomain(null)
          }
        }}
        domain={deployEntryDomain}
        sshConfigs={deployConfigs}
        safelineConfigs={safeDeployConfigs}
        casConfigs={casDeployConfigs}
        fnosConfigs={fnosDeployConfigs}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        casTargets={casTargets}
        fnosTargets={fnosTargets}
        loading={deployConfigLoading || safeDeployLoading || casDeployLoading || fnosDeployLoading}
        busy={busy}
        onAddSSH={() => {
          setDeployEditTarget(null)
          setDeployEditOpen(true)
        }}
        onEditSSH={(cfg) => {
          setDeployEditTarget(cfg)
          setDeployEditOpen(true)
        }}
        onCopySSH={(cfg) => {
          setDeployEditTarget({
            ...cfg,
            id: 0,
            name: `${cfg.name || '配置'} 副本`,
            created_at: '',
            updated_at: '',
          })
          setDeployEditOpen(true)
        }}
        onDeleteSSH={(cfg) => setDeployDeletePending(cfg)}
        onDeploySSH={(cfg) => void startDeployConfig('ssh', cfg)}
        onAddSafeline={() => {
          setSafeDeployEditTarget(null)
          setSafeDeployEditOpen(true)
        }}
        onEditSafeline={(cfg) => {
          setSafeDeployEditTarget(cfg)
          setSafeDeployEditOpen(true)
        }}
        onDeleteSafeline={(cfg) => setSafeDeployDeletePending(cfg)}
        onDeploySafeline={(cfg) => void startDeployConfig('safeline', cfg)}
        onAddCAS={() => {
          setCASDeployEditTarget(null)
          setCASDeployEditOpen(true)
        }}
        onEditCAS={(cfg) => {
          setCASDeployEditTarget(cfg)
          setCASDeployEditOpen(true)
        }}
        onDeleteCAS={(cfg) => setCASDeployDeletePending(cfg)}
        onDeployCAS={(cfg) => void startDeployConfig('cas', cfg)}
        onAddFnOS={() => {
          setFnOSDeployEditTarget(null)
          setFnOSDeployEditOpen(true)
        }}
        onEditFnOS={(cfg) => {
          setFnOSDeployEditTarget(cfg)
          setFnOSDeployEditOpen(true)
        }}
        onDeleteFnOS={(cfg) => setFnOSDeployDeletePending(cfg)}
        onDeployFnOS={(cfg) => void startDeployConfig('fnos', cfg)}
        onDeployAll={() => void startDeployAllConfigs()}
      />

      <SSHDeployConfigEditDialog
        open={deployEditOpen}
        onOpenChange={setDeployEditOpen}
        domain={deployDomain}
        config={deployEditTarget}
        targets={sshTargets}
        onSaved={() => {
          if (deployEntryDomain) void reloadDeployConfigs(deployEntryDomain.id)
        }}
      />

      <SafelineDeployConfigEditDialog
        open={safeDeployEditOpen}
        onOpenChange={setSafeDeployEditOpen}
        domain={safeDeployDomain}
        config={safeDeployEditTarget}
        targets={safelineTargets}
        onSaved={() => {
          if (deployEntryDomain) void reloadDeployConfigs(deployEntryDomain.id)
        }}
      />

      <CASTargetEditDialog
        open={casEditOpen}
        onOpenChange={setCASEditOpen}
        target={casEditTarget}
        onSaved={reloadDeployTargets}
      />

      <CASDeployConfigEditDialog
        open={casDeployEditOpen}
        onOpenChange={setCASDeployEditOpen}
        domain={casDeployDomain}
        config={casDeployEditTarget}
        targets={casTargets}
        onSaved={() => {
          if (deployEntryDomain) void reloadDeployConfigs(deployEntryDomain.id)
        }}
      />

      <FnOSTargetEditDialog
        open={fnosEditOpen}
        onOpenChange={setFnOSEditOpen}
        target={fnosEditTarget}
        credentials={sshCredentials}
        sshTargets={sshTargets}
        fnosTargets={fnosTargets}
        onManageCredentials={() => setSSHCredDrawerOpen(true)}
        onSaved={reloadDeployTargets}
      />

      <FnOSDeployConfigEditDialog
        open={fnosDeployEditOpen}
        onOpenChange={setFnOSDeployEditOpen}
        domain={fnosDeployDomain}
        config={fnosDeployEditTarget}
        targets={fnosTargets}
        onSaved={() => {
          if (deployEntryDomain) void reloadDeployConfigs(deployEntryDomain.id)
        }}
      />

      <ConfirmDialog
        open={!!deployDeletePending}
        onClose={() => setDeployDeletePending(null)}
        onConfirm={onDeleteSSHDeployConfig}
        title="删除部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {deployDeletePending?.name || `#${deployDeletePending?.id}`}
        </span>{' '}
        的 SSH 部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!safeDeployDeletePending}
        onClose={() => setSafeDeployDeletePending(null)}
        onConfirm={onDeleteSafelineDeployConfig}
        title="删除雷池部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {safeDeployDeletePending?.name || `#${safeDeployDeletePending?.id}`}
        </span>{' '}
        的雷池部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!sshDeletePending}
        onClose={() => setSSHDeletePending(null)}
        onConfirm={onDeleteSSHTarget}
        title="删除 SSH 机器"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {sshDeletePending?.name}
        </span>{' '}
        的部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!safeDeletePending}
        onClose={() => setSafeDeletePending(null)}
        onConfirm={onDeleteSafelineTarget}
        title="删除雷池实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {safeDeletePending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!casDeletePending}
        onClose={() => setCASDeletePending(null)}
        onConfirm={onDeleteCASTarget}
        title="删除阿里云 CAS 实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {casDeletePending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!casDeployDeletePending}
        onClose={() => setCASDeployDeletePending(null)}
        onConfirm={onDeleteCASDeployConfig}
        title="删除阿里云 CAS 部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {casDeployDeletePending?.name || `#${casDeployDeletePending?.id}`}
        </span>{' '}
        的 CAS 部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!fnosDeletePending}
        onClose={() => setFnOSDeletePending(null)}
        onConfirm={onDeleteFnOSTarget}
        title="删除 fnOS 实例"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {fnosDeletePending?.name}
        </span>{' '}
        及其部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!fnosDeployDeletePending}
        onClose={() => setFnOSDeployDeletePending(null)}
        onConfirm={onDeleteFnOSDeployConfig}
        title="删除 fnOS 部署配置"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {fnosDeployDeletePending?.name || `#${fnosDeployDeletePending?.id}`}
        </span>{' '}
        的 fnOS 部署配置。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!sshCredDeletePending}
        onClose={() => setSSHCredDeletePending(null)}
        onConfirm={onDeleteSSHCredential}
        title="删除登录凭证"
      >
        即将删除登录凭证{' '}
        <span className="font-mono font-medium text-foreground">
          {sshCredDeletePending?.name}
        </span>
        ；引用了该凭证的机器将无法连接，请确认。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!accountDeletePending}
        onClose={() => setAccountDeletePending(null)}
        onConfirm={onDeleteAccount}
        title="删除 CA 账号"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {accountDeletePending?.name}
        </span>{' '}
        账号；已被域名引用的账号不能删除。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!credDeletePending}
        onClose={() => setCredDeletePending(null)}
        onConfirm={onDeleteCredential}
        title="删除 DNS 凭证"
      >
        即将删除 provider{' '}
        <span className="font-mono font-medium text-foreground">
          {credDeletePending?.provider}
        </span>{' '}
        的凭证；已关联该 provider 的域名将无法继续签发，请确认。
      </ConfirmDialog>
    </div>
  )
}

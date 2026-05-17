import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Ban,
  Edit3,
  KeyRound,
  Loader2,
  Play,
  Plus,
  RefreshCw,
  ScrollText,
  Send,
  Server,
  ShieldCheck,
  Trash2,
  UploadCloud,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { avatarColor, getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './ui/alert-dialog'
import { cn } from '../lib/utils'
import type {
  AcmeAccount,
  Credential,
  Domain,
  SSHCredential,
  SSHDeployConfig,
  SSHTarget,
  SafelineDeployConfig,
  SafelineTarget,
  Task,
} from './acme/types'
import {
  KIND_LABEL,
  STATUS_LABEL,
  STATUS_STYLE,
  TASK_PAGE_SIZES,
  TASK_PAGE_SIZE_KEY,
  caLabel,
  daysUntil,
  fmtDate,
  fmtDateTime,
  readTaskPageSize,
  splitDeployConfigs,
  splitDeployTargets,
} from './acme/utils'
import { FieldRow } from './acme/FieldRow'
import { TaskPager } from './acme/TaskPager'
import { LogDrawer } from './acme/LogDrawer'
import { DomainEditDialog } from './acme/dialogs/DomainEditDialog'
import { SSHTargetEditDialog } from './acme/dialogs/SSHTargetEditDialog'
import { SSHCredentialEditDialog } from './acme/dialogs/SSHCredentialEditDialog'
import { SafelineTargetEditDialog } from './acme/dialogs/SafelineTargetEditDialog'
import { AccountEditDialog } from './acme/dialogs/AccountEditDialog'
import { CredentialEditDialog } from './acme/dialogs/CredentialEditDialog'
import { SSHDeployConfigEditDialog } from './acme/dialogs/SSHDeployConfigEditDialog'
import { SafelineDeployConfigEditDialog } from './acme/dialogs/SafelineDeployConfigEditDialog'
import { CredentialsDrawer } from './acme/drawers/CredentialsDrawer'
import { AccountsDrawer } from './acme/drawers/AccountsDrawer'
import { DeployTargetsEntryDrawer } from './acme/drawers/DeployTargetsEntryDrawer'
import { SSHCredentialsDrawer } from './acme/drawers/SSHCredentialsDrawer'
import { DeployConfigsDrawer } from './acme/drawers/DeployConfigsDrawer'

export default function AcmePage() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [accounts, setAccounts] = useState<AcmeAccount[]>([])
  const [sshTargets, setSSHTargets] = useState<SSHTarget[]>([])
  const [safelineTargets, setSafelineTargets] = useState<SafelineTarget[]>([])
  const [sshCredentials, setSSHCredentials] = useState<SSHCredential[]>([])
  const [providers, setProviders] = useState<string[]>([])
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [taskPage, setTaskPage] = useState(1)
  const [taskTotal, setTaskTotal] = useState(0)
  const [taskPageSize, setTaskPageSize] = useState(readTaskPageSize)
  const taskPageRef = useRef(1)
  const taskPageSizeRef = useRef(readTaskPageSize())
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState<string | null>(null)

  const [editOpen, setEditOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Domain | null>(null)
  const [deletePending, setDeletePending] = useState<Domain | null>(null)
  const [revokePending, setRevokePending] = useState<Domain | null>(null)
  const [deployDomain, setDeployDomain] = useState<Domain | null>(null)
  const [deployConfigs, setDeployConfigs] = useState<SSHDeployConfig[]>([])
  const [deployConfigLoading, setDeployConfigLoading] = useState(false)
  const [deployEditOpen, setDeployEditOpen] = useState(false)
  const [deployEditTarget, setDeployEditTarget] = useState<SSHDeployConfig | null>(null)
  const [deployDeletePending, setDeployDeletePending] = useState<SSHDeployConfig | null>(null)
  const [safeDeployDomain, setSafeDeployDomain] = useState<Domain | null>(null)
  const [safeDeployConfigs, setSafeDeployConfigs] = useState<SafelineDeployConfig[]>([])
  const [safeDeployLoading, setSafeDeployLoading] = useState(false)
  const [safeDeployEditOpen, setSafeDeployEditOpen] = useState(false)
  const [safeDeployEditTarget, setSafeDeployEditTarget] = useState<SafelineDeployConfig | null>(null)
  const [safeDeployDeletePending, setSafeDeployDeletePending] = useState<SafelineDeployConfig | null>(null)
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
  const [sshCredDrawerOpen, setSSHCredDrawerOpen] = useState(false)
  const [sshCredEditOpen, setSSHCredEditOpen] = useState(false)
  const [sshCredEditTarget, setSSHCredEditTarget] = useState<SSHCredential | null>(null)
  const [sshCredDeletePending, setSSHCredDeletePending] = useState<SSHCredential | null>(null)

  const cs = getColorSet('emerald')
  const accountSummary = useMemo(() => {
    const m = new Map<number, AcmeAccount>()
    for (const a of accounts) m.set(a.id, a)
    return (id: number) => {
      const a = m.get(id)
      if (!a) return id ? `#${id}` : '未选择 CA'
      const ca = caLabel(a.ca)
      return a.name && a.name !== ca ? `${ca} / ${a.name}` : ca
    }
  }, [accounts])

  const reloadAll = useCallback(async () => {
    setLoading(true)
    try {
      const [d, p, t, c, a, targets, sc] = await Promise.all([
        api.get('/acme/domains'),
        api.get('/acme/providers'),
        api.get(`/acme/tasks?page=${taskPageRef.current}&page_size=${taskPageSizeRef.current}`),
        api.get('/acme/credentials'),
        api.get('/acme/accounts'),
        api.get('/acme/deploy/targets'),
        api.get('/acme/ssh-credentials'),
      ])
      const groupedTargets = splitDeployTargets(targets.data?.data ?? [])
      setDomains(d.data?.data ?? [])
      setProviders(p.data?.data ?? [])
      setTasks(t.data?.data ?? [])
      setTaskTotal(t.data?.total ?? 0)
      setTaskPage(taskPageRef.current)
      setCredentials(c.data?.data ?? [])
      setAccounts(a.data?.data ?? [])
      setSSHTargets(groupedTargets.ssh)
      setSafelineTargets(groupedTargets.safeline)
      setSSHCredentials(sc.data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  const reloadAccounts = useCallback(async () => {
    try {
      const { data } = await api.get('/acme/accounts')
      setAccounts(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载 ACME 账号失败')
    }
  }, [])

  const reloadCredentials = useCallback(async () => {
    try {
      const [p, c] = await Promise.all([
        api.get('/acme/providers'),
        api.get('/acme/credentials'),
      ])
      setProviders(p.data?.data ?? [])
      setCredentials(c.data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载凭证失败')
    }
  }, [])

  const reloadDeployTargets = useCallback(async () => {
    try {
      const { data } = await api.get('/acme/deploy/targets')
      const groupedTargets = splitDeployTargets(data?.data ?? [])
      setSSHTargets(groupedTargets.ssh)
      setSafelineTargets(groupedTargets.safeline)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载部署目标失败')
    }
  }, [])

  const reloadSSHCredentials = useCallback(async () => {
    try {
      const { data } = await api.get('/acme/ssh-credentials')
      setSSHCredentials(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载登录凭证失败')
    }
  }, [])

  const onDeleteSSHCredential = async () => {
    const c = sshCredDeletePending
    if (!c) return
    setSSHCredDeletePending(null)
    try {
      await api.delete(`/acme/ssh-credentials/${c.id}`)
      toast.success('已删除')
      await reloadSSHCredentials()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const reloadDeployConfigs = useCallback(async (domainID: number) => {
    setDeployConfigLoading(true)
    setSafeDeployLoading(true)
    try {
      const { data } = await api.get(`/acme/domains/${domainID}/deploy-configs`)
      const groupedConfigs = splitDeployConfigs(data?.data ?? [])
      setDeployConfigs(groupedConfigs.ssh)
      setSafeDeployConfigs(groupedConfigs.safeline)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载部署配置失败')
    } finally {
      setDeployConfigLoading(false)
      setSafeDeployLoading(false)
    }
  }, [])

  const onDeleteCredential = async () => {
    const c = credDeletePending
    if (!c) return
    setCredDeletePending(null)
    try {
      await api.delete(`/acme/credentials/${c.id}`)
      toast.success('已删除')
      await reloadCredentials()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteAccount = async () => {
    const a = accountDeletePending
    if (!a) return
    setAccountDeletePending(null)
    try {
      await api.delete(`/acme/accounts/${a.id}`)
      toast.success('已删除')
      await reloadAll()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteSSHTarget = async () => {
    const t = sshDeletePending
    if (!t) return
    setSSHDeletePending(null)
    try {
      await api.delete(`/acme/deploy/targets/${t.id}`)
      toast.success('已删除')
      await reloadDeployTargets()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteSafelineTarget = async () => {
    const t = safeDeletePending
    if (!t) return
    setSafeDeletePending(null)
    try {
      await api.delete(`/acme/deploy/targets/${t.id}`)
      toast.success('已删除')
      await reloadDeployTargets()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteSSHDeployConfig = async () => {
    const cfg = deployDeletePending
    if (!cfg) return
    setDeployDeletePending(null)
    try {
      await api.delete(`/acme/deploy/configs/${cfg.id}`)
      toast.success('已删除')
      if (deployEntryDomain) await reloadDeployConfigs(deployEntryDomain.id)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onDeleteSafelineDeployConfig = async () => {
    const cfg = safeDeployDeletePending
    if (!cfg) return
    setSafeDeployDeletePending(null)
    try {
      await api.delete(`/acme/deploy/configs/${cfg.id}`)
      toast.success('已删除')
      if (deployEntryDomain) await reloadDeployConfigs(deployEntryDomain.id)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  useEffect(() => {
    reloadAll()
  }, [reloadAll])

  const loadTasks = useCallback(async (page: number) => {
    try {
      const { data } = await api.get(
        `/acme/tasks?page=${page}&page_size=${taskPageSizeRef.current}`,
      )
      setTasks(data?.data ?? [])
      setTaskTotal(data?.total ?? 0)
      setTaskPage(page)
      taskPageRef.current = page
    } catch {
      /* silent */
    }
  }, [])

  const reloadTasks = useCallback(async () => {
    await loadTasks(taskPageRef.current)
  }, [loadTasks])

  const changeTaskPageSize = useCallback(
    (size: number) => {
      setTaskPageSize(size)
      taskPageSizeRef.current = size
      localStorage.setItem(TASK_PAGE_SIZE_KEY, String(size))
      void loadTasks(1)
    },
    [loadTasks],
  )

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

  const startUploadCAS = async (d: Domain) => {
    setBusy(`upload-cas-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/upload-cas`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交上传 CAS，任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交上传 CAS 失败')
    } finally {
      setBusy(null)
    }
  }

  const openDeployConfigs = (d: Domain) => {
    setDeployEntryDomain(d)
    setDeployDomain(d)
    setSafeDeployDomain(d)
    void reloadDeployConfigs(d.id)
  }

  const startDeploySSHConfig = async (cfg: SSHDeployConfig) => {
    if (!deployDomain) return
    setBusy(`deploy-ssh-config-${cfg.id}`)
    try {
      const { data } = await api.post(`/acme/deploy/configs/${cfg.id}/deploy`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交 SSH 部署，任务 #${taskID}`)
      await reloadTasks()
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交 SSH 部署失败')
    } finally {
      setBusy(null)
    }
  }

  const startDeploySafelineConfig = async (cfg: SafelineDeployConfig) => {
    if (!safeDeployDomain) return
    setBusy(`deploy-safeline-config-${cfg.id}`)
    try {
      const { data } = await api.post(`/acme/deploy/configs/${cfg.id}/deploy`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交雷池部署，任务 #${taskID}`)
      await reloadTasks()
      await reloadDeployConfigs(safeDeployDomain.id)
      setLogTaskID(taskID)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '提交雷池部署失败')
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
            自动签发与续期，并上传 CAS
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
        {domains.map((d) => {
          const days = daysUntil(d.not_after)
          const revoked = d.cert_status === 'revoked'
          const expiring = days !== null && days <= 30
          const expired = days !== null && days <= 0
          const certBadge = revoked
            ? { cls: 'bg-rose-500/10 text-rose-600 dark:text-rose-400', text: '已吊销' }
            : expired
            ? { cls: 'bg-rose-500/10 text-rose-600 dark:text-rose-400', text: '已过期' }
            : expiring
              ? {
                  cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
                  text: `${days} 天到期`,
                }
              : days !== null
                ? {
                    cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
                    text: `${days} 天`,
                  }
                : { cls: 'bg-muted text-muted-foreground', text: '未签发' }
          const issuing = busy === `issue-${d.id}`
          const uploadingCAS = busy === `upload-cas-${d.id}`
          const issueLabel = revoked || days !== null ? '重签' : '签发'
          return (
            <Card
              key={d.id}
              className={cn(
                'group flex h-full flex-col overflow-hidden transition-[transform,box-shadow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform hover:-translate-y-1',
                cs.border,
                cs.halo,
              )}
            >
              <div className="flex items-center gap-3 px-4 pt-4">
                <div
                  className={cn(
                    'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[13px] font-medium text-white shadow-sm',
                    avatarColor(d.main_domain),
                  )}
                >
                  {d.main_domain.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className="truncate text-[14px] font-semibold tracking-tight"
                    title={d.main_domain}
                  >
                    {d.main_domain}
                  </div>
                  <div className="mt-0.5 text-[12px] text-muted-foreground line-clamp-2 sm:line-clamp-none sm:truncate">
                    {accountSummary(d.account_id)} · {d.provider} · {d.enabled ? '自动续期' : '已停用'}
                  </div>
                </div>
                <span
                  className={cn(
                    'shrink-0 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                    certBadge.cls,
                  )}
                >
                  {certBadge.text}
                </span>
              </div>

              <div className="mt-3 space-y-0 px-4">
                <FieldRow label="SAN" value={d.san_domains} />
                <FieldRow label="到期" value={fmtDate(d.not_after)} />
                <FieldRow label="签发" value={fmtDate(d.issued_at)} />
                {revoked && <FieldRow label="吊销" value={fmtDate(d.revoked_at)} />}
              </div>

              <div className="mt-3 grid grid-cols-3 gap-2 px-4 pb-4 sm:flex sm:gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  className="h-9 sm:h-8 sm:flex-1"
                  onClick={() => startIssue(d)}
                  disabled={busy !== null}
                >
                  {issuing ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Play className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {issueLabel}
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="h-9 w-full sm:h-8 sm:w-8"
                  onClick={() => startUploadCAS(d)}
                  disabled={busy !== null || !d.not_after || revoked || !d.cas_enabled}
                  title={
                    !d.cas_enabled
                      ? '请先在编辑里开启「上传到阿里云 CAS」'
                      : !d.not_after
                        ? '当前域名还没有证书'
                        : revoked
                          ? '当前证书已吊销'
                          : '上传当前证书到 CAS'
                  }
                >
                  {uploadingCAS ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <UploadCloud className="h-3.5 w-3.5" />
                  )}
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="h-9 w-full sm:h-8 sm:w-8"
                  onClick={() => openDeployConfigs(d)}
                  disabled={busy !== null}
                  title="部署配置"
                >
                  <Send className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="h-9 w-full sm:h-8 sm:w-8"
                  onClick={() => {
                    setEditTarget(d)
                    setEditOpen(true)
                  }}
                  disabled={busy !== null}
                  title="编辑域名"
                >
                  <Edit3 className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="h-9 w-full hover:text-destructive sm:h-8 sm:w-8"
                  onClick={() => setRevokePending(d)}
                  disabled={busy !== null || !d.not_after || revoked}
                  title={!d.not_after ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : '吊销当前证书'}
                >
                  <Ban className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="h-9 w-full hover:text-destructive sm:h-8 sm:w-8"
                  onClick={() => setDeletePending(d)}
                  disabled={busy !== null}
                  title="删除域名"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </Card>
          )
        })}
        {!loading && domains.length === 0 && (
          <Card className="col-span-full px-4 py-12 text-center text-[12.5px] text-muted-foreground">
            还没有域名，点击右上「新增域名」开始
          </Card>
        )}
      </div>

      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-[14px] font-semibold tracking-tight">任务历史</h2>
        <div className="w-full sm:w-auto">
          <TaskPager
            page={taskPage}
            pageSize={taskPageSize}
            total={taskTotal}
            onGo={(p) => void loadTasks(p)}
            onPageSizeChange={changeTaskPageSize}
          />
        </div>
      </div>
      <div className="space-y-2">
        {tasks.map((t) => (
          <Card
            key={t.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 text-[12.5px]"
          >
            <span
              className={cn(
                'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                STATUS_STYLE[t.status] || 'bg-muted text-muted-foreground',
              )}
            >
              {STATUS_LABEL[t.status] || t.status}
            </span>
            <span className="font-mono">#{t.id}</span>
            <span className="font-medium">{t.main_domain}</span>
            <span className="text-muted-foreground">{KIND_LABEL[t.kind] || t.kind}</span>
            <span className="w-full font-mono text-[11.5px] text-muted-foreground sm:ml-auto sm:w-auto">
              {fmtDateTime(t.started_at)}
            </span>
            <Button
              size="sm"
              variant="outline"
              className="h-9 w-full sm:h-8 sm:w-auto"
              onClick={() => setLogTaskID(t.id)}
            >
              <ScrollText className="mr-1.5 h-3.5 w-3.5" />
              日志
            </Button>
          </Card>
        ))}
        {!loading && tasks.length === 0 && (
          <p className="py-6 text-center text-[12.5px] text-muted-foreground">
            还没有任务
          </p>
        )}
      </div>

      {taskTotal > TASK_PAGE_SIZES[0] && (
        <div className="mt-3 hidden justify-end sm:flex">
          <TaskPager
            page={taskPage}
            pageSize={taskPageSize}
            total={taskTotal}
            onGo={(p) => void loadTasks(p)}
            onPageSizeChange={changeTaskPageSize}
          />
        </div>
      )}

      <DomainEditDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        accounts={accounts.filter((a) => a.enabled || a.id === editTarget?.account_id)}
        providers={providers}
        onSaved={reloadAll}
      />

      <AlertDialog
        open={!!deletePending}
        onOpenChange={(o) => {
          if (!o) setDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除域名配置</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {deletePending?.main_domain}
              </span>{' '}
              的 ACME 配置、关联证书记录与任务流水。本地落盘的证书文件不会被删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!revokePending}
        onOpenChange={(o) => {
          if (!o) setRevokePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>吊销当前证书</AlertDialogTitle>
            <AlertDialogDescription>
              即将向 CA 吊销{' '}
              <span className="font-mono font-medium text-foreground">
                {revokePending?.main_domain}
              </span>{' '}
              当前证书。吊销不可逆，且不会自动删除 CAS 证书或切换 CDN 配置。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (revokePending) void startRevoke(revokePending)
              }}
            >
              吊销
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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
      />

      <SSHTargetEditDialog
        open={sshEditOpen}
        onOpenChange={setSSHEditOpen}
        target={sshEditTarget}
        credentials={sshCredentials}
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
          }
        }}
        domain={deployEntryDomain}
        sshConfigs={deployConfigs}
        safelineConfigs={safeDeployConfigs}
        sshTargets={sshTargets}
        safelineTargets={safelineTargets}
        loading={deployConfigLoading || safeDeployLoading}
        busy={busy}
        onAddSSH={() => {
          setDeployEditTarget(null)
          setDeployEditOpen(true)
        }}
        onEditSSH={(cfg) => {
          setDeployEditTarget(cfg)
          setDeployEditOpen(true)
        }}
        onDeleteSSH={(cfg) => setDeployDeletePending(cfg)}
        onDeploySSH={(cfg) => void startDeploySSHConfig(cfg)}
        onAddSafeline={() => {
          setSafeDeployEditTarget(null)
          setSafeDeployEditOpen(true)
        }}
        onEditSafeline={(cfg) => {
          setSafeDeployEditTarget(cfg)
          setSafeDeployEditOpen(true)
        }}
        onDeleteSafeline={(cfg) => setSafeDeployDeletePending(cfg)}
        onDeploySafeline={(cfg) => void startDeploySafelineConfig(cfg)}
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

      <AlertDialog
        open={!!deployDeletePending}
        onOpenChange={(o) => {
          if (!o) setDeployDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除部署配置</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {deployDeletePending?.name || `#${deployDeletePending?.id}`}
              </span>{' '}
              的 SSH 部署配置。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteSSHDeployConfig}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!safeDeployDeletePending}
        onOpenChange={(o) => {
          if (!o) setSafeDeployDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除雷池部署配置</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {safeDeployDeletePending?.name || `#${safeDeployDeletePending?.id}`}
              </span>{' '}
              的雷池部署配置。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteSafelineDeployConfig}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!sshDeletePending}
        onOpenChange={(o) => {
          if (!o) setSSHDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 SSH 机器</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {sshDeletePending?.name}
              </span>{' '}
              的部署配置。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteSSHTarget}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!safeDeletePending}
        onOpenChange={(o) => {
          if (!o) setSafeDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除雷池实例</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {safeDeletePending?.name}
              </span>{' '}
              及其部署配置。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteSafelineTarget}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!sshCredDeletePending}
        onOpenChange={(o) => {
          if (!o) setSSHCredDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除登录凭证</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除登录凭证{' '}
              <span className="font-mono font-medium text-foreground">
                {sshCredDeletePending?.name}
              </span>
              ；引用了该凭证的机器将无法连接，请确认。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteSSHCredential}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!accountDeletePending}
        onOpenChange={(o) => {
          if (!o) setAccountDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 CA 账号</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除{' '}
              <span className="font-mono font-medium text-foreground">
                {accountDeletePending?.name}
              </span>{' '}
              账号；已被域名引用的账号不能删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteAccount}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!credDeletePending}
        onOpenChange={(o) => {
          if (!o) setCredDeletePending(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 DNS 凭证</AlertDialogTitle>
            <AlertDialogDescription>
              即将删除 provider{' '}
              <span className="font-mono font-medium text-foreground">
                {credDeletePending?.provider}
              </span>{' '}
              的凭证；已关联该 provider 的域名将无法继续签发，请确认。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={onDeleteCredential}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

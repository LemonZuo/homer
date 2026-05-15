import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  RefreshCw,
  Loader2,
  Plus,
  Edit3,
  Trash2,
  Play,
  ScrollText,
  KeyRound,
  ShieldCheck,
  Ban,
  UploadCloud,
  Server,
  Send,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { avatarColor, getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Label } from './ui/label'
import { Switch } from './ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
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
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'
import { cn } from '../lib/utils'

interface Domain {
  id: number
  main_domain: string
  san_domains: string
  account_id: number
  provider: string
  enabled: boolean
  created_at: string
  updated_at: string
  not_before?: string
  not_after?: string
  cas_cert_id?: number
  cert_status?: string
  revoked_at?: string
  issued_at?: string
}

interface AcmeAccount {
  id: number
  name: string
  ca: 'letsencrypt' | 'zerossl' | 'custom' | string
  directory_url: string
  email: string
  eab_kid: string
  eab_hmac: string
  enabled: boolean
  created_at: string
  updated_at: string
}

interface Credential {
  id: number
  provider: string
  envs_json: string
  created_at: string
  updated_at: string
}

interface DeployTarget {
  id: number
  name: string
  kind: 'ssh' | 'safeline' | string
  endpoint: string
  auth_json: string
  config_json: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

interface SSHTarget {
  id: number
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key' | string
  password: string
  private_key: string
  passphrase: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

interface SSHDeployConfig {
  id: number
  domain_id: number
  target_id: number
  name: string
  cert_path: string
  key_path: string
  chain_path: string
  fullchain_path: string
  deploy_command: string
  auto_deploy: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

interface DeployConfig {
  id: number
  domain_id: number
  target_id: number
  kind: 'ssh' | 'safeline' | string
  name: string
  config_json: string
  state_json: string
  auto_deploy: boolean
  enabled: boolean
  created_at?: string
  updated_at?: string
}

interface SafelineTarget {
  id: number
  name: string
  base_url: string
  api_token: string
  skip_tls_verify: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

interface SafelineDeployConfig {
  id: number
  domain_id: number
  target_id: number
  name: string
  cert_id: number
  cert_type: number
  auto_deploy: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

interface Task {
  id: number
  domain_id: number
  main_domain: string
  kind: string
  status: string
  started_at: string
  finished_at: string | null
  log_text: string
  error_msg: string
}

function fmtDate(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function fmtDateTime(s?: string | null) {
  if (!s) return '—'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function daysUntil(s?: string | null): number | null {
  if (!s) return null
  const d = new Date(s)
  if (isNaN(d.getTime())) return null
  return Math.ceil((d.getTime() - Date.now()) / 86400000)
}

function safeParseJSON(s: string): Record<string, any> {
  try {
    const v = JSON.parse(s || '{}')
    return typeof v === 'object' && v !== null ? v : {}
  } catch {
    return {}
  }
}

function parseSSHEndpoint(endpoint: string): { host: string; port: number } {
  const value = endpoint.trim()
  const match = value.match(/^(.*):(\d+)$/)
  if (!match) return { host: value, port: 22 }
  return { host: match[1], port: Number(match[2]) || 22 }
}

function deployTargetToSSH(t: DeployTarget): SSHTarget {
  const auth = safeParseJSON(t.auth_json)
  const endpoint = parseSSHEndpoint(t.endpoint)
  return {
    id: t.id,
    name: t.name,
    host: endpoint.host,
    port: endpoint.port,
    username: String(auth.username ?? ''),
    auth_type: String(auth.auth_type ?? 'password'),
    password: String(auth.password ?? ''),
    private_key: String(auth.private_key ?? ''),
    passphrase: String(auth.passphrase ?? ''),
    enabled: t.enabled,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

function deployTargetToSafeline(t: DeployTarget): SafelineTarget {
  const auth = safeParseJSON(t.auth_json)
  const cfg = safeParseJSON(t.config_json)
  return {
    id: t.id,
    name: t.name,
    base_url: t.endpoint,
    api_token: String(auth.api_token ?? ''),
    skip_tls_verify: Boolean(cfg.skip_tls_verify),
    enabled: t.enabled,
    created_at: t.created_at ?? '',
    updated_at: t.updated_at ?? '',
  }
}

function splitDeployTargets(rows: DeployTarget[]) {
  return {
    ssh: rows.filter((t) => t.kind === 'ssh').map(deployTargetToSSH),
    safeline: rows.filter((t) => t.kind === 'safeline').map(deployTargetToSafeline),
  }
}

function sshTargetToDeployTarget(t: SSHTarget): DeployTarget {
  return {
    id: t.id,
    name: t.name,
    kind: 'ssh',
    endpoint: `${t.host}:${t.port || 22}`,
    auth_json: JSON.stringify({
      username: t.username,
      auth_type: t.auth_type,
      password: t.password,
      private_key: t.private_key,
      passphrase: t.passphrase,
    }),
    config_json: '{}',
    enabled: t.enabled,
  }
}

function safelineTargetToDeployTarget(t: SafelineTarget): DeployTarget {
  return {
    id: t.id,
    name: t.name,
    kind: 'safeline',
    endpoint: t.base_url,
    auth_json: JSON.stringify({ api_token: t.api_token }),
    config_json: JSON.stringify({ skip_tls_verify: t.skip_tls_verify }),
    enabled: t.enabled,
  }
}

function deployConfigToSSH(c: DeployConfig): SSHDeployConfig {
  const cfg = safeParseJSON(c.config_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    cert_path: String(cfg.cert_path ?? ''),
    key_path: String(cfg.key_path ?? ''),
    chain_path: String(cfg.chain_path ?? ''),
    fullchain_path: String(cfg.fullchain_path ?? ''),
    deploy_command: String(cfg.deploy_command ?? ''),
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
    created_at: c.created_at ?? '',
    updated_at: c.updated_at ?? '',
  }
}

function deployConfigToSafeline(c: DeployConfig): SafelineDeployConfig {
  const cfg = safeParseJSON(c.config_json)
  const state = safeParseJSON(c.state_json)
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    name: c.name,
    cert_id: Number(state.cert_id ?? 0) || 0,
    cert_type: Number(cfg.cert_type ?? 2) || 2,
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
    created_at: c.created_at ?? '',
    updated_at: c.updated_at ?? '',
  }
}

function splitDeployConfigs(rows: DeployConfig[]) {
  return {
    ssh: rows.filter((c) => c.kind === 'ssh').map(deployConfigToSSH),
    safeline: rows.filter((c) => c.kind === 'safeline').map(deployConfigToSafeline),
  }
}

function sshConfigToDeployConfig(c: SSHDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'ssh',
    name: c.name,
    config_json: JSON.stringify({
      cert_path: c.cert_path,
      key_path: c.key_path,
      chain_path: c.chain_path,
      fullchain_path: c.fullchain_path,
      deploy_command: c.deploy_command,
    }),
    state_json: '{}',
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

function safelineConfigToDeployConfig(c: SafelineDeployConfig): DeployConfig {
  return {
    id: c.id,
    domain_id: c.domain_id,
    target_id: c.target_id,
    kind: 'safeline',
    name: c.name,
    config_json: JSON.stringify({ cert_type: c.cert_type || 2 }),
    state_json: JSON.stringify({ cert_id: c.cert_id || 0 }),
    auto_deploy: c.auto_deploy,
    enabled: c.enabled,
  }
}

function FieldRow({ label, value }: { label: string; value: string }) {
  const empty = !value || !value.trim()
  return (
    <div className="flex items-center gap-3 py-1 text-[12.5px]">
      <span className="w-16 shrink-0 text-muted-foreground">{label}</span>
      {empty ? (
        <span className="min-w-0 flex-1 text-muted-foreground/70">—</span>
      ) : (
        <span
          className="min-w-0 flex-1 truncate font-mono text-[12.5px] leading-relaxed"
          title={value}
        >
          {value}
        </span>
      )}
    </div>
  )
}

const STATUS_STYLE: Record<string, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  failed: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
}

const STATUS_LABEL: Record<string, string> = {
  pending: '待运行',
  running: '运行中',
  success: '成功',
  failed: '失败',
}

const KIND_LABEL: Record<string, string> = {
  issue: '签发',
  renew: '续期',
  revoke: '吊销',
  upload_cas: '上传 CAS',
  deploy_ssh: '部署 SSH',
  deploy_safeline: '部署雷池',
}

const TASK_PAGE_SIZES = [5, 10, 20, 50, 100]
const TASK_PAGE_SIZE_KEY = 'acme.taskPageSize'

function readTaskPageSize(): number {
  const v = Number(localStorage.getItem(TASK_PAGE_SIZE_KEY))
  return TASK_PAGE_SIZES.includes(v) ? v : TASK_PAGE_SIZES[0]
}

function TaskPager({
  page,
  pageSize,
  total,
  onGo,
  onPageSizeChange,
}: {
  page: number
  pageSize: number
  total: number
  onGo: (page: number) => void
  onPageSizeChange: (size: number) => void
}) {
  if (total <= TASK_PAGE_SIZES[0]) return null
  const pages = Math.ceil(total / pageSize)
  return (
    <div className="flex items-center gap-3 text-[12px] text-muted-foreground">
      <span className="font-mono">
        {page} / {pages}（共 {total} 条）
      </span>
      <select
        className="h-8 rounded-md border border-input bg-background px-2 text-[12px]"
        value={pageSize}
        onChange={(e) => onPageSizeChange(Number(e.target.value))}
      >
        {TASK_PAGE_SIZES.map((s) => (
          <option key={s} value={s}>
            {s} 条/页
          </option>
        ))}
      </select>
      <Button
        size="sm"
        variant="outline"
        disabled={page <= 1}
        onClick={() => onGo(page - 1)}
      >
        <ChevronLeft className="mr-1 h-3.5 w-3.5" />
        上一页
      </Button>
      <Button
        size="sm"
        variant="outline"
        disabled={page >= pages}
        onClick={() => onGo(page + 1)}
      >
        下一页
        <ChevronRight className="ml-1 h-3.5 w-3.5" />
      </Button>
    </div>
  )
}

export default function AcmePage() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [accounts, setAccounts] = useState<AcmeAccount[]>([])
  const [sshTargets, setSSHTargets] = useState<SSHTarget[]>([])
  const [safelineTargets, setSafelineTargets] = useState<SafelineTarget[]>([])
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
      const [d, p, t, c, a, targets] = await Promise.all([
        api.get('/acme/domains'),
        api.get('/acme/providers'),
        api.get(`/acme/tasks?page=${taskPageRef.current}&page_size=${taskPageSizeRef.current}`),
        api.get('/acme/credentials'),
        api.get('/acme/accounts'),
        api.get('/acme/deploy/targets'),
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
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-[17px] font-semibold tracking-tight">ACME 签发</h1>
          <p className="mt-0.5 text-[12.5px] text-muted-foreground">
            自动签发与续期，并上传 CAS
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
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
            onClick={() => setCredDrawerOpen(true)}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            DNS 凭证
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAccountDrawerOpen(true)}
          >
            <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
            CA 账号
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setTargetEntryOpen(true)}
          >
            <Server className="mr-1.5 h-3.5 w-3.5" />
            部署目标
          </Button>
          <Button
            size="sm"
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
                  <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
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

              <div className="mt-3 flex gap-2 px-4 pb-4">
                <Button
                  size="sm"
                  variant="outline"
                  className="flex-1"
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
                  onClick={() => startUploadCAS(d)}
                  disabled={busy !== null || !d.not_after || revoked}
                  title={!d.not_after ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : '上传当前证书到 CAS'}
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
                  onClick={() => openDeployConfigs(d)}
                  disabled={busy !== null}
                  title="部署配置"
                >
                  <Send className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  onClick={() => {
                    setEditTarget(d)
                    setEditOpen(true)
                  }}
                  disabled={busy !== null}
                >
                  <Edit3 className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="hover:text-destructive"
                  onClick={() => setRevokePending(d)}
                  disabled={busy !== null || !d.not_after || revoked}
                  title={!d.not_after ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : '吊销当前证书'}
                >
                  <Ban className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="icon"
                  variant="outline"
                  className="hover:text-destructive"
                  onClick={() => setDeletePending(d)}
                  disabled={busy !== null}
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
        <TaskPager
          page={taskPage}
          pageSize={taskPageSize}
          total={taskTotal}
          onGo={(p) => void loadTasks(p)}
          onPageSizeChange={changeTaskPageSize}
        />
      </div>
      <div className="space-y-2">
        {tasks.map((t) => (
          <Card
            key={t.id}
            className="flex flex-wrap items-center gap-3 px-4 py-3 text-[12.5px]"
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
            <span className="ml-auto font-mono text-[11.5px] text-muted-foreground">
              {fmtDateTime(t.started_at)}
            </span>
            <Button
              size="sm"
              variant="outline"
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
        <div className="mt-3 flex justify-end">
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
          // 关闭后刷新任务列表，状态可能已变
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
        onSaved={reloadDeployTargets}
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

interface EditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Domain | null
  accounts: AcmeAccount[]
  providers: string[]
  onSaved: () => void
}

// 校验 DNS 名：可选 `*.` 通配符前缀 + 至少两段 label，label 不超过 63 字符
const DOMAIN_RE =
  /^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/

function isValidDomain(s: string): boolean {
  const v = s.trim().toLowerCase()
  if (v.length === 0 || v.length > 253) return false
  return DOMAIN_RE.test(v)
}

function DomainEditDialog({ open, onOpenChange, target, accounts, providers, onSaved }: EditProps) {
  const [domains, setDomains] = useState<string[]>([])
  const [draft, setDraft] = useState('')
  const [accountID, setAccountID] = useState<number>(0)
  const [provider, setProvider] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [draftError, setDraftError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    if (target) {
      const main = target.main_domain ? [target.main_domain] : []
      const sans = (target.san_domains || '')
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      setDomains([...main, ...sans])
      setAccountID(target.account_id || accounts[0]?.id || 0)
      setProvider(target.provider || providers[0] || '')
      setEnabled(target.enabled)
    } else {
      setDomains([])
      setAccountID(accounts[0]?.id || 0)
      setProvider(providers[0] || '')
      setEnabled(true)
    }
    setDraft('')
    setDraftError(null)
  }, [open, target, accounts, providers])

  const commitDraft = (): boolean => {
    const parts = draft
      .split(/[\s,;]+/)
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
    if (parts.length === 0) {
      setDraftError(null)
      return true
    }
    const invalid = parts.filter((p) => !isValidDomain(p))
    if (invalid.length > 0) {
      setDraftError(`格式不合法：${invalid.join(', ')}`)
      return false
    }
    setDomains((prev) => {
      const seen = new Set(prev)
      const merged = [...prev]
      for (const p of parts) {
        if (!seen.has(p)) {
          merged.push(p)
          seen.add(p)
        }
      }
      return merged
    })
    setDraft('')
    setDraftError(null)
    return true
  }

  const removeDomain = (i: number) =>
    setDomains((prev) => prev.filter((_, idx) => idx !== i))

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
      e.preventDefault()
      commitDraft()
    } else if (e.key === 'Backspace' && draft === '' && domains.length > 0) {
      e.preventDefault()
      setDomains((prev) => prev.slice(0, -1))
    }
  }

  const save = async () => {
    // 草稿里可能还有内容，先校验
    const draftParts = draft
      .split(/[\s,;]+/)
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
    const invalid = draftParts.filter((p) => !isValidDomain(p))
    if (invalid.length > 0) {
      setDraftError(`格式不合法：${invalid.join(', ')}`)
      return
    }
    const all = Array.from(new Set([...domains, ...draftParts]))
    if (all.length === 0) {
      toast.error('至少填一个域名')
      return
    }
    if (!provider) {
      toast.error('provider 必填')
      return
    }
    if (!accountID) {
      toast.error('CA 账号必填')
      return
    }
    const payload = {
      main_domain: all[0],
      san_domains: all.slice(1).join(','),
      account_id: accountID,
      provider,
      enabled,
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/domains/${target.id}`, payload)
      } else {
        await api.post('/acme/domains', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{target ? '编辑域名' : '新增域名'}</DialogTitle>
          <DialogDescription>
            第一个域名作为主域名，其余作为 SAN；输入后回车 / 空格 / 逗号添加
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label>域名</Label>
            <div
              className="flex min-h-[36px] w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-background px-2 py-1.5 text-[13px] focus-within:ring-2 focus-within:ring-ring"
              onClick={() => inputRef.current?.focus()}
            >
              {domains.map((d, i) => (
                <span
                  key={`${d}-${i}`}
                  className={cn(
                    'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 font-mono text-[11.5px]',
                    i === 0
                      ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                      : 'bg-muted text-foreground',
                  )}
                  title={i === 0 ? '主域名' : 'SAN'}
                >
                  {d}
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation()
                      removeDomain(i)
                    }}
                    className="text-muted-foreground hover:text-destructive"
                    aria-label={`移除 ${d}`}
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                ref={inputRef}
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value)
                  if (draftError) setDraftError(null)
                }}
                onKeyDown={onKeyDown}
                onBlur={() => commitDraft()}
                placeholder={domains.length === 0 ? 'example.com（回车添加，可继续添加 *.example.com）' : ''}
                className="min-w-[160px] flex-1 bg-transparent font-mono text-[12px] outline-none placeholder:text-muted-foreground"
              />
            </div>
            {draftError && (
              <p className="text-[11.5px] text-rose-600 dark:text-rose-400">{draftError}</p>
            )}
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="account">CA 账号</Label>
            <select
              id="account"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={accountID ? String(accountID) : ''}
              onChange={(e) => setAccountID(Number(e.target.value))}
            >
              {accounts.length === 0 && <option value="">（暂无账号，请先添加）</option>}
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} ({caLabel(a.ca)})
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="provider">DNS Provider</Label>
            <select
              id="provider"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
            >
              {providers.length === 0 && <option value="">（暂无凭证，请先添加）</option>}
              {providers.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="enabled">启用自动续期</Label>
            <Switch
              id="enabled"
              checked={enabled}
              onChange={(v) => setEnabled(v)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function LogDrawer({ taskID, onClose }: { taskID: number | null; onClose: () => void }) {
  const [lines, setLines] = useState<string[]>([])
  const [done, setDone] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const open = taskID !== null

  useEffect(() => {
    if (taskID === null) return
    setLines([])
    setDone(null)
    const es = new EventSource(`/api/acme/tasks/${taskID}/stream`)
    es.addEventListener('log', (ev: MessageEvent) => {
      setLines((prev) => [...prev, ev.data])
    })
    es.addEventListener('done', (ev: MessageEvent) => {
      setDone(ev.data)
      es.close()
    })
    es.onerror = () => {
      es.close()
    }
    return () => es.close()
  }, [taskID])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines, done])

  const title = useMemo(() => (taskID ? `任务 #${taskID} 日志` : '日志'), [taskID])

  return (
    <Drawer
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
    >
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{title}</DrawerTitle>
          <DrawerDescription>
            {done
              ? `状态：${STATUS_LABEL[done] || done}`
              : '实时推送（SSE）—— 关闭后可在任务历史里重看完整日志'}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 overflow-auto px-4 pb-4">
          <pre className="whitespace-pre-wrap break-all rounded-lg border border-border bg-muted/40 p-3 font-mono text-[11.5px] leading-relaxed">
            {lines.length === 0 ? '（暂无日志）' : lines.join('\n')}
            <div ref={bottomRef} />
          </pre>
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface CredentialsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  credentials: Credential[]
  onAdd: () => void
  onEdit: (c: Credential) => void
  onDelete: (c: Credential) => void
}

function CredentialsDrawer({
  open,
  onOpenChange,
  credentials,
  onAdd,
  onEdit,
  onDelete,
}: CredentialsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>DNS 凭证</DrawerTitle>
          <DrawerDescription>
            按 lego provider key 维护环境变量；保存后立刻可用于签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加凭证
            </Button>
          </div>
          {credentials.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有凭证，点击「添加凭证」开始
            </p>
          ) : (
            credentials.map((c) => (
              <Card key={c.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="font-mono text-[13px] font-medium">{c.provider}</div>
                    <div
                      className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground"
                      title={c.envs_json}
                    >
                      {Object.keys(safeParseEnvs(c.envs_json)).join(', ') || '（空）'}
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onEdit(c)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(c)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

function caLabel(ca: string) {
  switch (ca) {
    case 'letsencrypt':
      return "Let's Encrypt"
    case 'zerossl':
      return 'ZeroSSL'
    case 'custom':
      return '自定义'
    default:
      return ca || '未知'
  }
}

interface AccountsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  accounts: AcmeAccount[]
  onAdd: () => void
  onEdit: (a: AcmeAccount) => void
  onDelete: (a: AcmeAccount) => void
}

function AccountsDrawer({
  open,
  onOpenChange,
  accounts,
  onAdd,
  onEdit,
  onDelete,
}: AccountsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>CA 账号</DrawerTitle>
          <DrawerDescription>
            维护 ACME CA、邮箱与 ZeroSSL EAB；域名可选择不同账号签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加账号
            </Button>
          </div>
          {accounts.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 CA 账号，点击「添加账号」开始
            </p>
          ) : (
            accounts.map((a) => (
              <Card key={a.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">
                        {a.name}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          a.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {a.enabled ? '启用' : '停用'}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                      {caLabel(a.ca)} · {a.email}
                    </div>
                    {a.ca === 'custom' && (
                      <div
                        className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                        title={a.directory_url}
                      >
                        {a.directory_url}
                      </div>
                    )}
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onEdit(a)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(a)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface DeployTargetsEntryDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  onAddSSH: () => void
  onEditSSH: (t: SSHTarget) => void
  onDeleteSSH: (t: SSHTarget) => void
  onAddSafeline: () => void
  onEditSafeline: (t: SafelineTarget) => void
  onDeleteSafeline: (t: SafelineTarget) => void
  onTestSafeline: (t: SafelineTarget) => void
}

function DeployTargetsEntryDrawer({
  open,
  onOpenChange,
  sshTargets,
  safelineTargets,
  onAddSSH,
  onEditSSH,
  onDeleteSSH,
  onAddSafeline,
  onEditSafeline,
  onDeleteSafeline,
  onTestSafeline,
}: DeployTargetsEntryDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>部署目标</DrawerTitle>
          <DrawerDescription>
            管理证书部署时可选择的远程目标
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-5 overflow-auto px-4 pb-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-[13px] font-medium">SSH 机器</div>
              <div className="text-[11.5px] text-muted-foreground">{sshTargets.length} 台机器</div>
            </div>
            <Button size="sm" onClick={onAddSSH}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加机器
            </Button>
          </div>
          {sshTargets.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 SSH 机器
            </p>
          ) : (
            <div className="space-y-2">
              {sshTargets.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex items-center gap-3">
                    <Server className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">{t.name}</span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            t.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {t.enabled ? '启用' : '停用'}
                        </span>
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {t.username}@{t.host}:{t.port || 22} · {authLabel(t.auth_type)}
                      </div>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => onEditSSH(t)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="hover:text-destructive"
                      onClick={() => onDeleteSSH(t)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between gap-3 border-t border-border pt-5">
            <div>
              <div className="text-[13px] font-medium">雷池 WAF</div>
              <div className="text-[11.5px] text-muted-foreground">{safelineTargets.length} 个实例</div>
            </div>
            <Button size="sm" onClick={onAddSafeline}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加实例
            </Button>
          </div>
          {safelineTargets.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有雷池实例
            </p>
          ) : (
            <div className="space-y-2">
              {safelineTargets.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex items-center gap-3">
                    <ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">{t.name}</span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            t.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {t.enabled ? '启用' : '停用'}
                        </span>
                        {t.skip_tls_verify && (
                          <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
                            跳过 TLS
                          </span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {t.base_url}
                      </div>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => onTestSafeline(t)} disabled={!t.enabled}>
                      <RefreshCw className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => onEditSafeline(t)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="hover:text-destructive"
                      onClick={() => onDeleteSafeline(t)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface DeployConfigsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  sshConfigs: SSHDeployConfig[]
  safelineConfigs: SafelineDeployConfig[]
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  loading: boolean
  busy: string | null
  onAddSSH: () => void
  onEditSSH: (cfg: SSHDeployConfig) => void
  onDeleteSSH: (cfg: SSHDeployConfig) => void
  onDeploySSH: (cfg: SSHDeployConfig) => void
  onAddSafeline: () => void
  onEditSafeline: (cfg: SafelineDeployConfig) => void
  onDeleteSafeline: (cfg: SafelineDeployConfig) => void
  onDeploySafeline: (cfg: SafelineDeployConfig) => void
  onDeployAll: () => void
}

function DeployConfigsDrawer({
  open,
  onOpenChange,
  domain,
  sshConfigs,
  safelineConfigs,
  sshTargets,
  safelineTargets,
  loading,
  busy,
  onAddSSH,
  onEditSSH,
  onDeleteSSH,
  onDeploySSH,
  onAddSafeline,
  onEditSafeline,
  onDeleteSafeline,
  onDeploySafeline,
  onDeployAll,
}: DeployConfigsDrawerProps) {
  const revoked = domain?.cert_status === 'revoked'
  const hasCert = Boolean(domain?.not_after)
  const sshDeployableCount = sshConfigs.filter((cfg) => {
    const t = targetByID(sshTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const safelineDeployableCount = safelineConfigs.filter((cfg) => {
    const t = safelineTargetByID(safelineTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const deployableCount = sshDeployableCount + safelineDeployableCount
  const deployingAll = Boolean(domain && busy === `deploy-domain-${domain.id}`)
  const canDeployAll = hasCert && !revoked && deployableCount > 0 && busy === null

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>部署配置</DrawerTitle>
          <DrawerDescription>
            {domain?.main_domain ?? '当前域名'} 的证书部署配置
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-5 overflow-auto px-4 pb-4">
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={onDeployAll}
              disabled={!canDeployAll}
              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : deployableCount === 0 ? '没有可部署的启用配置' : undefined}
            >
              {deployingAll ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Send className="mr-1.5 h-3.5 w-3.5" />
              )}
              一键部署
            </Button>
            <Button size="sm" onClick={onAddSSH} disabled={sshTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加 SSH
            </Button>
            <Button size="sm" onClick={onAddSafeline} disabled={safelineTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加雷池
            </Button>
          </div>

          {loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <>
              <section className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-[13px] font-medium">SSH 部署</div>
                    <div className="text-[11.5px] text-muted-foreground">
                      写入远程文件并执行命令
                    </div>
                  </div>
                </div>
                {sshTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加 SSH 机器，再配置部署路径
                  </p>
                ) : sshConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有 SSH 部署配置
                  </p>
                ) : (
                  sshConfigs.map((cfg) => {
                    const t = targetByID(sshTargets, cfg.target_id)
                    const deploying = busy === `deploy-ssh-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex items-start gap-3">
                          <Send className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {configTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {targetSummary(t)}
                            </div>
                            <div
                              className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                              title={configPrimaryPath(cfg)}
                            >
                              {configPrimaryPath(cfg)}
                            </div>
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => onDeploySSH(cfg)}
                            disabled={!canDeploy}
                            title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                          >
                            {deploying ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Send className="h-3.5 w-3.5" />
                            )}
                          </Button>
                          <Button size="sm" variant="outline" onClick={() => onEditSSH(cfg)}>
                            <Edit3 className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="hover:text-destructive"
                            onClick={() => onDeleteSSH(cfg)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>

              <section className="space-y-2 border-t border-border pt-5">
                <div>
                  <div className="text-[13px] font-medium">雷池部署</div>
                  <div className="text-[11.5px] text-muted-foreground">
                    上传到 WAF 证书管理
                  </div>
                </div>
                {safelineTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加雷池实例，再配置证书上传
                  </p>
                ) : safelineConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有雷池部署配置
                  </p>
                ) : (
                  safelineConfigs.map((cfg) => {
                    const t = safelineTargetByID(safelineTargets, cfg.target_id)
                    const deploying = busy === `deploy-safeline-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex items-start gap-3">
                          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {safelineConfigTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {safelineTargetSummary(t)}
                            </div>
                            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                              cert_id={cfg.cert_id || '新增'} · type={cfg.cert_type || 2}
                            </div>
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => onDeploySafeline(cfg)}
                            disabled={!canDeploy}
                            title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                          >
                            {deploying ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <ShieldCheck className="h-3.5 w-3.5" />
                            )}
                          </Button>
                          <Button size="sm" variant="outline" onClick={() => onEditSafeline(cfg)}>
                            <Edit3 className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="hover:text-destructive"
                            onClick={() => onDeleteSafeline(cfg)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>
            </>
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface SSHDeployConfigsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  configs: SSHDeployConfig[]
  targets: SSHTarget[]
  loading: boolean
  busy: string | null
  onAdd: () => void
  onEdit: (cfg: SSHDeployConfig) => void
  onDelete: (cfg: SSHDeployConfig) => void
  onDeploy: (cfg: SSHDeployConfig) => void
  onDeployAll: () => void
}

function targetByID(targets: SSHTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

function targetSummary(t?: SSHTarget) {
  if (!t) return 'SSH 机器不存在'
  return `${t.name} · ${t.username}@${t.host}:${t.port || 22}`
}

function configTitle(cfg: SSHDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
}

function configPrimaryPath(cfg: SSHDeployConfig) {
  return cfg.fullchain_path || cfg.cert_path || cfg.key_path || '未配置路径'
}

export function SSHDeployConfigsDrawer({
  open,
  onOpenChange,
  domain,
  configs,
  targets,
  loading,
  busy,
  onAdd,
  onEdit,
  onDelete,
  onDeploy,
  onDeployAll,
}: SSHDeployConfigsDrawerProps) {
  const revoked = domain?.cert_status === 'revoked'
  const hasCert = Boolean(domain?.not_after)
  const deployableCount = configs.filter((cfg) => {
    const t = targetByID(targets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const deployingAll = Boolean(domain && busy === `deploy-ssh-domain-${domain.id}`)
  const canDeployAll = hasCert && !revoked && deployableCount > 0 && busy === null
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>部署配置</DrawerTitle>
          <DrawerDescription>
            {domain?.main_domain ?? '当前域名'} 的 SSH 部署路径、命令和自动部署策略
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={onDeployAll}
              disabled={!canDeployAll}
              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : deployableCount === 0 ? '没有可部署的启用配置' : undefined}
            >
              {deployingAll ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Send className="mr-1.5 h-3.5 w-3.5" />
              )}
              一键部署
            </Button>
            <Button size="sm" onClick={onAdd} disabled={targets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加配置
            </Button>
          </div>
          {targets.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              先添加 SSH 机器，再配置部署路径
            </p>
          ) : loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            </div>
          ) : configs.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有部署配置，点击「添加配置」开始
            </p>
          ) : (
            configs.map((cfg) => {
              const t = targetByID(targets, cfg.target_id)
              const deploying = busy === `deploy-ssh-config-${cfg.id}`
              const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
              return (
                <Card key={cfg.id} className="px-4 py-3">
                  <div className="flex items-start gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">
                          {configTitle(cfg)}
                        </span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            cfg.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {cfg.enabled ? '启用' : '停用'}
                        </span>
                        {cfg.auto_deploy && (
                          <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                            自动部署
                          </span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                        {targetSummary(t)}
                      </div>
                      <div
                        className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                        title={configPrimaryPath(cfg)}
                      >
                        {configPrimaryPath(cfg)}
                      </div>
                    </div>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onDeploy(cfg)}
                      disabled={!canDeploy}
                      title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                    >
                      {deploying ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Send className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => onEdit(cfg)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="hover:text-destructive"
                      onClick={() => onDelete(cfg)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              )
            })
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface SSHDeployConfigEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  config: SSHDeployConfig | null
  targets: SSHTarget[]
  onSaved: () => void
}

function SSHDeployConfigEditDialog({
  open,
  onOpenChange,
  domain,
  config,
  targets,
  onSaved,
}: SSHDeployConfigEditProps) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [fullchainPath, setFullchainPath] = useState('')
  const [keyPath, setKeyPath] = useState('')
  const [certPath, setCertPath] = useState('')
  const [chainPath, setChainPath] = useState('')
  const [deployCommand, setDeployCommand] = useState('')
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setFullchainPath(config?.fullchain_path ?? '/etc/nginx/ssl/{domain}/fullchain.pem')
    setKeyPath(config?.key_path ?? '/etc/nginx/ssl/{domain}/key.pem')
    setCertPath(config?.cert_path ?? '')
    setChainPath(config?.chain_path ?? '')
    setDeployCommand(config?.deploy_command ?? 'nginx -t && systemctl reload nginx')
    setAutoDeploy(config?.auto_deploy ?? false)
    setEnabled(config?.enabled ?? true)
  }, [open, config, targets])

  const save = async () => {
    if (!domain) return
    const form = {
      id: config?.id ?? 0,
      domain_id: domain.id,
      target_id: targetID,
      name: name.trim(),
      fullchain_path: fullchainPath.trim(),
      key_path: keyPath.trim(),
      cert_path: certPath.trim(),
      chain_path: chainPath.trim(),
      deploy_command: deployCommand.trim(),
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择 SSH 机器')
      return
    }
    if (!form.key_path) {
      toast.error('远端 key.pem 路径必填')
      return
    }
    if (!form.fullchain_path && !form.cert_path) {
      toast.error('fullchain.pem 路径和 cert.pem 路径至少填写一个')
      return
    }
    const payload = sshConfigToDeployConfig(form)
    setSaving(true)
    try {
      if (config?.id) {
        await api.put(`/acme/deploy/configs/${config.id}`, payload)
      } else {
        await api.post(`/acme/domains/${domain.id}/deploy-configs`, payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const selectableTargets = targets.filter((t) => t.enabled || t.id === targetID)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{config ? '编辑部署配置' : '新增部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 的证书部署路径和部署命令，支持 {'{domain}'} 占位符
          </DialogDescription>
        </DialogHeader>
        <div className="grid max-h-[70vh] gap-3.5 overflow-auto pr-1">
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-name">配置名称</Label>
            <Input
              id="deploy-config-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="nginx 主站"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-target">SSH 机器</Label>
            <select
              id="deploy-config-target"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={targetID ? String(targetID) : ''}
              onChange={(e) => setTargetID(Number(e.target.value))}
            >
              {selectableTargets.length === 0 && (
                <option value="">（暂无启用的 SSH 机器）</option>
              )}
              {selectableTargets.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.username}@{t.host}:{t.port || 22})
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-2">
            <Label>远端路径</Label>
            <Input
              value={fullchainPath}
              onChange={(e) => setFullchainPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/fullchain.pem"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={keyPath}
              onChange={(e) => setKeyPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/key.pem（必填）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={certPath}
              onChange={(e) => setCertPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/cert.pem（可选）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={chainPath}
              onChange={(e) => setChainPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/chain.pem（可选）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-command">部署命令（可选）</Label>
            <Textarea
              id="deploy-config-command"
              value={deployCommand}
              onChange={(e) => setDeployCommand(e.target.value)}
              placeholder="nginx -t && systemctl reload nginx"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="deploy-config-auto">签发/续期成功后自动部署</Label>
            <Switch id="deploy-config-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="deploy-config-enabled">启用</Label>
            <Switch id="deploy-config-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface SafelineDeployConfigsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  configs: SafelineDeployConfig[]
  targets: SafelineTarget[]
  loading: boolean
  busy: string | null
  onAdd: () => void
  onEdit: (cfg: SafelineDeployConfig) => void
  onDelete: (cfg: SafelineDeployConfig) => void
  onDeploy: (cfg: SafelineDeployConfig) => void
  onDeployAll: () => void
}

function safelineTargetByID(targets: SafelineTarget[], id: number) {
  return targets.find((t) => t.id === id)
}

function safelineTargetSummary(t?: SafelineTarget) {
  if (!t) return '雷池实例不存在'
  return `${t.name} · ${t.base_url}`
}

function safelineConfigTitle(cfg: SafelineDeployConfig) {
  return cfg.name?.trim() || `配置 #${cfg.id}`
}

export function SafelineDeployConfigsDrawer({
  open,
  onOpenChange,
  domain,
  configs,
  targets,
  loading,
  busy,
  onAdd,
  onEdit,
  onDelete,
  onDeploy,
  onDeployAll,
}: SafelineDeployConfigsDrawerProps) {
  const revoked = domain?.cert_status === 'revoked'
  const hasCert = Boolean(domain?.not_after)
  const deployableCount = configs.filter((cfg) => {
    const t = safelineTargetByID(targets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const deployingAll = Boolean(domain && busy === `deploy-safeline-domain-${domain.id}`)
  const canDeployAll = hasCert && !revoked && deployableCount > 0 && busy === null
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>雷池部署配置</DrawerTitle>
          <DrawerDescription>
            {domain?.main_domain ?? '当前域名'} 上传到雷池 WAF 证书管理的配置
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={onDeployAll}
              disabled={!canDeployAll}
              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : deployableCount === 0 ? '没有可部署的启用配置' : undefined}
            >
              {deployingAll ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <ShieldCheck className="mr-1.5 h-3.5 w-3.5" />
              )}
              一键部署
            </Button>
            <Button size="sm" onClick={onAdd} disabled={targets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加配置
            </Button>
          </div>
          {targets.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              先添加雷池实例，再配置证书上传
            </p>
          ) : loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            </div>
          ) : configs.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有雷池部署配置，点击「添加配置」开始
            </p>
          ) : (
            configs.map((cfg) => {
              const t = safelineTargetByID(targets, cfg.target_id)
              const deploying = busy === `deploy-safeline-config-${cfg.id}`
              const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
              return (
                <Card key={cfg.id} className="px-4 py-3">
                  <div className="flex items-start gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">
                          {safelineConfigTitle(cfg)}
                        </span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            cfg.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {cfg.enabled ? '启用' : '停用'}
                        </span>
                        {cfg.auto_deploy && (
                          <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                            自动部署
                          </span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                        {safelineTargetSummary(t)}
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                        cert_id={cfg.cert_id || '新增'} · type={cfg.cert_type || 2}
                      </div>
                    </div>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onDeploy(cfg)}
                      disabled={!canDeploy}
                      title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                    >
                      {deploying ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <ShieldCheck className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => onEdit(cfg)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="hover:text-destructive"
                      onClick={() => onDelete(cfg)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              )
            })
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface SafelineDeployConfigEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  config: SafelineDeployConfig | null
  targets: SafelineTarget[]
  onSaved: () => void
}

function SafelineDeployConfigEditDialog({
  open,
  onOpenChange,
  domain,
  config,
  targets,
  onSaved,
}: SafelineDeployConfigEditProps) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [certID, setCertID] = useState('')
  const [certType, setCertType] = useState('2')
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setCertID(config?.cert_id ? String(config.cert_id) : '')
    setCertType(String(config?.cert_type || 2))
    setAutoDeploy(config?.auto_deploy ?? false)
    setEnabled(config?.enabled ?? true)
  }, [open, config, targets])

  const save = async () => {
    if (!domain) return
    const certIDNum = certID.trim() ? Number(certID) : 0
    const certTypeNum = Number(certType) || 2
    const form = {
      id: config?.id ?? 0,
      domain_id: domain.id,
      target_id: targetID,
      name: name.trim(),
      cert_id: certIDNum,
      cert_type: certTypeNum,
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择雷池实例')
      return
    }
    if (!Number.isInteger(certIDNum) || certIDNum < 0) {
      toast.error('雷池 cert_id 无效')
      return
    }
    if (!Number.isInteger(certTypeNum) || certTypeNum <= 0) {
      toast.error('雷池证书类型无效')
      return
    }
    const payload = safelineConfigToDeployConfig(form)
    setSaving(true)
    try {
      if (config?.id) {
        await api.put(`/acme/deploy/configs/${config.id}`, payload)
      } else {
        await api.post(`/acme/domains/${domain.id}/deploy-configs`, payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const selectableTargets = targets.filter((t) => t.enabled || t.id === targetID)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{config ? '编辑雷池部署配置' : '新增雷池部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 上传到雷池证书管理；cert_id 留空表示首次新增，部署成功后会自动写回
          </DialogDescription>
        </DialogHeader>
        <div className="grid max-h-[70vh] gap-3.5 overflow-auto pr-1">
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-deploy-name">配置名称</Label>
            <Input
              id="safeline-deploy-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="雷池证书"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-deploy-target">雷池实例</Label>
            <select
              id="safeline-deploy-target"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={targetID ? String(targetID) : ''}
              onChange={(e) => setTargetID(Number(e.target.value))}
            >
              {selectableTargets.length === 0 && (
                <option value="">（暂无启用的雷池实例）</option>
              )}
              {selectableTargets.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.base_url})
                </option>
              ))}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="grid gap-1.5">
              <Label htmlFor="safeline-cert-id">雷池 cert_id</Label>
              <Input
                id="safeline-cert-id"
                value={certID}
                onChange={(e) => setCertID(e.target.value)}
                placeholder="留空则新增"
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="safeline-cert-type">证书类型</Label>
              <select
                id="safeline-cert-type"
                value={certType}
                onChange={(e) => setCertType(e.target.value)}
                className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              >
                <option value="2">手动上传证书（2）</option>
                <option value="1">类型 1（兼容）</option>
              </select>
            </div>
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-deploy-auto">签发/续期成功后自动部署</Label>
            <Switch id="safeline-deploy-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-deploy-enabled">启用</Label>
            <Switch id="safeline-deploy-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface SSHTargetsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  targets: SSHTarget[]
  onAdd: () => void
  onEdit: (t: SSHTarget) => void
  onDelete: (t: SSHTarget) => void
}

function authLabel(authType: string) {
  return authType === 'key' ? '证书' : '密码'
}

export function SSHTargetsDrawer({
  open,
  onOpenChange,
  targets,
  onAdd,
  onEdit,
  onDelete,
}: SSHTargetsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>SSH 机器</DrawerTitle>
          <DrawerDescription>
            配置远程证书部署目标；支持密码认证和私钥证书认证
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加机器
            </Button>
          </div>
          {targets.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 SSH 机器，点击「添加机器」开始
            </p>
          ) : (
            targets.map((t) => (
              <Card key={t.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">
                        {t.name}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          t.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {t.enabled ? '启用' : '停用'}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                      {t.username}@{t.host}:{t.port || 22} · {authLabel(t.auth_type)}
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onEdit(t)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(t)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface SSHTargetEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SSHTarget | null
  onSaved: () => void
}

function SSHTargetEditDialog({ open, onOpenChange, target, onSaved }: SSHTargetEditProps) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('22')
  const [username, setUsername] = useState('')
  const [authType, setAuthType] = useState<'password' | 'key'>('password')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setHost(target?.host ?? '')
    setPort(String(target?.port || 22))
    setUsername(target?.username ?? '')
    setAuthType(target?.auth_type === 'key' ? 'key' : 'password')
    setPassword(target?.password ?? '')
    setPrivateKey(target?.private_key ?? '')
    setPassphrase(target?.passphrase ?? '')
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const portNum = Number(port)
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      host: host.trim(),
      port: portNum,
      username: username.trim(),
      auth_type: authType,
      password: password.trim(),
      private_key: privateKey.trim(),
      passphrase: passphrase.trim(),
      enabled,
      created_at: target?.created_at ?? '',
      updated_at: target?.updated_at ?? '',
    }
    if (!form.name) {
      toast.error('目标名称必填')
      return
    }
    if (!form.host) {
      toast.error('SSH 主机必填')
      return
    }
    if (!form.username) {
      toast.error('SSH 用户名必填')
      return
    }
    if (!Number.isInteger(portNum) || portNum <= 0 || portNum > 65535) {
      toast.error('SSH 端口无效')
      return
    }
    if (authType === 'password' && !form.password) {
      toast.error('密码认证需要填写密码')
      return
    }
    if (authType === 'key' && !form.private_key) {
      toast.error('证书模式需要填写私钥')
      return
    }
    const payload = sshTargetToDeployTarget(form)
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/deploy/targets/${target.id}`, payload)
      } else {
        await api.post('/acme/deploy/targets', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{target ? '编辑 SSH 机器' : '新增 SSH 机器'}</DialogTitle>
          <DialogDescription>
            只保存机器连接和认证信息；证书路径和部署命令在每次部署时填写
          </DialogDescription>
        </DialogHeader>
        <div className="grid max-h-[70vh] gap-3.5 overflow-auto pr-1">
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-name">目标名称</Label>
            <Input
              id="ssh-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2 grid gap-1.5">
              <Label htmlFor="ssh-host">主机</Label>
              <Input
                id="ssh-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="example.com / 10.0.0.1"
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="ssh-port">端口</Label>
              <Input
                id="ssh-port"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-user">用户名</Label>
            <Input
              id="ssh-user"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="ssh-auth">认证方式</Label>
            <select
              id="ssh-auth"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={authType}
              onChange={(e) => setAuthType(e.target.value === 'key' ? 'key' : 'password')}
            >
              <option value="password">密码模式</option>
              <option value="key">证书模式（私钥）</option>
            </select>
          </div>
          {authType === 'password' ? (
            <div className="grid gap-1.5">
              <Label htmlFor="ssh-password">密码</Label>
              <Input
                id="ssh-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="new-password"
                data-lpignore="true"
                data-1p-ignore="true"
              />
            </div>
          ) : (
            <>
              <div className="grid gap-1.5">
                <Label htmlFor="ssh-private-key">私钥</Label>
                <Textarea
                  id="ssh-private-key"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                  autoComplete="off"
                  data-lpignore="true"
                  data-1p-ignore="true"
                  className="min-h-[140px] font-mono text-[11.5px]"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="ssh-passphrase">私钥口令（可选）</Label>
                <Input
                  id="ssh-passphrase"
                  type="password"
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  autoComplete="new-password"
                  data-lpignore="true"
                  data-1p-ignore="true"
                />
              </div>
            </>
          )}
          <div className="flex items-center justify-between">
            <Label htmlFor="ssh-enabled">启用</Label>
            <Switch id="ssh-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface SafelineTargetsDrawerProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  targets: SafelineTarget[]
  onAdd: () => void
  onEdit: (t: SafelineTarget) => void
  onDelete: (t: SafelineTarget) => void
  onTest: (t: SafelineTarget) => void
}

export function SafelineTargetsDrawer({
  open,
  onOpenChange,
  targets,
  onAdd,
  onEdit,
  onDelete,
  onTest,
}: SafelineTargetsDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>雷池 WAF</DrawerTitle>
          <DrawerDescription>
            配置雷池 Open API 实例；证书部署使用 X-SLCE-API-TOKEN
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加实例
            </Button>
          </div>
          {targets.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有雷池实例，点击「添加实例」开始
            </p>
          ) : (
            targets.map((t) => (
              <Card key={t.id} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">
                        {t.name}
                      </span>
                      <span
                        className={cn(
                          'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                          t.enabled
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {t.enabled ? '启用' : '停用'}
                      </span>
                      {t.skip_tls_verify && (
                        <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
                          跳过 TLS
                        </span>
                      )}
                    </div>
                    <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                      {t.base_url}
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onTest(t)} disabled={!t.enabled}>
                    <RefreshCw className="h-3.5 w-3.5" />
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => onEdit(t)}>
                    <Edit3 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="hover:text-destructive"
                    onClick={() => onDelete(t)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

interface SafelineTargetEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: SafelineTarget | null
  onSaved: () => void
}

function SafelineTargetEditDialog({ open, onOpenChange, target, onSaved }: SafelineTargetEditProps) {
  const [name, setName] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [apiToken, setAPIToken] = useState('')
  const [skipTLSVerify, setSkipTLSVerify] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setBaseURL(target?.base_url ?? '')
    setAPIToken(target?.api_token ?? '')
    setSkipTLSVerify(target?.skip_tls_verify ?? false)
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const form = {
      id: target?.id ?? 0,
      name: name.trim(),
      base_url: baseURL.trim().replace(/\/+$/, ''),
      api_token: apiToken.trim(),
      skip_tls_verify: skipTLSVerify,
      enabled,
      created_at: target?.created_at ?? '',
      updated_at: target?.updated_at ?? '',
    }
    if (!form.name) {
      toast.error('实例名称必填')
      return
    }
    if (!form.base_url) {
      toast.error('雷池地址必填')
      return
    }
    if (!form.api_token) {
      toast.error('API Token 必填')
      return
    }
    const payload = safelineTargetToDeployTarget(form)
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/deploy/targets/${target.id}`, payload)
      } else {
        await api.post('/acme/deploy/targets', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{target ? '编辑雷池实例' : '新增雷池实例'}</DialogTitle>
          <DialogDescription>
            地址填写管理端根地址，例如 https://waf.example.com:9443
          </DialogDescription>
        </DialogHeader>
        <div className="grid max-h-[70vh] gap-3.5 overflow-auto pr-1">
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-name">实例名称</Label>
            <Input
              id="safeline-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-base-url">雷池地址</Label>
            <Input
              id="safeline-base-url"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder="https://waf.example.com:9443"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-token">API Token</Label>
            <Input
              id="safeline-token"
              type="password"
              value={apiToken}
              onChange={(e) => setAPIToken(e.target.value)}
              autoComplete="new-password"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-skip-tls">跳过 TLS 校验</Label>
            <Switch id="safeline-skip-tls" checked={skipTLSVerify} onChange={(v) => setSkipTLSVerify(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-enabled">启用</Label>
            <Switch id="safeline-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface AccountEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: AcmeAccount | null
  onSaved: () => void
}

function AccountEditDialog({ open, onOpenChange, target, onSaved }: AccountEditProps) {
  const [name, setName] = useState('')
  const [ca, setCA] = useState('letsencrypt')
  const [directoryURL, setDirectoryURL] = useState('')
  const [email, setEmail] = useState('')
  const [eabKID, setEABKID] = useState('')
  const [eabHMAC, setEABHMAC] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setCA(target?.ca ?? 'letsencrypt')
    setDirectoryURL(target?.directory_url ?? '')
    setEmail(target?.email ?? '')
    setEABKID(target?.eab_kid ?? '')
    setEABHMAC(target?.eab_hmac ?? '')
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const payload = {
      name: name.trim(),
      ca,
      directory_url: directoryURL.trim(),
      email: email.trim(),
      eab_kid: eabKID.trim(),
      eab_hmac: eabHMAC.trim(),
      enabled,
    }
    if (!payload.name) {
      toast.error('账号名称必填')
      return
    }
    if (!payload.email) {
      toast.error('邮箱必填')
      return
    }
    if (ca === 'custom' && !payload.directory_url) {
      toast.error('自定义 CA 需要 directory URL')
      return
    }
    if (ca === 'zerossl' && (!payload.eab_kid || !payload.eab_hmac)) {
      toast.error('ZeroSSL 需要 EAB KID 与 EAB HMAC')
      return
    }
    if ((payload.eab_kid && !payload.eab_hmac) || (!payload.eab_kid && payload.eab_hmac)) {
      toast.error('EAB KID 与 EAB HMAC 需要同时填写')
      return
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/acme/accounts/${target.id}`, payload)
      } else {
        await api.post('/acme/accounts', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{target ? '编辑 CA 账号' : '新增 CA 账号'}</DialogTitle>
          <DialogDescription>
            账号配置保存到数据库；签发时按域名选择的账号注册或复用本地账号私钥
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="account-name">账号名称</Label>
            <Input
              id="account-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如 zerossl-main / letsencrypt"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="account-ca">CA</Label>
            <select
              id="account-ca"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={ca}
              onChange={(e) => setCA(e.target.value)}
            >
              <option value="letsencrypt">Let's Encrypt</option>
              <option value="zerossl">ZeroSSL</option>
              <option value="custom">自定义 ACME directory</option>
            </select>
          </div>
          {ca === 'custom' && (
            <div className="grid gap-1.5">
              <Label htmlFor="account-dir">Directory URL</Label>
              <Input
                id="account-dir"
                value={directoryURL}
                onChange={(e) => setDirectoryURL(e.target.value)}
                placeholder="https://acme.example.com/directory"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="account-email">邮箱</Label>
            <Input
              id="account-email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@example.com"
              className="font-mono text-[12px]"
            />
          </div>
          {(ca === 'zerossl' || ca === 'custom') && (
            <>
              <div className="grid gap-1.5">
                <Label htmlFor="account-eab-kid">
                  EAB KID{ca === 'zerossl' ? '' : '（可选）'}
                </Label>
                <Input
                  id="account-eab-kid"
                  value={eabKID}
                  onChange={(e) => setEABKID(e.target.value)}
                  className="font-mono text-[12px]"
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="account-eab-hmac">
                  EAB HMAC{ca === 'zerossl' ? '' : '（可选）'}
                </Label>
                <Input
                  id="account-eab-hmac"
                  value={eabHMAC}
                  onChange={(e) => setEABHMAC(e.target.value)}
                  className="font-mono text-[12px]"
                />
              </div>
            </>
          )}
          <div className="flex items-center justify-between">
            <Label htmlFor="account-enabled">启用</Label>
            <Switch
              id="account-enabled"
              checked={enabled}
              onChange={(v) => setEnabled(v)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function safeParseEnvs(s: string): Record<string, string> {
  try {
    const v = JSON.parse(s || '{}')
    return typeof v === 'object' && v !== null ? v : {}
  } catch {
    return {}
  }
}

interface CredentialEditProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Credential | null
  onSaved: () => void
}

interface EnvPair {
  key: string
  value: string
}

interface ProviderSchema {
  key: string
  label: string
  required: string[]
  optional?: string[]
}

// 常用 DNS provider + 对应 lego 环境变量；完整列表见 lego 文档
const PROVIDER_SCHEMAS: ProviderSchema[] = [
  {
    key: 'alidns',
    label: '阿里云 DNS (alidns)',
    required: ['ALICLOUD_ACCESS_KEY', 'ALICLOUD_SECRET_KEY'],
    optional: ['ALICLOUD_REGION_ID', 'ALICLOUD_SECURITY_TOKEN'],
  },
  {
    key: 'tencentcloud',
    label: '腾讯云 DNS (tencentcloud)',
    required: ['TENCENTCLOUD_SECRET_ID', 'TENCENTCLOUD_SECRET_KEY'],
    optional: ['TENCENTCLOUD_REGION'],
  },
  {
    key: 'dnspod',
    label: 'DNSPod 旧版 (dnspod)',
    required: ['DNSPOD_API_KEY'],
  },
  {
    key: 'huaweicloud',
    label: '华为云 DNS (huaweicloud)',
    required: ['HUAWEICLOUD_ACCESS_KEY_ID', 'HUAWEICLOUD_SECRET_ACCESS_KEY', 'HUAWEICLOUD_REGION'],
  },
  {
    key: 'cloudflare',
    label: 'Cloudflare (cloudflare)',
    required: ['CLOUDFLARE_DNS_API_TOKEN'],
    optional: ['CLOUDFLARE_ZONE_API_TOKEN'],
  },
  {
    key: 'godaddy',
    label: 'GoDaddy (godaddy)',
    required: ['GODADDY_API_KEY', 'GODADDY_API_SECRET'],
  },
  {
    key: 'gcore',
    label: 'Gcore (gcore)',
    required: ['GCORE_PERMANENT_API_TOKEN'],
  },
  {
    key: 'digitalocean',
    label: 'DigitalOcean (digitalocean)',
    required: ['DO_AUTH_TOKEN'],
  },
  {
    key: 'namecheap',
    label: 'Namecheap (namecheap)',
    required: ['NAMECHEAP_API_USER', 'NAMECHEAP_API_KEY'],
  },
  {
    key: 'gandiv5',
    label: 'Gandi v5 (gandiv5)',
    required: ['GANDIV5_PERSONAL_ACCESS_TOKEN'],
  },
  {
    key: 'route53',
    label: 'AWS Route 53 (route53)',
    required: ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_REGION'],
  },
]

function getProviderSchema(key: string): ProviderSchema | undefined {
  return PROVIDER_SCHEMAS.find((p) => p.key === key)
}

function CredentialEditDialog({ open, onOpenChange, target, onSaved }: CredentialEditProps) {
  const [provider, setProvider] = useState('')
  const [providerMode, setProviderMode] = useState<'preset' | 'custom'>('preset')
  const [pairs, setPairs] = useState<EnvPair[]>([{ key: '', value: '' }])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) {
      const p = target?.provider ?? PROVIDER_SCHEMAS[0].key
      const schema = getProviderSchema(p)
      setProvider(p)
      setProviderMode(target ? (schema ? 'preset' : 'custom') : 'preset')
      if (target) {
        const obj = safeParseEnvs(target.envs_json ?? '{}')
        const arr = Object.entries(obj).map(([key, value]) => ({ key, value: String(value) }))
        setPairs(arr.length ? arr : [{ key: '', value: '' }])
      } else {
        setPairs((schema?.required ?? []).map((k) => ({ key: k, value: '' })) || [{ key: '', value: '' }])
      }
    }
  }, [open, target])

  const schema = getProviderSchema(provider)
  const unusedOptional = schema?.optional?.filter((k) => !pairs.some((p) => p.key === k)) ?? []

  const updatePair = (i: number, patch: Partial<EnvPair>) => {
    setPairs((prev) => prev.map((p, idx) => (idx === i ? { ...p, ...patch } : p)))
  }
  const addPair = (key = '') => setPairs((prev) => [...prev, { key, value: '' }])
  const removePair = (i: number) =>
    setPairs((prev) => (prev.length === 1 ? [{ key: '', value: '' }] : prev.filter((_, idx) => idx !== i)))

  const onPresetChange = (key: string) => {
    if (key === '__custom__') {
      setProviderMode('custom')
      setProvider('')
      setPairs([{ key: '', value: '' }])
      return
    }
    setProviderMode('preset')
    setProvider(key)
    const next = getProviderSchema(key)
    setPairs((next?.required ?? []).map((k) => ({ key: k, value: '' })) || [{ key: '', value: '' }])
  }

  const save = async () => {
    const p = provider.trim()
    if (!p) {
      toast.error('provider 必填')
      return
    }
    const obj: Record<string, string> = {}
    const seen = new Set<string>()
    for (const pair of pairs) {
      const k = pair.key.trim()
      if (!k) continue
      if (seen.has(k)) {
        toast.error(`重复的 key：${k}`)
        return
      }
      seen.add(k)
      obj[k] = pair.value
    }
    setSaving(true)
    try {
      const { data } = await api.post('/acme/credentials', {
        provider: p,
        envs_json: JSON.stringify(obj),
      })
      if (data?.warning) {
        toast.warning(data.warning)
      } else {
        toast.success('已保存（凭证有效）')
      }
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{target ? '编辑凭证' : '新增凭证'}</DialogTitle>
          <DialogDescription>
            选择 DNS provider 后会自动列出所需环境变量，填入对应取值即可；
            其他未列出的 provider 可选「自定义」手动填 key
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="cred-provider">Provider</Label>
            {target ? (
              <Input id="cred-provider" value={provider} disabled />
            ) : (
              <select
                id="cred-provider"
                className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
                value={providerMode === 'custom' ? '__custom__' : provider}
                onChange={(e) => onPresetChange(e.target.value)}
              >
                {PROVIDER_SCHEMAS.map((p) => (
                  <option key={p.key} value={p.key}>
                    {p.label}
                  </option>
                ))}
                <option value="__custom__">自定义（手动填 provider key）</option>
              </select>
            )}
            {providerMode === 'custom' && !target && (
              <Input
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                placeholder="lego provider key，如 azure / hetzner"
                className="font-mono text-[12px]"
              />
            )}
          </div>
          <div className="grid gap-1.5">
            <Label>环境变量</Label>
            <div className="space-y-2">
              {pairs.map((pair, i) => {
                const isRequired = schema?.required.includes(pair.key)
                const isFixedKey = isRequired || (schema?.optional?.includes(pair.key) ?? false)
                if (isFixedKey) {
                  return (
                    <div key={i} className="grid gap-1">
                      <div className="flex items-center justify-between">
                        <span
                          className="font-mono text-[11.5px] text-muted-foreground"
                          title={pair.key}
                        >
                          {pair.key}
                          {isRequired && <span className="ml-1 text-rose-500">*</span>}
                        </span>
                        {!isRequired && (
                          <button
                            type="button"
                            onClick={() => removePair(i)}
                            className="text-[11px] text-muted-foreground hover:text-destructive"
                          >
                            移除
                          </button>
                        )}
                      </div>
                      <Input
                        value={pair.value}
                        onChange={(e) => updatePair(i, { value: e.target.value })}
                        placeholder={isRequired ? '必填' : 'value（可选）'}
                        className="font-mono text-[12px]"
                      />
                    </div>
                  )
                }
                return (
                  <div key={i} className="flex gap-2">
                    <Input
                      value={pair.key}
                      onChange={(e) => updatePair(i, { key: e.target.value })}
                      placeholder="KEY"
                      className="flex-1 font-mono text-[12px]"
                    />
                    <Input
                      value={pair.value}
                      onChange={(e) => updatePair(i, { value: e.target.value })}
                      placeholder="value"
                      className="flex-1 font-mono text-[12px]"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      className="shrink-0 hover:text-destructive"
                      onClick={() => removePair(i)}
                      disabled={pairs.length === 1 && !pair.key && !pair.value}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                )
              })}
              {unusedOptional.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <span className="text-[11.5px] text-muted-foreground">可选：</span>
                  {unusedOptional.map((k) => (
                    <button
                      key={k}
                      type="button"
                      onClick={() => addPair(k)}
                      className="rounded-md border border-dashed border-input px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground hover:bg-muted"
                    >
                      + {k}
                    </button>
                  ))}
                </div>
              )}
              <Button size="sm" variant="outline" onClick={() => addPair()} className="w-full">
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加自定义变量
              </Button>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

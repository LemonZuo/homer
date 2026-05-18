import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
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
  Task,
} from './types'
import {
  TASK_PAGE_SIZE_KEY,
  caLabel,
  readTaskPageSize,
  splitDeployConfigs,
  splitDeployTargets,
} from './utils'

export function useAcmeData() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [accounts, setAccounts] = useState<AcmeAccount[]>([])
  const [sshTargets, setSSHTargets] = useState<SSHTarget[]>([])
  const [safelineTargets, setSafelineTargets] = useState<SafelineTarget[]>([])
  const [casTargets, setCASTargets] = useState<CASTarget[]>([])
  const [fnosTargets, setFnOSTargets] = useState<FnOSTarget[]>([])
  const [sshCredentials, setSSHCredentials] = useState<SSHCredential[]>([])
  const [providers, setProviders] = useState<string[]>([])
  const [credentials, setCredentials] = useState<Credential[]>([])

  const [tasks, setTasks] = useState<Task[]>([])
  const [taskPage, setTaskPage] = useState(1)
  const [taskTotal, setTaskTotal] = useState(0)
  const [taskPageSize, setTaskPageSize] = useState(readTaskPageSize)
  const [taskStatus, setTaskStatus] = useState('')
  const taskPageRef = useRef(1)
  const taskPageSizeRef = useRef(readTaskPageSize())
  const taskStatusRef = useRef('')

  const [loading, setLoading] = useState(true)

  const [deployConfigs, setDeployConfigs] = useState<SSHDeployConfig[]>([])
  const [deployConfigLoading, setDeployConfigLoading] = useState(false)
  const [safeDeployConfigs, setSafeDeployConfigs] = useState<SafelineDeployConfig[]>([])
  const [safeDeployLoading, setSafeDeployLoading] = useState(false)
  const [casDeployConfigs, setCASDeployConfigs] = useState<CASDeployConfig[]>([])
  const [casDeployLoading, setCASDeployLoading] = useState(false)
  const [fnosDeployConfigs, setFnOSDeployConfigs] = useState<FnOSDeployConfig[]>([])
  const [fnosDeployLoading, setFnOSDeployLoading] = useState(false)

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
        api.get(`/acme/tasks?page=${taskPageRef.current}&page_size=${taskPageSizeRef.current}${taskStatusRef.current ? `&status=${taskStatusRef.current}` : ''}`),
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
      setCASTargets(groupedTargets.cas)
      setFnOSTargets(groupedTargets.fnos)
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
      setCASTargets(groupedTargets.cas)
      setFnOSTargets(groupedTargets.fnos)
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

  const reloadDeployConfigs = useCallback(async (domainID: number) => {
    setDeployConfigLoading(true)
    setSafeDeployLoading(true)
    setCASDeployLoading(true)
    setFnOSDeployLoading(true)
    try {
      const { data } = await api.get(`/acme/domains/${domainID}/deploy-configs`)
      const groupedConfigs = splitDeployConfigs(data?.data ?? [])
      setDeployConfigs(groupedConfigs.ssh)
      setSafeDeployConfigs(groupedConfigs.safeline)
      setCASDeployConfigs(groupedConfigs.cas)
      setFnOSDeployConfigs(groupedConfigs.fnos)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载部署配置失败')
    } finally {
      setDeployConfigLoading(false)
      setSafeDeployLoading(false)
      setCASDeployLoading(false)
      setFnOSDeployLoading(false)
    }
  }, [])

  useEffect(() => {
    reloadAll()
  }, [reloadAll])

  const loadTasks = useCallback(async (page: number) => {
    try {
      const { data } = await api.get(
        `/acme/tasks?page=${page}&page_size=${taskPageSizeRef.current}${taskStatusRef.current ? `&status=${taskStatusRef.current}` : ''}`,
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

  const changeTaskStatus = useCallback(
    (status: string) => {
      setTaskStatus(status)
      taskStatusRef.current = status
      void loadTasks(1)
    },
    [loadTasks],
  )

  return {
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
  }
}

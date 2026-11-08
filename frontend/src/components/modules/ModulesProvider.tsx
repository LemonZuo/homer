import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import axios from 'axios'
import { toast } from 'sonner'
import { api } from '../../api'
import { CORE_PAGES, ModulesContext, type ModuleSetting, type ModuleSnapshot } from '../../lib/modules'

function errorMessage(error: unknown) {
  return axios.isAxiosError(error) ? error.response?.data?.error || error.message : '模块配置加载失败'
}

export function ModulesProvider({ children }: { children: ReactNode }) {
  const [snapshot, setSnapshot] = useState<ModuleSnapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [connected, setConnected] = useState(false)
  const [pending, setPending] = useState<string[]>([])
  const eventSequence = useRef(0)

  const apply = useCallback((next: ModuleSnapshot) => {
    if (!next || !Array.isArray(next.modules)) throw new Error('模块配置格式错误')
    // 同实例下 revision 相同即内容相同：保留旧对象，避免轮询/重复推送引发整树重渲染。
    setSnapshot((prev) => prev?.instance === next.instance && prev.revision >= next.revision ? prev : next)
    setError('')
    setLoading(false)
  }, [])

  const reload = useCallback(async () => {
    const sequence = eventSequence.current
    setLoading(true)
    try {
      const { data } = await api.get<{ data: ModuleSnapshot }>('/system/modules')
      if (sequence === eventSequence.current) apply(data.data)
    } catch (e) {
      if (sequence === eventSequence.current) setError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [apply])

  useEffect(() => {
    queueMicrotask(() => { void reload() })
    const stream = new EventSource('/api/system/modules/stream')
    stream.onopen = () => setConnected(true)
    stream.onerror = () => setConnected(false)
    stream.addEventListener('modules', (event) => {
      try {
        const next = JSON.parse((event as MessageEvent).data) as ModuleSnapshot
        apply(next)
        eventSequence.current++
        setConnected(true)
      } catch {
        setError('模块配置同步失败，请刷新重试')
      }
    })
    const refresh = () => { void reload() }
    const interceptor = api.interceptors.response.use(undefined, (failure: unknown) => {
      if (axios.isAxiosError(failure) && failure.response?.data?.code === 'MODULE_DISABLED') refresh()
      return Promise.reject(failure)
    })
    const visible = () => { if (document.visibilityState === 'visible') refresh() }
    // SSE 断线期间仍定期校正；回到标签页和接口拒绝时也刷新。
    const interval = window.setInterval(refresh, 30000)
    document.addEventListener('visibilitychange', visible)
    return () => {
      stream.close()
      window.clearInterval(interval)
      api.interceptors.response.eject(interceptor)
      document.removeEventListener('visibilitychange', visible)
    }
  }, [apply, reload])

  const update = useCallback(async (module: ModuleSetting, enabled: boolean) => {
    setPending((prev) => [...prev, module.key])
    try {
      const { data } = await api.put<{ data: ModuleSnapshot }>(`/system/modules/${module.key}`, { enabled, version: module.version })
      apply(data.data)
      toast.success(`${module.label}已${enabled ? '启用' : '禁用'}`)
    } catch (e) {
      toast.error(errorMessage(e))
      await reload()
    } finally {
      setPending((prev) => prev.filter((key) => key !== module.key))
    }
  }, [apply, reload])

  const isEnabled = useCallback((key: string) => CORE_PAGES.includes(key) ||
    snapshot?.modules.some((module) => module.key === key && module.enabled) === true, [snapshot])

  return <ModulesContext.Provider value={{ snapshot, loading, error, connected, pending, reload, update, isEnabled }}>{children}</ModulesContext.Provider>
}

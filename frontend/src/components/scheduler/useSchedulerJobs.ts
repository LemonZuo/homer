import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import axios from 'axios'
import { api } from '../../api'
import type { Job } from './types'
import { useModules } from '../../lib/modules'

export function useSchedulerJobs() {
  const { snapshot } = useModules()
  const moduleVersions = snapshot?.modules.map((module) => `${module.key}:${module.version}:${!module.enabled && module.active > 0}`).join(',')
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/scheduler/jobs')
      setJobs(data?.data ?? [])
    } catch (e) {
      toast.error(axios.isAxiosError(e) ? e.response?.data?.error || e.message : '加载任务失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => { void load() })
  }, [load, moduleVersions])

  const runJob = useCallback(
    async (name: string) => {
      try {
        await api.post(`/scheduler/jobs/${encodeURIComponent(name)}/run`)
        toast.success('已触发，稍后刷新查看结果')
        setTimeout(load, 1200)
      } catch (e) {
        toast.error(axios.isAxiosError(e) ? e.response?.data?.error || e.message : '触发失败')
      }
    },
    [load],
  )

  return {
    jobs,
    loading,
    load,
    runJob,
  }
}

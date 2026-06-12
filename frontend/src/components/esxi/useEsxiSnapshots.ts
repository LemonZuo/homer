import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'

import { api } from '../../api'
import { extractErr, isStaleSample, useNowTick } from './format'
import type { Snapshot } from './types'

function normalizeSnapshots(arr: unknown): Snapshot[] {
  return Array.isArray(arr) ? (arr as Snapshot[]) : []
}

export function useEsxiSnapshots() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const now = useNowTick()

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/esxi/snapshot')
      setSnapshots(normalizeSnapshots(data?.data))
    } catch (e) {
      toast.error(extractErr(e, '加载失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const triggerSample = useCallback(async () => {
    setRefreshing(true)
    try {
      const { data } = await api.post('/esxi/refresh')
      setSnapshots(normalizeSnapshots(data?.data))
      toast.success('已触发一次采样')
    } catch (e) {
      toast.error(extractErr(e, '采样失败'))
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    queueMicrotask(() => {
      void reload()
    })
  }, [reload])

  useEffect(() => {
    const es = new EventSource('/api/esxi/stream')
    es.addEventListener('snapshot', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data)
        setSnapshots(normalizeSnapshots(data))
        setLoading(false)
      } catch {
        // 单帧损坏忽略
      }
    })
    return () => es.close()
  }, [])

  const stats = useMemo(() => {
    const hostCnt = snapshots.length
    let onlineHosts = 0
    let totalVMs = 0
    let runningVMs = 0
    let cpuPeak = -1
    for (const s of snapshots) {
      if (s.reachable && !isStaleSample(s.sampled_at, now)) onlineHosts++
      if (s.vms) {
        totalVMs += s.vms.length
        for (const v of s.vms) {
          if (v.state === 'powered_on') runningVMs++
        }
      }
      if (s.cpu_temperature?.max_c != null && s.cpu_temperature.max_c > cpuPeak) {
        cpuPeak = s.cpu_temperature.max_c
      }
    }
    return { hostCnt, onlineHosts, totalVMs, runningVMs, cpuPeak }
  }, [now, snapshots])

  const lastSampled = useMemo(() => {
    let latest = ''
    for (const s of snapshots) {
      if (s.sampled_at && (!latest || s.sampled_at > latest)) latest = s.sampled_at
    }
    return latest
  }, [snapshots])

  return {
    snapshots,
    loading,
    refreshing,
    empty: !loading && snapshots.length === 0,
    stats,
    lastSampled,
    reload,
    triggerSample,
  }
}

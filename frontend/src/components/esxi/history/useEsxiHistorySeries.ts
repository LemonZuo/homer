import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'

import { api } from '../../../api'
import { extractErr } from '../format'
import type { SeriesPoint } from '../types'
import type { MetricKey } from './types'

export function useEsxiHistorySeries({
  hostKind,
  hostID,
}: {
  hostKind: string
  hostID: number
}) {
  const [range, setRange] = useState('24h')
  const [metric, setMetric] = useState<MetricKey>('cpu_cores')
  const [series, setSeries] = useState<SeriesPoint[] | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (r: string) => {
      setLoading(true)
      try {
        const { data } = await api.get('/esxi/series', {
          params: { host_kind: hostKind, host_id: hostID, range: r },
        })
        setSeries(data?.data ?? [])
      } catch (e) {
        toast.error(extractErr(e, '加载历史失败'))
      } finally {
        setLoading(false)
      }
    },
    [hostKind, hostID],
  )

  useEffect(() => {
    queueMicrotask(() => {
      void load(range)
    })
  }, [load, range])

  return {
    range,
    setRange,
    metric,
    setMetric,
    series,
    loading,
  }
}

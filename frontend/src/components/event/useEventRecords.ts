import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import type { EventForm, EventItem } from './types'

export function useEventRecords() {
  const [records, setRecords] = useState<EventItem[]>([])
  const [loading, setLoading] = useState(true)
  const [notifying, setNotifying] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/event')
      setRecords(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const save = useCallback(
    async (target: EventItem | null, form: EventForm) => {
      if (target) {
        await api.put(`/event/${target.id}`, form)
      } else {
        await api.post('/event', form)
      }
      await load()
    },
    [load],
  )

  const remove = useCallback(
    async (target: EventItem) => {
      try {
        await api.delete(`/event/${target.id}`)
        await load()
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '删除失败')
      }
    },
    [load],
  )

  const toggle = useCallback(async (record: EventItem, value: boolean) => {
    setRecords((prev) => prev.map((x) => (x.id === record.id ? { ...x, enabled: value } : x)))
    try {
      await api.put(`/event/${record.id}`, { ...record, enabled: value })
    } catch {
      setRecords((prev) => prev.map((x) => (x.id === record.id ? record : x)))
    }
  }, [])

  const notify = useCallback(async (record: EventItem) => {
    setNotifying(record.id)
    try {
      const { data } = await api.post(`/event/${record.id}/notify`)
      toast.success(data?.message || '已推送企业微信')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '执行失败')
    } finally {
      setNotifying(null)
    }
  }, [])

  return { records, loading, notifying, save, remove, toggle, notify }
}

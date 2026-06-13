import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import type { Birthday, BirthdayForm } from './types'

export function useBirthdayRecords() {
  const [records, setRecords] = useState<Birthday[]>([])
  const [loading, setLoading] = useState(true)
  const [notifying, setNotifying] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/birthday')
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
    async (target: Birthday | null, form: BirthdayForm) => {
      if (target) {
        await api.put(`/birthday/${target.id}`, form)
      } else {
        await api.post('/birthday', form)
      }
      await load()
    },
    [load],
  )

  const remove = useCallback(
    async (target: Birthday) => {
      try {
        await api.delete(`/birthday/${target.id}`)
        await load()
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '删除失败')
      }
    },
    [load],
  )

  const toggle = useCallback(async (record: Birthday, value: boolean) => {
    setRecords((prev) => prev.map((x) => (x.id === record.id ? { ...x, enabled: value } : x)))
    try {
      await api.put(`/birthday/${record.id}`, { ...record, enabled: value })
    } catch {
      setRecords((prev) => prev.map((x) => (x.id === record.id ? record : x)))
    }
  }, [])

  const notify = useCallback(async (record: Birthday) => {
    setNotifying(record.id)
    try {
      const { data } = await api.post(`/birthday/${record.id}/notify`)
      toast.success(data?.message || '已推送企业微信')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '执行失败')
    } finally {
      setNotifying(null)
    }
  }, [])

  return { records, loading, notifying, save, remove, toggle, notify }
}

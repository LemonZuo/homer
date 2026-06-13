import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import { LS_KEY, type Forwarder } from './types'

export function useSmsForwarders() {
  const [forwarders, setForwarders] = useState<Forwarder[]>([])
  const [selectedID, setSelectedID] = useState<number | null>(() => {
    const v = localStorage.getItem(LS_KEY)
    return v ? Number(v) : null
  })

  const enabledList = useMemo(() => forwarders.filter((f) => f.enabled), [forwarders])

  const loadForwarders = useCallback(async () => {
    try {
      const { data } = await api.get('/sms/forwarders')
      const rows: Forwarder[] = data?.data ?? []
      setForwarders(rows)
      setSelectedID((prev) => {
        const stillValid = prev != null && rows.some((r) => r.id === prev && r.enabled)
        if (stillValid) return prev
        const first = rows.find((r) => r.enabled)
        return first ? first.id : null
      })
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载转发器失败')
    }
  }, [])

  useEffect(() => {
    loadForwarders()
  }, [loadForwarders])

  useEffect(() => {
    if (selectedID != null) localStorage.setItem(LS_KEY, String(selectedID))
  }, [selectedID])

  return {
    forwarders,
    enabledList,
    selectedID,
    setSelectedID,
    loadForwarders,
  }
}

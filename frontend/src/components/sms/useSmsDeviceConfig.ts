import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'

export function useSmsDeviceConfig(selectedID: number | null) {
  const [config, setConfig] = useState<any>(null)
  const [configLoading, setConfigLoading] = useState(false)

  const fetchConfig = useCallback(async () => {
    if (selectedID == null) {
      toast.error('请先选择短信转发器')
      return
    }
    setConfigLoading(true)
    try {
      const { data } = await api.post('/sms/config/query', { target_id: selectedID })
      setConfig(data)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '配置查询失败')
    } finally {
      setConfigLoading(false)
    }
  }, [selectedID])

  useEffect(() => {
    if (selectedID == null) {
      setConfig(null)
      return
    }
    let cancelled = false
    setConfigLoading(true)
    api
      .post('/sms/config/query', { target_id: selectedID })
      .then(({ data }) => {
        if (!cancelled) setConfig(data)
      })
      .catch((e) => {
        if (!cancelled) toast.error(e?.response?.data?.error || e?.message || '配置查询失败')
      })
      .finally(() => {
        if (!cancelled) setConfigLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selectedID])

  return {
    config,
    configLoading,
    fetchConfig,
  }
}

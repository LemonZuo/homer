import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import type { Channel, ModuleBindings, ModuleMeta, TypeMeta } from './types'

export function useNotifyData() {
  const [modules, setModules] = useState<ModuleMeta[]>([])
  const [types, setTypes] = useState<TypeMeta[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [bindings, setBindings] = useState<ModuleBindings>({})
  const [testingID, setTestingID] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)

  const loadMeta = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/meta')
      setModules(data?.data?.modules ?? [])
      setTypes(data?.data?.types ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载元信息失败')
    }
  }, [])

  const loadChannels = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/channels')
      setChannels(data?.data ?? [])
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载通道失败')
    }
  }, [])

  const loadBindings = useCallback(async () => {
    try {
      const { data } = await api.get('/notify/bindings')
      setBindings(data?.data ?? {})
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载绑定失败')
    }
  }, [])

  const reloadAll = useCallback(async () => {
    setLoading(true)
    try {
      await Promise.all([loadMeta(), loadChannels(), loadBindings()])
    } finally {
      setLoading(false)
    }
  }, [loadMeta, loadChannels, loadBindings])

  useEffect(() => {
    reloadAll()
  }, [reloadAll])

  const deleteChannel = useCallback(
    async (channel: Channel) => {
      try {
        await api.delete(`/notify/channels/${channel.id}`)
        toast.success('已删除')
        loadChannels()
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '删除失败')
      }
    },
    [loadChannels],
  )

  const testChannel = useCallback(async (id: number) => {
    setTestingID(id)
    try {
      await api.post(`/notify/channels/${id}/test`)
      toast.success('已发送测试消息')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '测试失败')
    } finally {
      setTestingID(null)
    }
  }, [])

  const toggleBinding = useCallback(
    async (module: string, channelID: number) => {
      const cur = bindings[module] ?? []
      const next = cur.includes(channelID)
        ? cur.filter((x) => x !== channelID)
        : [...cur, channelID]
      try {
        await api.put(`/notify/bindings/${module}`, { channel_ids: next })
        setBindings((b) => ({ ...b, [module]: next }))
        loadChannels()
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '保存绑定失败')
      }
    },
    [bindings, loadChannels],
  )

  return {
    modules,
    types,
    channels,
    bindings,
    testingID,
    loading,
    loadChannels,
    reloadAll,
    deleteChannel,
    testChannel,
    toggleBinding,
  }
}

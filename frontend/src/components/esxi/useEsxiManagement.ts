import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { api } from '../../api'
import { extractErr } from './format'
import type { EsxiCredential, EsxiHost } from './types'

interface UseEsxiManagementArgs {
  reloadSnapshots: () => Promise<void> | void
}

export function useEsxiManagement({ reloadSnapshots }: UseEsxiManagementArgs) {
  const [hostsOpen, setHostsOpen] = useState(false)
  const [hostEditOpen, setHostEditOpen] = useState(false)
  const [editingHost, setEditingHost] = useState<EsxiHost | null>(null)
  const [credsOpen, setCredsOpen] = useState(false)
  const [credEditOpen, setCredEditOpen] = useState(false)
  const [editingCred, setEditingCred] = useState<EsxiCredential | null>(null)
  const [hosts, setHosts] = useState<EsxiHost[]>([])
  const [credentials, setCredentials] = useState<EsxiCredential[]>([])

  const loadHosts = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/hosts')
      setHosts(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载机器失败'))
    }
  }, [])

  const loadCredentials = useCallback(async () => {
    try {
      const { data } = await api.get('/esxi/credentials')
      setCredentials(data?.data ?? [])
    } catch (e) {
      toast.error(extractErr(e, '加载凭证失败'))
    }
  }, [])

  const openHostsDrawer = useCallback(() => {
    setHostsOpen(true)
    void loadHosts()
    void loadCredentials()
  }, [loadHosts, loadCredentials])

  const openCredsDrawer = useCallback(() => {
    setCredsOpen(true)
    void loadCredentials()
  }, [loadCredentials])

  const onAddHost = () => {
    setEditingHost(null)
    setHostEditOpen(true)
  }

  const onEditHost = (h: EsxiHost) => {
    setEditingHost(h)
    setHostEditOpen(true)
  }

  const onDeleteHost = async (h: EsxiHost) => {
    if (!window.confirm(`确认删除 ESXi 机器「${h.name}」?`)) return
    try {
      await api.delete(`/esxi/hosts/${h.id}`)
      toast.success('已删除')
      void loadHosts()
      void reloadSnapshots()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }

  const onTestHost = async (h: EsxiHost) => {
    try {
      const { data } = await api.post(`/esxi/hosts/${h.id}/test`)
      const r = data?.data
      if (r?.ok) {
        toast.success(r.summary ? `连通成功 · ${r.summary}` : '连通成功')
      } else {
        toast.error(r?.error || '连通失败')
      }
    } catch (e) {
      toast.error(extractErr(e, '测试失败'))
    }
  }

  const onAddCredential = () => {
    setEditingCred(null)
    setCredEditOpen(true)
  }

  const onEditCredential = (c: EsxiCredential) => {
    setEditingCred(c)
    setCredEditOpen(true)
  }

  const onDeleteCredential = async (c: EsxiCredential) => {
    if (!window.confirm(`确认删除 ESXi 凭证「${c.name}」?`)) return
    try {
      await api.delete(`/esxi/credentials/${c.id}`)
      toast.success('已删除')
      void loadCredentials()
    } catch (e) {
      toast.error(extractErr(e, '删除失败'))
    }
  }

  return {
    hostsOpen,
    setHostsOpen,
    hostEditOpen,
    setHostEditOpen,
    editingHost,
    credsOpen,
    setCredsOpen,
    credEditOpen,
    setCredEditOpen,
    editingCred,
    hosts,
    credentials,
    loadHosts,
    loadCredentials,
    openHostsDrawer,
    openCredsDrawer,
    onAddHost,
    onEditHost,
    onDeleteHost,
    onTestHost,
    onAddCredential,
    onEditCredential,
    onDeleteCredential,
  }
}

export type EsxiManagement = ReturnType<typeof useEsxiManagement>

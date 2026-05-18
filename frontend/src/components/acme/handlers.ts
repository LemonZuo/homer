import { toast } from 'sonner'
import { api } from '../../api'

export type DeployConfigKind = 'ssh' | 'safeline' | 'cas' | 'fnos'

// 把「取 pending → 清 pending → DELETE → toast → reload」这套完全一致的删除流程收敛为一个工厂。
export function makeDeleteHandler<T extends { id: number }>(opts: {
  get: () => T | null
  clear: () => void
  url: (t: T) => string
  reload: () => Promise<void> | void
}) {
  return async () => {
    const t = opts.get()
    if (!t) return
    opts.clear()
    try {
      await api.delete(opts.url(t))
      toast.success('已删除')
      await opts.reload()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }
}

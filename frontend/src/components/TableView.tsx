import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Plus, Search, Loader2, Inbox } from 'lucide-react'
import { motion } from 'motion/react'
import { api } from '../api'
import { getTable } from '../tables'
import { getColorSet } from '../colors'
import { cn } from '../lib/utils'
import RecordCard from './RecordCard'
import RecordForm from './RecordForm'
import Modal from './Modal'
import { Input } from './ui/input'
import { Button } from './ui/button'
import { Badge } from './ui/badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './ui/alert-dialog'

export default function TableView() {
  const { tableKey } = useParams<{ tableKey: string }>()
  const def = useMemo(() => (tableKey ? getTable(tableKey) : undefined), [tableKey])

  const [records, setRecords] = useState<Record<string, any>[]>([])
  const [loading, setLoading] = useState(false)
  const [kw, setKw] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Record<string, any> | null>(null)
  const [pendingDelete, setPendingDelete] = useState<Record<string, any> | null>(null)

  const load = async () => {
    if (!def) return
    setLoading(true)
    try {
      const { data } = await api.get(`/${def.path}`)
      setRecords(data.data ?? [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setRecords([])
    setKw('')
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tableKey])

  if (!def) return <div className="p-6 text-muted-foreground">未找到该表</div>
  const cs = getColorSet(def.color)

  const filtered = records.filter((r) => {
    if (!kw) return true
    const s = kw.toLowerCase()
    return Object.values(r).some((v) =>
      v !== null && v !== undefined && String(v).toLowerCase().includes(s)
    )
  })

  const onAdd = () => {
    setEditing(null)
    setModalOpen(true)
  }
  const onEdit = (r: Record<string, any>) => {
    setEditing(r)
    setModalOpen(true)
  }

  const onSubmit = async (data: Record<string, any>) => {
    if (editing && editing.id) {
      await api.put(`/${def.path}/${editing.id}`, data)
    } else {
      await api.post(`/${def.path}`, data)
    }
    setModalOpen(false)
    await load()
  }

  const confirmDelete = async () => {
    const r = pendingDelete
    if (!r) return
    setPendingDelete(null)
    await api.delete(`/${def.path}/${r.id}`)
    await load()
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:pt-10 sm:px-8">
      {/* 桌面端 header */}
      <div className="mb-8 hidden flex-wrap items-end justify-between gap-3 sm:flex">
        <div className="flex items-center gap-3">
          <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
          <h1 className="text-[28px] font-bold tracking-tight leading-none">{def.label}</h1>
          <Badge variant="muted" className="font-mono tabular-nums">{filtered.length}</Badge>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search
              size={14}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              value={kw}
              onChange={(e) => setKw(e.target.value)}
              placeholder="搜索"
              className="h-8 w-44 pl-7 text-[13px] transition-[width] focus-visible:w-60"
            />
          </div>
          <Button size="sm" onClick={onAdd}>
            <Plus className="h-3.5 w-3.5" />
            新增
          </Button>
        </div>
      </div>

      {/* 移动端：搜索框 */}
      <div className="mb-4 sm:hidden">
        <div className="relative">
          <Search
            size={14}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            value={kw}
            onChange={(e) => setKw(e.target.value)}
            placeholder={`搜索 ${filtered.length} 条记录`}
            className="h-10 pl-8"
          />
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center py-20 text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-16 text-center text-muted-foreground">
          <Inbox className="h-5 w-5 opacity-50" />
          <p className="text-[13px]">{kw ? '没有匹配的记录' : '暂无数据'}</p>
        </div>
      ) : (
        <motion.div
          className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
          initial="hidden"
          animate="visible"
          variants={{
            visible: { transition: { staggerChildren: 0.03 } },
            hidden: {},
          }}
        >
          {filtered.map((r) => (
            <motion.div
              key={r.id}
              className="h-full"
              variants={{
                hidden: { opacity: 0, y: 8 },
                visible: {
                  opacity: 1,
                  y: 0,
                  transition: { duration: 0.25, ease: [0.16, 1, 0.3, 1] },
                },
              }}
            >
              <RecordCard
                record={r}
                def={def}
                onEdit={() => onEdit(r)}
                onDelete={() => setPendingDelete(r)}
              />
            </motion.div>
          ))}
        </motion.div>
      )}

      {/* 移动端浮动新增按钮 */}
      <Button
        size="icon"
        onClick={onAdd}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增"
      >
        <Plus className="h-5 w-5" />
      </Button>

      <Modal
        open={modalOpen}
        title={editing ? `编辑 ${def.label}` : `新增 ${def.label}`}
        onClose={() => setModalOpen(false)}
      >
        <RecordForm
          fields={def.fields}
          initial={editing ?? {}}
          onSubmit={onSubmit}
          onCancel={() => setModalOpen(false)}
        />
      </Modal>

      <AlertDialog
        open={!!pendingDelete}
        onOpenChange={(o) => { if (!o) setPendingDelete(null) }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              该操作不可撤销，记录将被永久删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

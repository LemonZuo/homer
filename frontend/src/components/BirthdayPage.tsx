import { useCallback, useEffect, useMemo, useState } from 'react'
import { Plus, Search, Loader2, Inbox, Pencil, Trash2, Bell } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { avatarColor, getColorSet } from '../colors'
import { cn } from '../lib/utils'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Badge } from './ui/badge'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Switch } from './ui/switch'
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip'
import Modal from './Modal'
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

interface Birthday {
  id: number
  name: string
  birthday: string
  chinese_birthday: string
  zodiac: string
  enabled: boolean
}

const blank = { name: '', birthday: '', enabled: true }

export default function BirthdayPage() {
  const [records, setRecords] = useState<Birthday[]>([])
  const [loading, setLoading] = useState(true)
  const [kw, setKw] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Birthday | null>(null)
  const [form, setForm] = useState<Record<string, any>>(blank)
  const [saving, setSaving] = useState(false)
  const [formErr, setFormErr] = useState('')
  const [pendingDelete, setPendingDelete] = useState<Birthday | null>(null)
  const [notifying, setNotifying] = useState<number | null>(null)

  const cs = getColorSet('orange')

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

  const filtered = useMemo(() => {
    if (!kw) return records
    const s = kw.toLowerCase()
    return records.filter((r) =>
      Object.values(r).some(
        (v) => v !== null && v !== undefined && String(v).toLowerCase().includes(s),
      ),
    )
  }, [records, kw])

  const openAdd = () => {
    setEditing(null)
    setForm(blank)
    setFormErr('')
    setModalOpen(true)
  }
  const openEdit = (r: Birthday) => {
    setEditing(r)
    setForm({ name: r.name, birthday: r.birthday, enabled: r.enabled })
    setFormErr('')
    setModalOpen(true)
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setFormErr('')
    try {
      if (editing) {
        await api.put(`/birthday/${editing.id}`, form)
      } else {
        await api.post('/birthday', form)
      }
      setModalOpen(false)
      await load()
    } catch (e: any) {
      setFormErr(e?.response?.data?.error || e?.message || '提交失败')
    } finally {
      setSaving(false)
    }
  }

  const confirmDelete = async () => {
    const r = pendingDelete
    if (!r) return
    setPendingDelete(null)
    try {
      await api.delete(`/birthday/${r.id}`)
      await load()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }

  const onToggle = async (r: Birthday, value: boolean) => {
    setRecords((prev) => prev.map((x) => (x.id === r.id ? { ...x, enabled: value } : x)))
    try {
      await api.put(`/birthday/${r.id}`, { ...r, enabled: value })
    } catch {
      setRecords((prev) => prev.map((x) => (x.id === r.id ? r : x)))
    }
  }

  const runNotify = async (r: Birthday) => {
    setNotifying(r.id)
    try {
      const { data } = await api.post(`/birthday/${r.id}/notify`)
      toast.success(data?.message || '已推送企业微信')
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '执行失败')
    } finally {
      setNotifying(null)
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-8 hidden flex-wrap items-end justify-between gap-3 sm:flex">
        <div className="flex items-center gap-3">
          <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
          <h1 className="text-[28px] font-bold leading-none tracking-tight">生日提醒</h1>
          <Badge variant="muted" className="font-mono tabular-nums">
            {filtered.length}
          </Badge>
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
          <Button size="sm" onClick={openAdd}>
            <Plus className="h-3.5 w-3.5" />
            新增
          </Button>
        </div>
      </div>

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
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((r) => (
            <Card
              key={r.id}
              className={cn(
                'group relative flex h-full flex-col overflow-hidden transition-[transform,box-shadow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] will-change-transform hover:-translate-y-1',
                cs.border,
                cs.halo,
              )}
            >
              <div className="flex items-center gap-3 px-4 pt-4">
                <div
                  className={cn(
                    'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[13px] font-medium text-white shadow-sm',
                    avatarColor(r.name || '?'),
                  )}
                >
                  {(r.name || '?').charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[14px] font-semibold tracking-tight">
                    {r.name || '(无姓名)'}
                  </div>
                  <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
                    {[r.birthday, r.zodiac].filter(Boolean).join(' · ')}
                  </div>
                </div>
                <div className="flex shrink-0 gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        onClick={() => runNotify(r)}
                        disabled={notifying === r.id}
                        aria-label="发送提醒"
                      >
                        {notifying === r.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Bell className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>发送提醒</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        onClick={() => openEdit(r)}
                        aria-label="编辑"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>编辑</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 hover:text-destructive"
                        onClick={() => setPendingDelete(r)}
                        aria-label="删除"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>删除</TooltipContent>
                  </Tooltip>
                </div>
              </div>

              <div className="mt-3 space-y-0 px-4 pb-3">
                <Row label="公历生日" value={r.birthday} />
                <Row label="农历生日" value={r.chinese_birthday} />
                <Row label="生肖" value={r.zodiac} />
                <div className="flex items-center gap-3 py-1 text-[12.5px]">
                  <span className="w-16 shrink-0 text-muted-foreground">启用提醒</span>
                  <div className="flex min-w-0 flex-1 items-center justify-end">
                    <Switch
                      checked={r.enabled}
                      onChange={(v) => onToggle(r, v)}
                      size="sm"
                    />
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Button
        size="icon"
        onClick={openAdd}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增"
      >
        <Plus className="h-5 w-5" />
      </Button>

      <Modal
        open={modalOpen}
        title={editing ? '编辑生日提醒' : '新增生日提醒'}
        onClose={() => setModalOpen(false)}
      >
        <form onSubmit={submit} className="space-y-3.5">
          <div className="space-y-1.5">
            <Label htmlFor="bd-name">姓名</Label>
            <Input
              id="bd-name"
              value={form.name ?? ''}
              placeholder="张三"
              onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="bd-date">公历生日</Label>
            <Input
              id="bd-date"
              type="date"
              value={form.birthday ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, birthday: e.target.value }))}
              onClick={(e) => {
                try {
                  ;(e.currentTarget as HTMLInputElement).showPicker?.()
                } catch {
                  /* 老浏览器回退默认行为 */
                }
              }}
            />
          </div>
          {editing && (
            <>
              <div className="space-y-1.5">
                <Label>农历生日</Label>
                <Input value={editing.chinese_birthday || '保存后自动计算'} disabled />
              </div>
              <div className="space-y-1.5">
                <Label>生肖</Label>
                <Input value={editing.zodiac || '保存后自动计算'} disabled />
              </div>
            </>
          )}
          <div className="flex items-center justify-between">
            <Label htmlFor="bd-enabled">启用提醒</Label>
            <Switch
              id="bd-enabled"
              checked={Boolean(form.enabled)}
              onChange={(v) => setForm((p) => ({ ...p, enabled: v }))}
            />
          </div>
          {formErr && <p className="text-[12.5px] text-destructive">{formErr}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setModalOpen(false)}
            >
              取消
            </Button>
            <Button type="submit" size="sm" disabled={saving}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </div>
        </form>
      </Modal>

      <AlertDialog
        open={!!pendingDelete}
        onOpenChange={(o) => {
          if (!o) setPendingDelete(null)
        }}
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

function Row({ label, value }: { label: string; value: string }) {
  const empty = !value || !value.trim()
  return (
    <div className="flex items-center gap-3 py-1 text-[12.5px]">
      <span className="w-16 shrink-0 text-muted-foreground">{label}</span>
      {empty ? (
        <span className="min-w-0 flex-1 text-muted-foreground/70">—</span>
      ) : (
        <span
          className="min-w-0 flex-1 truncate font-mono text-[12.5px] leading-relaxed"
          title={value}
        >
          {value}
        </span>
      )}
    </div>
  )
}

import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Switch } from '../ui/switch'
import { Textarea } from '../ui/textarea'
import Modal from '../Modal'
import type { EventForm, EventItem } from './types'
import { blankEventForm } from './types'

interface EventEditDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target: EventItem | null
  onSave: (target: EventItem | null, form: EventForm) => Promise<void>
}

export function EventEditDialog({
  open,
  onOpenChange,
  target,
  onSave,
}: EventEditDialogProps) {
  const [form, setForm] = useState<EventForm>(blankEventForm)
  const [saving, setSaving] = useState(false)
  const [formErr, setFormErr] = useState('')

  useEffect(() => {
    if (!open) return
    setForm(
      target
        ? {
            title: target.title,
            event_date: target.event_date,
            lead_days: target.lead_days,
            remark: target.remark,
            enabled: target.enabled,
          }
        : blankEventForm,
    )
    setFormErr('')
  }, [open, target])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setFormErr('')
    try {
      await onSave(target, form)
      onOpenChange(false)
    } catch (err: any) {
      setFormErr(err?.response?.data?.error || err?.message || '提交失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      title={target ? '编辑事项提醒' : '新增事项提醒'}
      onClose={() => onOpenChange(false)}
    >
      <form onSubmit={submit} className="space-y-3.5">
        <div className="space-y-1.5">
          <Label htmlFor="ev-title">标题</Label>
          <Input
            id="ev-title"
            value={form.title ?? ''}
            placeholder="会议 / 体检 / 截止..."
            onChange={(e) => setForm((p) => ({ ...p, title: e.target.value }))}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="ev-date">事项日期</Label>
          <Input
            id="ev-date"
            type="date"
            value={form.event_date ?? ''}
            onChange={(e) => setForm((p) => ({ ...p, event_date: e.target.value }))}
            onClick={(e) => {
              try {
                ;(e.currentTarget as HTMLInputElement).showPicker?.()
              } catch {
                /* 老浏览器回退默认行为 */
              }
            }}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="ev-lead">提前天数</Label>
          <Input
            id="ev-lead"
            type="number"
            value={form.lead_days ?? ''}
            placeholder="5"
            onChange={(e) =>
              setForm((p) => ({
                ...p,
                lead_days: e.target.value === '' ? null : Number(e.target.value),
              }))
            }
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="ev-remark">备注</Label>
          <Textarea
            id="ev-remark"
            value={form.remark ?? ''}
            rows={3}
            onChange={(e) => setForm((p) => ({ ...p, remark: e.target.value }))}
          />
        </div>
        <div className="flex items-center justify-between">
          <Label htmlFor="ev-enabled">启用提醒</Label>
          <Switch
            id="ev-enabled"
            checked={Boolean(form.enabled)}
            onChange={(v) => setForm((p) => ({ ...p, enabled: v }))}
          />
        </div>
        {formErr && <p className="text-[12.5px] text-destructive">{formErr}</p>}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button type="submit" size="sm" disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

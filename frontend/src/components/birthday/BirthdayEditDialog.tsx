import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Switch } from '../ui/switch'
import Modal from '../Modal'
import type { Birthday, BirthdayForm } from './types'
import { blankBirthdayForm } from './types'

interface BirthdayEditDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  target: Birthday | null
  onSave: (target: Birthday | null, form: BirthdayForm) => Promise<void>
}

export function BirthdayEditDialog({
  open,
  onOpenChange,
  target,
  onSave,
}: BirthdayEditDialogProps) {
  const [form, setForm] = useState<BirthdayForm>(blankBirthdayForm)
  const [saving, setSaving] = useState(false)
  const [formErr, setFormErr] = useState('')

  useEffect(() => {
    if (!open) return
    setForm(target ? { name: target.name, birthday: target.birthday, enabled: target.enabled } : blankBirthdayForm)
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
      title={target ? '编辑生日提醒' : '新增生日提醒'}
      onClose={() => onOpenChange(false)}
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
        {target && (
          <>
            <div className="space-y-1.5">
              <Label>农历生日</Label>
              <Input value={target.chinese_birthday || '保存后自动计算'} disabled />
            </div>
            <div className="space-y-1.5">
              <Label>生肖</Label>
              <Input value={target.zodiac || '保存后自动计算'} disabled />
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

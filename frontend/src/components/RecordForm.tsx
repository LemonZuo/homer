import { useState, useEffect } from 'react'
import type { Field } from '../tables'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Label } from './ui/label'
import { Button } from './ui/button'
import { Switch } from './ui/switch'

interface Props {
  fields: Field[]
  initial?: Record<string, any>
  onSubmit: (data: Record<string, any>) => Promise<void> | void
  onCancel: () => void
}

export default function RecordForm({ fields, initial, onSubmit, onCancel }: Props) {
  const [data, setData] = useState<Record<string, any>>({})
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    setData(initial ?? {})
  }, [initial])

  const set = (k: string, v: any) => setData((prev) => ({ ...prev, [k]: v }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setErr('')
    try {
      await onSubmit(data)
    } catch (e: any) {
      setErr(e?.response?.data?.error || e?.message || '提交失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3.5">
      {fields.map((f) => {
        const fid = `field-${f.key}`
        if (f.type === 'switch') {
          return (
            <div key={f.key} className="flex items-center justify-between">
              <Label htmlFor={fid}>{f.label}</Label>
              <Switch
                id={fid}
                checked={Boolean(data[f.key])}
                onChange={(v) => set(f.key, v)}
                disabled={f.readonly}
              />
            </div>
          )
        }
        return (
          <div key={f.key} className="space-y-1.5">
            <Label htmlFor={fid}>{f.label}</Label>
            {f.type === 'textarea' ? (
              <Textarea
                id={fid}
                value={data[f.key] ?? ''}
                placeholder={f.placeholder}
                onChange={(e) => set(f.key, e.target.value)}
                disabled={f.readonly}
                rows={3}
              />
            ) : (
              <Input
                id={fid}
                type={
                  f.type === 'date'
                    ? 'date'
                    : f.type === 'password'
                      ? 'password'
                      : f.type === 'number'
                        ? 'number'
                        : 'text'
                }
                value={data[f.key] ?? ''}
                placeholder={f.placeholder}
                onChange={(e) => {
                  const v = e.target.value
                  if (f.type === 'number') {
                    set(f.key, v === '' ? null : Number(v))
                  } else {
                    set(f.key, v)
                  }
                }}
                onClick={
                  f.type === 'date' && !f.readonly
                    ? (e) => {
                        try {
                          ;(e.currentTarget as HTMLInputElement).showPicker?.()
                        } catch {
                          /* 老浏览器不支持时回退到默认行为 */
                        }
                      }
                    : undefined
                }
                disabled={f.readonly}
              />
            )}
          </div>
        )
      })}
      {err && <p className="text-[12.5px] text-destructive">{err}</p>}
      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          取消
        </Button>
        <Button type="submit" size="sm" disabled={loading}>
          {loading ? '保存中…' : '保存'}
        </Button>
      </div>
    </form>
  )
}

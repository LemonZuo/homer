import { useState, useEffect } from 'react'
import type { Field } from '../tables'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Label } from './ui/label'
import { Button } from './ui/button'

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
      {fields.map((f) => (
        <div key={f.key} className="space-y-1.5">
          <Label htmlFor={`field-${f.key}`}>{f.label}</Label>
          {f.type === 'textarea' ? (
            <Textarea
              id={`field-${f.key}`}
              value={data[f.key] ?? ''}
              placeholder={f.placeholder}
              onChange={(e) => set(f.key, e.target.value)}
              rows={3}
            />
          ) : (
            <Input
              id={`field-${f.key}`}
              value={data[f.key] ?? ''}
              placeholder={f.placeholder}
              onChange={(e) => set(f.key, e.target.value)}
            />
          )}
        </div>
      ))}
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

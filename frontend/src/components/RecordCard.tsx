import { useState } from 'react'
import { Copy, Check, Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import type { TableDef } from '../tables'
import { avatarColor, getColorSet } from '../colors'
import { cn } from '../lib/utils'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip'

interface Props {
  record: Record<string, any>
  def: TableDef
  onEdit: () => void
  onDelete: () => void
}

function pickFirst(r: Record<string, any>, keys: string[]): string {
  for (const k of keys) {
    const v = r[k]
    if (v !== undefined && v !== null && String(v).trim() !== '') return String(v)
  }
  return ''
}

export default function RecordCard({ record, def, onEdit, onDelete }: Props) {
  const [copied, setCopied] = useState<string | null>(null)

  const title = pickFirst(record, def.titleKeys) || '(无标题)'
  const subtitle = def.subtitleKeys
    .map((k) => record[k])
    .filter((v) => v !== undefined && v !== null && String(v).trim() !== '')
    .join(' · ')

  const avatarBg = avatarColor(title)
  const avatar = title.charAt(0).toUpperCase()
  const cs = getColorSet(def.color)

  const copy = async (key: string, val: any) => {
    if (val === undefined || val === null) return
    try {
      await navigator.clipboard.writeText(String(val))
      setCopied(key)
      setTimeout(() => setCopied((c) => (c === key ? null : c)), 1200)
    } catch {
      toast.error('复制失败')
    }
  }

  return (
    <Card
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
            avatarBg,
          )}
        >
          {avatar}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[14px] font-semibold tracking-tight">
            {title}
          </div>
          {subtitle && (
            <div className="mt-0.5 truncate text-[12px] text-muted-foreground">
              {subtitle}
            </div>
          )}
        </div>
        <div className="flex shrink-0 gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={onEdit}
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
                onClick={onDelete}
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
        {def.fields.map((f) => {
          const v = record[f.key]
          const empty = v === undefined || v === null || String(v).trim() === ''
          const s = empty ? '' : String(v)
          return (
            <div key={f.key} className="group/row flex items-center gap-3 py-1 text-[12.5px]">
              <span className="w-14 shrink-0 text-muted-foreground">{f.label}</span>
              {empty ? (
                <span className="flex-1 min-w-0 text-muted-foreground/70">—</span>
              ) : (
                <span
                  className="flex-1 min-w-0 truncate font-mono text-[12.5px] leading-relaxed"
                  title={s}
                >
                  {s}
                </span>
              )}
              {!empty && (
                <button
                  type="button"
                  onClick={() => copy(f.key, v)}
                  className={cn(
                    'shrink-0 rounded-md p-1 transition active:scale-90',
                    copied === f.key
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground',
                  )}
                  title={copied === f.key ? '已复制' : `复制${f.label}`}
                  aria-label={`复制${f.label}`}
                >
                  {copied === f.key ? (
                    <Check className="h-3.5 w-3.5" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                </button>
              )}
            </div>
          )
        })}
      </div>
    </Card>
  )
}

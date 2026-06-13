import { Inbox, Loader2, RefreshCw } from 'lucide-react'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { Input } from '../ui/input'
import { Select } from '../ui/select'
import type { QueryType, SmsItem } from './types'
import { simLabel, textOf, timeOf, whoOf } from './utils'

interface SmsQueryPanelProps {
  queryType: QueryType
  onQueryTypeChange: (type: QueryType) => void
  keyword: string
  onKeywordChange: (keyword: string) => void
  pageNum: number
  onPageNumChange: (page: number) => void
  pageSize: number
  onPageSizeChange: (size: number) => void
  queryLoading: boolean
  queryRaw: any
  list: SmsItem[]
  hasNext: boolean
  disabled: boolean
  onQuery: (overrides?: Partial<{ pageNum: number; type: QueryType; keyword: string; pageSize: number }>) => void
}

export function SmsQueryPanel({
  queryType,
  onQueryTypeChange,
  keyword,
  onKeywordChange,
  pageNum,
  onPageNumChange,
  pageSize,
  onPageSizeChange,
  queryLoading,
  queryRaw,
  list,
  hasNext,
  disabled,
  onQuery,
}: SmsQueryPanelProps) {
  return (
    <Card className="px-4 py-4">
      <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <Inbox className="h-4 w-4 text-muted-foreground" />
          <div className="text-[13px] font-medium">短信记录</div>
        </div>
        <div className="flex gap-2">
          {([1, 2] as const).map((type) => (
            <Button
              key={type}
              size="sm"
              variant={queryType === type ? 'default' : 'outline'}
              onClick={() => {
                onQueryTypeChange(type)
                onPageNumChange(1)
                onQuery({ type, pageNum: 1 })
              }}
            >
              {type === 1 ? '接收' : '发送'}
            </Button>
          ))}
        </div>
      </div>

      <div className="mb-3 flex flex-col gap-2 sm:flex-row">
        <Input
          placeholder="按号码/内容搜索"
          value={keyword}
          onChange={(e) => onKeywordChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              onPageNumChange(1)
              onQuery({ pageNum: 1 })
            }
          }}
        />
        <Button
          variant="outline"
          onClick={() => {
            onPageNumChange(1)
            onQuery({ pageNum: 1 })
          }}
          disabled={queryLoading || disabled}
        >
          {queryLoading ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          )}
          查询
        </Button>
      </div>

      {queryRaw == null ? (
        <p className="rounded-md border border-dashed border-border py-8 text-center text-[12px] text-muted-foreground">
          点击「查询」拉取记录
        </p>
      ) : list.length === 0 ? (
        <p className="rounded-md border border-dashed border-border py-8 text-center text-[12px] text-muted-foreground">
          没有数据
        </p>
      ) : (
        <div className="space-y-2">
          {list.map((item, index) => (
            <div key={item.id ?? index} className="rounded-md border border-border px-3 py-2">
              <div className="flex items-center justify-between gap-3 text-[12px]">
                <span className="truncate font-mono font-medium" title={whoOf(item)}>
                  {whoOf(item)}
                </span>
                <span className="shrink-0 text-muted-foreground">{timeOf(item)}</span>
              </div>
              <div className="mt-1 whitespace-pre-wrap break-words text-[12.5px] leading-relaxed">
                {textOf(item)}
              </div>
              {simLabel(item.sim_id) && (
                <div className="mt-1 text-[11px] text-muted-foreground">{simLabel(item.sim_id)}</div>
              )}
            </div>
          ))}
        </div>
      )}

      {queryRaw != null && (
        <div className="mt-3 flex items-center justify-between text-[12px] text-muted-foreground">
          <div>第 {pageNum} 页 · 本页 {list.length} 条</div>
          <div className="flex items-center gap-2">
            <Select<number>
              className="h-8 w-auto text-[12px]"
              value={pageSize}
              onChange={(size) => {
                onPageSizeChange(size)
                onPageNumChange(1)
                onQuery({ pageSize: size, pageNum: 1 })
              }}
              options={[10, 20, 50, 100].map((size) => ({ value: size, label: `${size}/页` }))}
            />
            <Button
              size="sm"
              variant="outline"
              disabled={queryLoading || pageNum <= 1}
              onClick={() => {
                const page = pageNum - 1
                onPageNumChange(page)
                onQuery({ pageNum: page })
              }}
            >
              上一页
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={queryLoading || !hasNext}
              onClick={() => {
                const page = pageNum + 1
                onPageNumChange(page)
                onQuery({ pageNum: page })
              }}
            >
              下一页
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}

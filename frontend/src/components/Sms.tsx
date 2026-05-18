import { useCallback, useEffect, useMemo, useState } from 'react'
import { Inbox, Loader2, RefreshCw, Send, Settings2, Smartphone } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { cn } from '../lib/utils'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Label } from './ui/label'
import { Select } from './ui/select'
import { DeviceInfo } from './sms/DeviceInfo'
import { ForwarderManageDrawer } from './sms/ForwarderManageDrawer'
import { LS_KEY, type Forwarder, type QueryType, type SimSlot } from './sms/types'
import { extractList, simLabel, textOf, timeOf, whoOf } from './sms/utils'

export default function Sms() {
  const cs = getColorSet('teal')

  const [forwarders, setForwarders] = useState<Forwarder[]>([])
  const [selectedID, setSelectedID] = useState<number | null>(() => {
    const v = localStorage.getItem(LS_KEY)
    return v ? Number(v) : null
  })
  const [manageOpen, setManageOpen] = useState(false)

  const [config, setConfig] = useState<any>(null)
  const [configLoading, setConfigLoading] = useState(false)

  const [sendForm, setSendForm] = useState<{ simSlot: SimSlot; phones: string; content: string }>({
    simSlot: 1,
    phones: '',
    content: '',
  })
  const [sending, setSending] = useState(false)

  const [queryType, setQueryType] = useState<QueryType>(1)
  const [keyword, setKeyword] = useState('')
  const [pageNum, setPageNum] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [queryLoading, setQueryLoading] = useState(false)
  const [queryRaw, setQueryRaw] = useState<any>(null)

  const enabledList = useMemo(() => forwarders.filter((f) => f.enabled), [forwarders])

  const loadForwarders = useCallback(async () => {
    try {
      const { data } = await api.get('/sms/forwarders')
      const rows: Forwarder[] = data?.data ?? []
      setForwarders(rows)
      setSelectedID((prev) => {
        const stillValid = prev != null && rows.some((r) => r.id === prev && r.enabled)
        if (stillValid) return prev
        const first = rows.find((r) => r.enabled)
        return first ? first.id : null
      })
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '加载转发器失败')
    }
  }, [])

  useEffect(() => {
    loadForwarders()
  }, [loadForwarders])

  useEffect(() => {
    if (selectedID != null) localStorage.setItem(LS_KEY, String(selectedID))
  }, [selectedID])

  // 切换/初始化转发器时自动拉一次设备信息 + 短信记录（接收，第 1 页）
  useEffect(() => {
    if (selectedID == null) {
      setConfig(null)
      setQueryRaw(null)
      return
    }
    let cancelled = false
    setConfigLoading(true)
    api
      .post('/sms/config/query', { target_id: selectedID })
      .then(({ data }) => {
        if (!cancelled) setConfig(data)
      })
      .catch((e) => {
        if (!cancelled) toast.error(e?.response?.data?.error || e?.message || '配置查询失败')
      })
      .finally(() => {
        if (!cancelled) setConfigLoading(false)
      })

    setQueryLoading(true)
    setPageNum(1)
    api
      .post('/sms/query', {
        target_id: selectedID,
        type: queryType,
        page_num: 1,
        page_size: pageSize,
        keyword: '',
      })
      .then(({ data }) => {
        if (!cancelled) setQueryRaw(data)
      })
      .catch((e) => {
        if (!cancelled) toast.error(e?.response?.data?.error || e?.message || '查询失败')
      })
      .finally(() => {
        if (!cancelled) setQueryLoading(false)
      })

    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedID])

  const requireTarget = useCallback(() => {
    if (selectedID == null) {
      toast.error('请先选择短信转发器')
      return false
    }
    return true
  }, [selectedID])

  const fetchConfig = useCallback(async () => {
    if (!requireTarget()) return
    setConfigLoading(true)
    try {
      const { data } = await api.post('/sms/config/query', { target_id: selectedID })
      setConfig(data)
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '配置查询失败')
    } finally {
      setConfigLoading(false)
    }
  }, [requireTarget, selectedID])

  const send = useCallback(async () => {
    if (!requireTarget()) return
    const phones = sendForm.phones.trim()
    const content = sendForm.content.trim()
    if (!phones) return toast.error('请填写手机号')
    if (!content) return toast.error('请填写短信内容')
    setSending(true)
    try {
      const { data } = await api.post('/sms/send', {
        target_id: selectedID,
        sim_slot: sendForm.simSlot,
        phone_numbers: phones,
        msg_content: content,
      })
      const code = data?.code ?? data?.errcode
      const msg = data?.msg ?? data?.message ?? data?.errmsg
      if (code !== undefined && code !== 200 && code !== 0) {
        toast.error(msg || '发送失败')
      } else {
        toast.success(msg || '已下发')
        setSendForm((f) => ({ ...f, content: '' }))
      }
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '发送失败')
    } finally {
      setSending(false)
    }
  }, [requireTarget, selectedID, sendForm])

  const runQuery = useCallback(
    async (overrides?: Partial<{ pageNum: number; type: QueryType; keyword: string; pageSize: number }>) => {
      if (!requireTarget()) return
      setQueryLoading(true)
      try {
        const { data } = await api.post('/sms/query', {
          target_id: selectedID,
          type: overrides?.type ?? queryType,
          page_num: overrides?.pageNum ?? pageNum,
          page_size: overrides?.pageSize ?? pageSize,
          keyword: overrides?.keyword ?? keyword,
        })
        setQueryRaw(data)
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '查询失败')
      } finally {
        setQueryLoading(false)
      }
    },
    [requireTarget, selectedID, queryType, pageNum, pageSize, keyword],
  )

  const list = useMemo(() => extractList(queryRaw), [queryRaw])
  const hasNext = list.length >= pageSize

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">短信转发器</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            通过 SmsForwarder Android 服务收发短信；签名鉴权模式
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select<number>
            className="h-9 min-w-[140px] flex-1 sm:flex-none"
            value={selectedID ?? 0}
            onChange={(v) => setSelectedID(v)}
            placeholder="无可用转发器"
            options={enabledList.map((f) => ({ value: f.id, label: f.name }))}
          />
          <Button variant="outline" size="sm" onClick={() => setManageOpen(true)}>
            <Settings2 className="mr-1.5 h-3.5 w-3.5" />
            管理
          </Button>
        </div>
      </div>

      {forwarders.length === 0 && (
        <Card className="mb-4 border-amber-500/40 bg-amber-500/5 px-4 py-3 text-[12.5px] text-amber-700 dark:text-amber-300">
          还没有短信转发器，点击右上角「管理」添加一个。
        </Card>
      )}

      {/* 设备信息 */}
      <Card className="mb-4 px-4 py-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Smartphone className="h-4 w-4 text-muted-foreground" />
            <div className="text-[13px] font-medium">设备信息</div>
          </div>
          <Button size="sm" variant="outline" onClick={fetchConfig} disabled={configLoading || selectedID == null}>
            {configLoading ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            )}
            刷新
          </Button>
        </div>
        {config ? (
          <DeviceInfo config={config} />
        ) : (
          <p className="rounded-md border border-dashed border-border py-6 text-center text-[12px] text-muted-foreground">
            点击「刷新」查询设备状态
          </p>
        )}
      </Card>

      {/* 发送短信 */}
      <Card className="mb-4 px-4 py-4">
        <div className="mb-3 flex items-center gap-2">
          <Send className="h-4 w-4 text-muted-foreground" />
          <div className="text-[13px] font-medium">发送短信</div>
        </div>
        <div className="space-y-3">
          <div>
            <Label className="mb-1.5 block text-[12px]">SIM 卡槽</Label>
            <div className="flex gap-2">
              {[1, 2].map((s) => (
                <Button
                  key={s}
                  type="button"
                  size="sm"
                  variant={sendForm.simSlot === s ? 'default' : 'outline'}
                  className="flex-1 sm:flex-none"
                  onClick={() => setSendForm((f) => ({ ...f, simSlot: s as SimSlot }))}
                >
                  SIM {s}
                </Button>
              ))}
            </div>
          </div>
          <div>
            <Label className="mb-1.5 block text-[12px]">收件人</Label>
            <Input
              placeholder="多个手机号用半角分号 ; 分隔"
              value={sendForm.phones}
              onChange={(e) => setSendForm((f) => ({ ...f, phones: e.target.value }))}
            />
          </div>
          <div>
            <Label className="mb-1.5 block text-[12px]">
              内容
              <span className="ml-2 text-muted-foreground">{sendForm.content.length} / 390 字</span>
            </Label>
            <Textarea
              rows={4}
              placeholder="70 字算一条，超出每 64 字递增一条，最多 6 条 / 390 字"
              maxLength={390}
              value={sendForm.content}
              onChange={(e) => setSendForm((f) => ({ ...f, content: e.target.value }))}
            />
          </div>
          <div className="flex justify-end">
            <Button onClick={send} disabled={sending || selectedID == null}>
              {sending ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Send className="mr-1.5 h-3.5 w-3.5" />
              )}
              发送
            </Button>
          </div>
        </div>
      </Card>

      {/* 短信记录 */}
      <Card className="px-4 py-4">
        <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Inbox className="h-4 w-4 text-muted-foreground" />
            <div className="text-[13px] font-medium">短信记录</div>
          </div>
          <div className="flex gap-2">
            {([1, 2] as const).map((t) => (
              <Button
                key={t}
                size="sm"
                variant={queryType === t ? 'default' : 'outline'}
                onClick={() => {
                  setQueryType(t)
                  setPageNum(1)
                  runQuery({ type: t, pageNum: 1 })
                }}
              >
                {t === 1 ? '接收' : '发送'}
              </Button>
            ))}
          </div>
        </div>

        <div className="mb-3 flex flex-col gap-2 sm:flex-row">
          <Input
            placeholder="按号码/内容搜索"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                setPageNum(1)
                runQuery({ pageNum: 1 })
              }
            }}
          />
          <Button
            variant="outline"
            onClick={() => {
              setPageNum(1)
              runQuery({ pageNum: 1 })
            }}
            disabled={queryLoading || selectedID == null}
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
            {list.map((it, i) => (
              <div key={it.id ?? i} className="rounded-md border border-border px-3 py-2">
                <div className="flex items-center justify-between gap-3 text-[12px]">
                  <span className="truncate font-mono font-medium" title={whoOf(it)}>
                    {whoOf(it)}
                  </span>
                  <span className="shrink-0 text-muted-foreground">{timeOf(it)}</span>
                </div>
                <div className="mt-1 whitespace-pre-wrap break-words text-[12.5px] leading-relaxed">
                  {textOf(it)}
                </div>
                {simLabel(it.sim_id) && (
                  <div className="mt-1 text-[11px] text-muted-foreground">{simLabel(it.sim_id)}</div>
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
                onChange={(n) => {
                  setPageSize(n)
                  setPageNum(1)
                  runQuery({ pageSize: n, pageNum: 1 })
                }}
                options={[10, 20, 50, 100].map((n) => ({ value: n, label: `${n}/页` }))}
              />
              <Button
                size="sm"
                variant="outline"
                disabled={queryLoading || pageNum <= 1}
                onClick={() => {
                  const p = pageNum - 1
                  setPageNum(p)
                  runQuery({ pageNum: p })
                }}
              >
                上一页
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={queryLoading || !hasNext}
                onClick={() => {
                  const p = pageNum + 1
                  setPageNum(p)
                  runQuery({ pageNum: p })
                }}
              >
                下一页
              </Button>
            </div>
          </div>
        )}
      </Card>

      <ForwarderManageDrawer
        open={manageOpen}
        onOpenChange={setManageOpen}
        forwarders={forwarders}
        onReload={loadForwarders}
      />
    </div>
  )
}

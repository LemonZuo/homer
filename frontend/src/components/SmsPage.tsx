import { useCallback, useEffect, useMemo, useState } from 'react'
import { Edit3, Inbox, Loader2, Plus, RefreshCw, Send, Settings2, Smartphone, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../api'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Textarea } from './ui/textarea'
import { Label } from './ui/label'
import { Select } from './ui/select'
import { Switch } from './ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'
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
import { cn } from '../lib/utils'

type QueryType = 1 | 2
type SimSlot = 1 | 2

// auth_mode 与 SmsForwarder Android「客户端安全措施」一致
type AuthMode = 0 | 1 | 2 | 3

const AUTH_MODES: { value: AuthMode; label: string }[] = [
  { value: 0, label: '无（明文）' },
  { value: 1, label: '签名（HmacSHA256）' },
  { value: 2, label: 'RSA' },
  { value: 3, label: 'SM4' },
]

interface Forwarder {
  id: number
  name: string
  server_url: string
  auth_mode: AuthMode
  sign_key: string
  rsa_public_key: string
  sm4_key: string
  timeout_seconds: number
  enabled: boolean
}

// SmsForwarder /sms/query 返回字段（pppscn/SmsForwarder Wiki 附录2）：
// name / number / content / date(ms) / type(1接收 2发送) / sim_id(0=SIM1 1=SIM2 -1未知) / sub_id
interface SmsItem {
  name?: string
  number?: string
  content?: string
  date?: number
  type?: number
  sim_id?: number
  sub_id?: number
  [k: string]: any
}

const LS_KEY = 'sms.forwarder.id'

function fmtJSON(v: any) {
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

const textOf = (it: SmsItem) => it.content ?? ''

const whoOf = (it: SmsItem) => {
  const num = it.number?.trim()
  const name = it.name?.trim()
  if (num && name && name !== 'Unknown Number') return `${name} · ${num}`
  return num || name || '—'
}

function fmtTime(v: any): string {
  if (v == null || v === '') return ''
  const n = Number(v)
  if (Number.isFinite(n) && n > 0) {
    const ms = n < 1e12 ? n * 1000 : n
    const d = new Date(ms)
    if (!isNaN(d.getTime())) {
      const p = (x: number) => String(x).padStart(2, '0')
      return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
    }
  }
  return String(v)
}

const timeOf = (it: SmsItem) => fmtTime(it.date)

const simLabel = (id?: number) => {
  if (id === 0) return 'SIM1'
  if (id === 1) return 'SIM2'
  return null
}

// /config/query data 部分
interface DeviceConfig {
  enable_api_battery_query?: boolean
  enable_api_call_query?: boolean
  enable_api_clone?: boolean
  enable_api_contact_add?: boolean
  enable_api_contact_query?: boolean
  enable_api_location?: boolean
  enable_api_sms_query?: boolean
  enable_api_sms_send?: boolean
  enable_api_wol?: boolean
  extra_device_mark?: string
  extra_sim1?: string
  extra_sim2?: string
  sim_info_list?: Record<
    string,
    {
      carrier_name?: string
      country_iso?: string
      icc_id?: string
      number?: string
      sim_slot_index?: number
      subscription_id?: number
    }
  >
  version_code?: number
  version_name?: string
}

const CAPABILITIES: { key: keyof DeviceConfig; label: string }[] = [
  { key: 'enable_api_sms_send', label: '发短信' },
  { key: 'enable_api_sms_query', label: '查短信' },
  { key: 'enable_api_call_query', label: '查通话' },
  { key: 'enable_api_contact_query', label: '查话簿' },
  { key: 'enable_api_contact_add', label: '加联系人' },
  { key: 'enable_api_battery_query', label: '查电池' },
  { key: 'enable_api_wol', label: '远程开机' },
  { key: 'enable_api_clone', label: '一键克隆' },
  { key: 'enable_api_location', label: '位置' },
]

function DeviceInfo({ config }: { config: any }) {
  const data: DeviceConfig | undefined = config?.data ?? config
  if (!data || typeof data !== 'object') {
    return (
      <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 font-mono text-[11.5px] leading-relaxed">
        {fmtJSON(config)}
      </pre>
    )
  }
  const sims = Object.values(data.sim_info_list ?? {})
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <div className="text-[14px] font-medium">{data.extra_device_mark || '未命名设备'}</div>
        {data.version_name && (
          <div className="font-mono text-[11.5px] text-muted-foreground">v{data.version_name}</div>
        )}
      </div>
      <div className="flex flex-wrap gap-1.5">
        {CAPABILITIES.map((c) => {
          const on = !!data[c.key]
          return (
            <span
              key={c.key}
              className={cn(
                'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                on
                  ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                  : 'bg-muted text-muted-foreground line-through',
              )}
            >
              {c.label}
            </span>
          )
        })}
      </div>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {[1, 2].map((slot) => {
          const labelKey = (slot === 1 ? 'extra_sim1' : 'extra_sim2') as keyof DeviceConfig
          const label = data[labelKey] as string | undefined
          const info = sims.find((s) => s?.sim_slot_index === slot - 1)
          return (
            <div key={slot} className="rounded-md border border-border px-3 py-2">
              <div className="flex items-baseline justify-between gap-2">
                <span className="text-[12.5px] font-medium">SIM{slot}</span>
                {label && (
                  <span className="truncate font-mono text-[11px] text-muted-foreground" title={label}>
                    {label}
                  </span>
                )}
              </div>
              {info ? (
                <div className="mt-1 space-y-0.5 font-mono text-[11px] text-muted-foreground">
                  {info.carrier_name && <div>{info.carrier_name}</div>}
                  {info.number && <div>{info.number}</div>}
                  {info.icc_id && <div className="truncate" title={info.icc_id}>ICCID {info.icc_id}</div>}
                </div>
              ) : (
                <div className="mt-1 text-[11px] text-muted-foreground">未插卡或未授予权限</div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

// SmsForwarder 标准信封 {code, msg, data: [...]}，data 即条目数组。
function extractList(payload: any): SmsItem[] {
  if (!payload) return []
  if (Array.isArray(payload?.data)) return payload.data
  if (Array.isArray(payload)) return payload
  return []
}

function ForwarderEditDialog({
  open,
  onOpenChange,
  target,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  target: Forwarder | null
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [serverURL, setServerURL] = useState('')
  const [authMode, setAuthMode] = useState<AuthMode>(1)
  const [signKey, setSignKey] = useState('')
  const [rsaPublicKey, setRSAPublicKey] = useState('')
  const [sm4Key, setSM4Key] = useState('')
  const [timeoutSeconds, setTimeoutSeconds] = useState<number | ''>(30)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setName(target?.name ?? '')
    setServerURL(target?.server_url ?? '')
    setAuthMode(target?.auth_mode ?? 1)
    setSignKey(target?.sign_key ?? '')
    setRSAPublicKey(target?.rsa_public_key ?? '')
    setSM4Key(target?.sm4_key ?? '')
    setTimeoutSeconds(target?.timeout_seconds ?? 30)
    setEnabled(target?.enabled ?? true)
  }, [open, target])

  const save = async () => {
    const timeout = typeof timeoutSeconds === 'number' ? timeoutSeconds : 0
    const payload = {
      name: name.trim(),
      server_url: serverURL.trim().replace(/\/+$/, ''),
      auth_mode: authMode,
      sign_key: signKey.trim(),
      rsa_public_key: rsaPublicKey.trim(),
      sm4_key: sm4Key.trim(),
      timeout_seconds: timeout > 0 ? timeout : 30,
      enabled,
    }
    if (!payload.name) return toast.error('名称必填')
    if (!payload.server_url) return toast.error('服务端地址必填')
    if (authMode === 1 && !payload.sign_key) return toast.error('签名模式需填签名密钥')
    if (authMode === 2 && !payload.rsa_public_key) return toast.error('RSA 模式需填服务端公钥')
    if (authMode === 3) {
      if (!payload.sm4_key) return toast.error('SM4 模式需填密钥')
      if (!/^[0-9a-fA-F]{32}$/.test(payload.sm4_key)) {
        return toast.error('SM4 密钥需为 32 位 hex（16 字节）')
      }
    }
    if (payload.timeout_seconds < 1 || payload.timeout_seconds > 300) {
      return toast.error('超时秒数需在 1 ~ 300 之间')
    }
    setSaving(true)
    try {
      if (target?.id) {
        await api.put(`/sms/forwarders/${target.id}`, payload)
      } else {
        await api.post('/sms/forwarders', payload)
      }
      toast.success('已保存')
      onOpenChange(false)
      onSaved()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] overflow-y-auto p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{target ? '编辑转发器' : '新增转发器'}</DialogTitle>
          <DialogDescription>
            服务端「客户端安全措施」需选「签名校验」，密钥与此处一致
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="fw-name">名称</Label>
            <Input
              id="fw-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如 家里旧手机 / 备用网关"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fw-url">服务端地址</Label>
            <Input
              id="fw-url"
              value={serverURL}
              onChange={(e) => setServerURL(e.target.value)}
              placeholder="http://192.168.1.100:5000"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fw-auth">客户端安全措施</Label>
            <Select<number>
              id="fw-auth"
              value={authMode}
              onChange={(v) => setAuthMode(v as AuthMode)}
              options={AUTH_MODES.map((m) => ({ value: m.value, label: m.label }))}
            />
            <p className="text-[11px] text-muted-foreground">
              需与服务端「设置 - 客户端安全措施」一致
            </p>
          </div>
          {authMode === 1 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-sign">签名密钥</Label>
              <Input
                id="fw-sign"
                value={signKey}
                onChange={(e) => setSignKey(e.target.value)}
                className="font-mono text-[12px]"
              />
            </div>
          )}
          {authMode === 2 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-rsa">RSA 公钥</Label>
              <Textarea
                id="fw-rsa"
                rows={4}
                value={rsaPublicKey}
                onChange={(e) => setRSAPublicKey(e.target.value)}
                placeholder="服务端 RSA 公钥，X.509/SPKI DER 的 Base64（不含 PEM 头尾）"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          {authMode === 3 && (
            <div className="grid gap-1.5">
              <Label htmlFor="fw-sm4">SM4 密钥</Label>
              <Input
                id="fw-sm4"
                value={sm4Key}
                onChange={(e) => setSM4Key(e.target.value)}
                placeholder="32 位 hex（16 字节）"
                className="font-mono text-[12px]"
              />
            </div>
          )}
          <div className="grid gap-1.5">
            <Label htmlFor="fw-timeout">请求超时（秒）</Label>
            <Input
              id="fw-timeout"
              type="number"
              min={1}
              max={300}
              value={timeoutSeconds}
              onChange={(e) => {
                const v = e.target.value
                setTimeoutSeconds(v === '' ? '' : Number(v))
              }}
              placeholder="默认 30，旧机器可调大"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="fw-enabled">启用</Label>
            <Switch id="fw-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
          </div>
        </div>
        <DialogFooter className="[&>button]:flex-1 sm:[&>button]:flex-none">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function SmsPage() {
  const cs = getColorSet('teal')

  const [forwarders, setForwarders] = useState<Forwarder[]>([])
  const [selectedID, setSelectedID] = useState<number | null>(() => {
    const v = localStorage.getItem(LS_KEY)
    return v ? Number(v) : null
  })
  const [manageOpen, setManageOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Forwarder | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [delTarget, setDelTarget] = useState<Forwarder | null>(null)

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

  const doDelete = useCallback(async () => {
    if (!delTarget) return
    try {
      await api.delete(`/sms/forwarders/${delTarget.id}`)
      toast.success('已删除')
      setDelTarget(null)
      loadForwarders()
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '删除失败')
    }
  }, [delTarget, loadForwarders])

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

      <Drawer open={manageOpen} onOpenChange={setManageOpen}>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>转发器管理</DrawerTitle>
            <DrawerDescription>配置多台 SmsForwarder 服务端，页面顶部可切换</DrawerDescription>
          </DrawerHeader>
          <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
            <div className="flex justify-end [&>button]:h-10 [&>button]:w-full sm:[&>button]:h-8 sm:[&>button]:w-auto">
              <Button
                size="sm"
                onClick={() => {
                  setEditTarget(null)
                  setEditOpen(true)
                }}
              >
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加转发器
              </Button>
            </div>
            {forwarders.length === 0 ? (
              <p className="py-8 text-center text-[12.5px] text-muted-foreground">还没有转发器</p>
            ) : (
              forwarders.map((f) => (
                <Card key={f.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">{f.name}</span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            f.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {f.enabled ? '启用' : '停用'}
                        </span>
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {f.server_url}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 sm:flex-none"
                        onClick={() => {
                          setEditTarget(f)
                          setEditOpen(true)
                        }}
                      >
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => setDelTarget(f)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </Card>
              ))
            )}
          </div>
        </DrawerContent>
      </Drawer>

      <ForwarderEditDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        onSaved={loadForwarders}
      />

      <AlertDialog open={!!delTarget} onOpenChange={(o) => !o && setDelTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除转发器</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除「{delTarget?.name}」？此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={doDelete}>删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

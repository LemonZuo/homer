import { Activity, Bell, CalendarDays, Cake, Cloud, Globe, Layers3, Loader2, LockKeyhole, MessageSquare, RefreshCw, Server, ShieldCheck, TrainFront, Zap } from 'lucide-react'
import { getColorSet } from '../colors'
import { useModules } from '../lib/modules'
import { cn } from '../lib/utils'
import { Button } from './ui/button'
import { Card } from './ui/card'
import { Switch } from './ui/switch'

const icons = { ups: Zap, esxi: Server, acme: ShieldCheck, certstore: Cloud, cdnops: Globe, birthday: Cake, event: CalendarDays, sms: MessageSquare, bypass: TrainFront }
const colors: Record<string, string> = { ups: 'teal', esxi: 'esxi', acme: 'emerald', certstore: 'violet', cdnops: 'sky', birthday: 'orange', event: 'blue', sms: 'teal', bypass: 'indigo' }

export default function Modules() {
  const { snapshot, loading, error, connected, pending, reload, update } = useModules()
  const modules = snapshot?.modules ?? []
  const groups = [...new Set(modules.map((module) => module.group))]
  const enabledCount = modules.filter((module) => module.enabled).length
  const cs = getColorSet('blue')

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <div className="hidden items-center gap-3 sm:flex">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">模块管理</h1>
          </div>
          <p className="text-[12.5px] leading-relaxed text-muted-foreground sm:mt-2">按需开启小管家的功能，保存后即时生效</p>
        </div>
        <Button variant="outline" size="sm" className="h-10 w-full sm:h-8 sm:w-auto" onClick={() => void reload()} disabled={loading}>
          {loading ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1.5 h-3.5 w-3.5" />}
          刷新
        </Button>
      </div>

      {error && <div role="alert" className="mb-4 rounded-xl border border-rose-500/20 bg-rose-500/5 px-4 py-3 text-[12.5px] text-rose-600 dark:text-rose-400">{error}。已保存的开关不会被更改，请刷新重试。</div>}

      <Card className="mb-6 overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5 sm:px-5">
          <div className="flex items-center gap-3">
            <div className={cn('flex h-9 w-9 items-center justify-center rounded-lg', cs.picker)}><Layers3 className="h-[18px] w-[18px]" /></div>
            <div>
              <p className="text-[13px] font-semibold">{snapshot ? `${enabledCount} 个已启用 · ${modules.length - enabledCount} 个已禁用` : '正在读取模块配置'}</p>
              <p className="mt-0.5 text-[11.5px] text-muted-foreground">设备、凭证和历史记录始终保留</p>
            </div>
          </div>
          <span className="flex items-center gap-1.5 text-[11.5px] text-muted-foreground" role="status">
            <span className={cn('h-1.5 w-1.5 rounded-full', connected ? 'bg-emerald-500' : 'bg-amber-500')} />
            {connected ? '实时同步' : '等待连接 · 定期同步'}
          </span>
        </div>
        <div className="border-t border-border/60 bg-muted/25 px-4 py-2.5 text-[12px] leading-relaxed text-muted-foreground sm:px-5">禁用后菜单和页面入口会隐藏，相关后台任务暂停。已开始的操作会安全收尾。</div>
      </Card>

      {!snapshot && <Card className="flex flex-col items-center gap-3 py-14 text-[13px] text-muted-foreground"><Layers3 className="h-7 w-7 opacity-40" />{loading ? '加载中…' : '暂时无法读取配置，请重试'}</Card>}

      <div className="space-y-6">
        {groups.map((group) => (
          <section key={group} aria-label={group}>
            <h2 className="mb-2.5 px-0.5 text-[13px] font-semibold tracking-tight">{group}</h2>
            <div className="space-y-2.5">
              {modules.filter((module) => module.group === group).map((module) => {
                const Icon = icons[module.key as keyof typeof icons] ?? Layers3
                const accent = getColorSet(colors[module.key] ?? 'blue')
                const saving = pending.includes(module.key)
                return (
                  <Card key={module.key} className={cn('overflow-hidden transition-colors', module.enabled && accent.border)}>
                    <div className="flex items-start gap-3 p-3.5 sm:p-4">
                      <div className={cn('mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', module.enabled ? accent.picker : 'bg-muted text-muted-foreground')}><Icon className="h-[18px] w-[18px]" /></div>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                          <label htmlFor={`module-${module.key}`} className="cursor-pointer text-[14px] font-semibold">{module.label}</label>
                          <span className={cn('rounded-full px-2 py-0.5 text-[10.5px] font-medium', module.enabled ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-muted text-muted-foreground')}>{module.enabled ? '已启用' : '已禁用'}</span>
                        </div>
                        <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground">{module.description}</p>
                        {!module.enabled && module.active > 0 && <p className="mt-1.5 flex items-center gap-1.5 text-[11.5px] text-amber-600 dark:text-amber-400"><Loader2 className="h-3 w-3 animate-spin" />仍有操作正在收尾</p>}
                      </div>
                      <div className="flex min-h-9 shrink-0 items-center gap-2">
                        {saving && <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
                        <Switch id={`module-${module.key}`} ariaLabel={`${module.label}模块`} checked={module.enabled} disabled={saving} onChange={(enabled) => void update(module, enabled)} />
                      </div>
                    </div>
                    <div className="border-t border-border/50 bg-muted/20 px-3.5 py-2.5 text-[11.5px] leading-relaxed text-muted-foreground sm:px-4">
                      <p><span className="font-medium text-foreground/70">禁用影响：</span>{module.impact}</p>
                      {module.hint && <p className="mt-0.5 text-muted-foreground/80">{module.hint}</p>}
                    </div>
                  </Card>
                )
              })}
            </div>
          </section>
        ))}
      </div>

      <div className="mt-7 rounded-xl border border-dashed border-border px-4 py-4">
        <div className="flex items-center gap-2 text-[12.5px] font-medium"><LockKeyhole className="h-3.5 w-3.5 text-muted-foreground" />始终可用</div>
        <div className="mt-2.5 flex flex-wrap gap-x-5 gap-y-2 text-[12px] text-muted-foreground">
          <span className="flex items-center gap-1.5"><Activity className="h-3.5 w-3.5" />任务调度</span>
          <span className="flex items-center gap-1.5"><Bell className="h-3.5 w-3.5" />通知渠道</span>
          <span className="flex items-center gap-1.5"><Layers3 className="h-3.5 w-3.5" />模块管理</span>
        </div>
        <p className="mt-2 text-[11.5px] leading-relaxed text-muted-foreground">公共能力保持运行，便于查看任务状态、管理通知和重新启用业务。</p>
      </div>
    </div>
  )
}

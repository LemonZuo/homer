import { cn } from '../../lib/utils'
import { CAPABILITIES, type DeviceConfig } from './types'
import { fmtJSON } from './utils'

export function DeviceInfo({ config }: { config: any }) {
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

import { Edit3, KeyRound, Plus, RefreshCw, Server, Trash2 } from 'lucide-react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../ui/drawer'
import { cn } from '../../lib/utils'
import type { EsxiHost } from './types'
import { authLabel } from './types'

export function EsxiHostsDrawer({
  open,
  onOpenChange,
  hosts,
  onAdd,
  onEdit,
  onDelete,
  onTest,
  onManageCredentials,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  hosts: EsxiHost[]
  onAdd: () => void
  onEdit: (h: EsxiHost) => void
  onDelete: (h: EsxiHost) => void
  onTest: (h: EsxiHost) => void
  onManageCredentials: () => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>ESXi 机器</DrawerTitle>
          <DrawerDescription>
            管理参与 ESXi 状态采样的主机;凭证库独立于 UPS / ACME
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button size="sm" variant="outline" onClick={onManageCredentials}>
              <KeyRound className="mr-1.5 h-3.5 w-3.5" />
              登录凭证
            </Button>
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加机器
            </Button>
          </div>
          {hosts.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 ESXi 机器,点「添加机器」新增
            </p>
          ) : (
            <div className="space-y-2">
              {hosts.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-3">
                    <Server className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="truncate font-mono text-[13px] font-medium">{t.name}</span>
                        <span
                          className={cn(
                            'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                            t.enabled
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {t.enabled ? '启用' : '停用'}
                        </span>
                      </div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-[11.5px] text-muted-foreground">
                        <span className="truncate">
                          {t.auth_source === 'credential'
                            ? `凭证@${t.endpoint} · 登录凭证`
                            : `${t.username || '未配置用户'}@${t.endpoint} · ${authLabel(t.auth_type)}`}
                        </span>
                        {t.bastion_host_id > 0 && (() => {
                          const b = hosts.find((x) => x.id === t.bastion_host_id)
                          return (
                            <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                              经 {b ? b.name : `#${t.bastion_host_id}`}
                            </span>
                          )
                        })()}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 sm:flex-none"
                        onClick={() => onTest(t)}
                        disabled={!t.enabled}
                      >
                        <RefreshCw className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 sm:flex-none"
                        onClick={() => onEdit(t)}
                      >
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => onDelete(t)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

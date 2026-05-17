import { Edit3, KeyRound, Plus, RefreshCw, Server, ShieldCheck, Trash2 } from 'lucide-react'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import { AliyunIcon } from '../../icons/AliyunIcon'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../../ui/drawer'
import { cn } from '../../../lib/utils'
import type { CASTarget, SSHTarget, SafelineTarget } from '../types'
import { authLabel } from '../utils'

export function DeployTargetsEntryDrawer({
  open,
  onOpenChange,
  sshTargets,
  safelineTargets,
  casTargets,
  onAddSSH,
  onEditSSH,
  onDeleteSSH,
  onTestSSH,
  onManageCredentials,
  onAddSafeline,
  onEditSafeline,
  onDeleteSafeline,
  onTestSafeline,
  onAddCAS,
  onEditCAS,
  onDeleteCAS,
  onTestCAS,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  casTargets: CASTarget[]
  onAddSSH: () => void
  onEditSSH: (t: SSHTarget) => void
  onDeleteSSH: (t: SSHTarget) => void
  onTestSSH: (t: SSHTarget) => void
  onManageCredentials: () => void
  onAddSafeline: () => void
  onEditSafeline: (t: SafelineTarget) => void
  onDeleteSafeline: (t: SafelineTarget) => void
  onTestSafeline: (t: SafelineTarget) => void
  onAddCAS: () => void
  onEditCAS: (t: CASTarget) => void
  onDeleteCAS: (t: CASTarget) => void
  onTestCAS: (t: CASTarget) => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>部署目标</DrawerTitle>
          <DrawerDescription>
            管理证书部署时可选择的远程目标
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-5 overflow-auto px-4 pb-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-[13px] font-medium">SSH 机器</div>
              <div className="text-[11.5px] text-muted-foreground">{sshTargets.length} 台机器</div>
            </div>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={onManageCredentials}>
                <KeyRound className="mr-1.5 h-3.5 w-3.5" />
                登录凭证
              </Button>
              <Button size="sm" onClick={onAddSSH}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加机器
              </Button>
            </div>
          </div>
          {sshTargets.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有 SSH 机器
            </p>
          ) : (
            <div className="space-y-2">
              {sshTargets.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                    <Server className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
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
                            ? `凭证@${t.host}:${t.port || 22} · 登录凭证`
                            : `${t.username}@${t.host}:${t.port || 22} · ${authLabel(t.auth_type)}`}
                        </span>
                        {(t.bastion_target_id ?? 0) > 0 && (() => {
                          const b = sshTargets.find((x) => x.id === t.bastion_target_id)
                          return (
                            <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                              经 {b ? b.name : `#${t.bastion_target_id}`}
                            </span>
                          )
                        })()}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onTestSSH(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditSSH(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => onDeleteSSH(t)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between gap-3 border-t border-border pt-5">
            <div>
              <div className="text-[13px] font-medium">雷池 WAF</div>
              <div className="text-[11.5px] text-muted-foreground">{safelineTargets.length} 个实例</div>
            </div>
            <Button size="sm" onClick={onAddSafeline}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加实例
            </Button>
          </div>
          {safelineTargets.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有雷池实例
            </p>
          ) : (
            <div className="space-y-2">
              {safelineTargets.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                    <ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
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
                        {t.skip_tls_verify && (
                          <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
                            跳过 TLS
                          </span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {t.base_url}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onTestSafeline(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditSafeline(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => onDeleteSafeline(t)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between gap-3 border-t border-border pt-5">
            <div>
              <div className="text-[13px] font-medium">阿里云 CAS</div>
              <div className="text-[11.5px] text-muted-foreground">{casTargets.length} 个实例</div>
            </div>
            <Button size="sm" onClick={onAddCAS}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加实例
            </Button>
          </div>
          {casTargets.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
              还没有阿里云 CAS 实例
            </p>
          ) : (
            <div className="space-y-2">
              {casTargets.map((t) => (
                <Card key={t.id} className="px-4 py-3">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                    <AliyunIcon className="h-4 w-4 shrink-0" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
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
                      <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                        {t.access_key_id || '未配置 AK'}
                      </div>
                    </div>
                    <div className="flex gap-2 sm:contents">
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onTestCAS(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditCAS(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="flex-1 hover:text-destructive sm:flex-none"
                        onClick={() => onDeleteCAS(t)}
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

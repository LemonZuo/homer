import { Edit3, HardDrive, KeyRound, Plus, RefreshCw, Server, ShieldCheck, Trash2 } from 'lucide-react'
import { Button } from '../../ui/button'
import { AliyunIcon } from '../../icons/AliyunIcon'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../../ui/drawer'
import type { CASTarget, FnOSTarget, SSHTarget, SafelineTarget } from '../types'
import { authLabel } from '../utils'
import { CardIconButton, CardTitleRow, DeployCard, EmptyState, EnabledBadge } from './DeployDrawerParts'

export function DeployTargetsEntryDrawer({
  open,
  onOpenChange,
  sshTargets,
  safelineTargets,
  casTargets,
  fnosTargets,
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
  onAddFnOS,
  onEditFnOS,
  onDeleteFnOS,
  onTestFnOS,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  casTargets: CASTarget[]
  fnosTargets: FnOSTarget[]
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
  onAddFnOS: () => void
  onEditFnOS: (t: FnOSTarget) => void
  onDeleteFnOS: (t: FnOSTarget) => void
  onTestFnOS: (t: FnOSTarget) => void
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
            <EmptyState>还没有 SSH 机器</EmptyState>
          ) : (
            <div className="space-y-2">
              {sshTargets.map((t) => (
                <DeployCard
                  key={t.id}
                  icon={<Server className="h-4 w-4 shrink-0 text-muted-foreground" />}
                  actions={
                    <>
                      <CardIconButton onClick={() => onTestSSH(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton onClick={() => onEditSSH(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton destructive onClick={() => onDeleteSSH(t)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </CardIconButton>
                    </>
                  }
                >
                  <CardTitleRow title={t.name}>
                    <EnabledBadge enabled={t.enabled} />
                  </CardTitleRow>
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
                </DeployCard>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between gap-3 border-t border-border pt-5">
            <div>
              <div className="text-[13px] font-medium">飞牛 OS</div>
              <div className="text-[11.5px] text-muted-foreground">{fnosTargets.length} 个实例</div>
            </div>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={onManageCredentials}>
                <KeyRound className="mr-1.5 h-3.5 w-3.5" />
                登录凭证
              </Button>
              <Button size="sm" onClick={onAddFnOS}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                添加实例
              </Button>
            </div>
          </div>
          {fnosTargets.length === 0 ? (
            <EmptyState>还没有 fnOS 实例</EmptyState>
          ) : (
            <div className="space-y-2">
              {fnosTargets.map((t) => (
                <DeployCard
                  key={t.id}
                  icon={<HardDrive className="h-4 w-4 shrink-0 text-muted-foreground" />}
                  actions={
                    <>
                      <CardIconButton onClick={() => onTestFnOS(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton onClick={() => onEditFnOS(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton destructive onClick={() => onDeleteFnOS(t)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </CardIconButton>
                    </>
                  }
                >
                  <CardTitleRow title={t.name}>
                    <EnabledBadge enabled={t.enabled} />
                  </CardTitleRow>
                  <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                    {t.auth_source === 'credential'
                      ? `凭证@${t.host}:${t.port || 22} · 登录凭证`
                      : `${t.username || '未配置用户'}@${t.host}:${t.port || 22} · ${authLabel(t.auth_type)}`}
                  </div>
                </DeployCard>
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
            <EmptyState>还没有雷池实例</EmptyState>
          ) : (
            <div className="space-y-2">
              {safelineTargets.map((t) => (
                <DeployCard
                  key={t.id}
                  icon={<ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground" />}
                  actions={
                    <>
                      <CardIconButton onClick={() => onTestSafeline(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton onClick={() => onEditSafeline(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton destructive onClick={() => onDeleteSafeline(t)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </CardIconButton>
                    </>
                  }
                >
                  <CardTitleRow title={t.name}>
                    <EnabledBadge enabled={t.enabled} />
                    {t.skip_tls_verify && (
                      <span className="rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
                        跳过 TLS
                      </span>
                    )}
                  </CardTitleRow>
                  <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                    {t.base_url}
                  </div>
                </DeployCard>
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
            <EmptyState>还没有阿里云 CAS 实例</EmptyState>
          ) : (
            <div className="space-y-2">
              {casTargets.map((t) => (
                <DeployCard
                  key={t.id}
                  icon={<AliyunIcon className="h-4 w-4 shrink-0" />}
                  actions={
                    <>
                      <CardIconButton onClick={() => onTestCAS(t)} disabled={!t.enabled}>
                        <RefreshCw className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton onClick={() => onEditCAS(t)}>
                        <Edit3 className="h-3.5 w-3.5" />
                      </CardIconButton>
                      <CardIconButton destructive onClick={() => onDeleteCAS(t)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </CardIconButton>
                    </>
                  }
                >
                  <CardTitleRow title={t.name}>
                    <EnabledBadge enabled={t.enabled} />
                  </CardTitleRow>
                  <div className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground">
                    {t.access_key_id || '未配置 AK'}
                  </div>
                </DeployCard>
              ))}
            </div>
          )}

        </div>
      </DrawerContent>
    </Drawer>
  )
}

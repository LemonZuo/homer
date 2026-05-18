import { Copy, Edit3, HardDrive, Loader2, Plus, Send, ShieldCheck, Trash2 } from 'lucide-react'
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
import type {
  CASDeployConfig,
  CASTarget,
  Domain,
  FnOSDeployConfig,
  FnOSTarget,
  SSHDeployConfig,
  SSHTarget,
  SafelineDeployConfig,
  SafelineTarget,
} from '../types'
import {
  casConfigTitle,
  casTargetByID,
  casTargetSummary,
  configPrimaryPath,
  configTitle,
  fnosConfigTitle,
  fnosTargetByID,
  fnosTargetSummary,
  safelineConfigTitle,
  safelineTargetByID,
  safelineTargetSummary,
  targetByID,
  targetSummary,
} from '../utils'

export function DeployConfigsDrawer({
  open,
  onOpenChange,
  domain,
  sshConfigs,
  safelineConfigs,
  casConfigs,
  fnosConfigs,
  sshTargets,
  safelineTargets,
  casTargets,
  fnosTargets,
  loading,
  busy,
  onAddSSH,
  onEditSSH,
  onCopySSH,
  onDeleteSSH,
  onDeploySSH,
  onAddSafeline,
  onEditSafeline,
  onDeleteSafeline,
  onDeploySafeline,
  onAddCAS,
  onEditCAS,
  onDeleteCAS,
  onDeployCAS,
  onAddFnOS,
  onEditFnOS,
  onDeleteFnOS,
  onDeployFnOS,
  onDeployAll,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  sshConfigs: SSHDeployConfig[]
  safelineConfigs: SafelineDeployConfig[]
  casConfigs: CASDeployConfig[]
  fnosConfigs: FnOSDeployConfig[]
  sshTargets: SSHTarget[]
  safelineTargets: SafelineTarget[]
  casTargets: CASTarget[]
  fnosTargets: FnOSTarget[]
  loading: boolean
  busy: string | null
  onAddSSH: () => void
  onEditSSH: (cfg: SSHDeployConfig) => void
  onCopySSH: (cfg: SSHDeployConfig) => void
  onDeleteSSH: (cfg: SSHDeployConfig) => void
  onDeploySSH: (cfg: SSHDeployConfig) => void
  onAddSafeline: () => void
  onEditSafeline: (cfg: SafelineDeployConfig) => void
  onDeleteSafeline: (cfg: SafelineDeployConfig) => void
  onDeploySafeline: (cfg: SafelineDeployConfig) => void
  onAddCAS: () => void
  onEditCAS: (cfg: CASDeployConfig) => void
  onDeleteCAS: (cfg: CASDeployConfig) => void
  onDeployCAS: (cfg: CASDeployConfig) => void
  onAddFnOS: () => void
  onEditFnOS: (cfg: FnOSDeployConfig) => void
  onDeleteFnOS: (cfg: FnOSDeployConfig) => void
  onDeployFnOS: (cfg: FnOSDeployConfig) => void
  onDeployAll: () => void
}) {
  const revoked = domain?.cert_status === 'revoked'
  const hasCert = Boolean(domain?.not_after)
  const sshDeployableCount = sshConfigs.filter((cfg) => {
    const t = targetByID(sshTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const safelineDeployableCount = safelineConfigs.filter((cfg) => {
    const t = safelineTargetByID(safelineTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const casDeployableCount = casConfigs.filter((cfg) => {
    const t = casTargetByID(casTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const fnosDeployableCount = fnosConfigs.filter((cfg) => {
    const t = fnosTargetByID(fnosTargets, cfg.target_id)
    return cfg.enabled && Boolean(t?.enabled)
  }).length
  const deployableCount = sshDeployableCount + safelineDeployableCount + casDeployableCount + fnosDeployableCount
  const deployingAll = Boolean(domain && busy === `deploy-domain-${domain.id}`)
  const canDeployAll = hasCert && !revoked && deployableCount > 0 && busy === null

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>部署配置</DrawerTitle>
          <DrawerDescription>
            {domain?.main_domain ?? '当前域名'} 的证书部署配置
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-5 overflow-auto px-4 pb-4">
          <div className="flex flex-wrap justify-end gap-2 [&>button]:h-10 [&>button]:flex-1 sm:[&>button]:h-8 sm:[&>button]:flex-none">
            <Button
              size="sm"
              variant="outline"
              onClick={onDeployAll}
              disabled={!canDeployAll}
              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : deployableCount === 0 ? '没有可部署的启用配置' : undefined}
            >
              {deployingAll ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Send className="mr-1.5 h-3.5 w-3.5" />
              )}
              一键部署
            </Button>
            <Button size="sm" onClick={onAddSSH} disabled={sshTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加 SSH
            </Button>
            <Button size="sm" onClick={onAddFnOS} disabled={fnosTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加 fnOS
            </Button>
            <Button size="sm" onClick={onAddSafeline} disabled={safelineTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加雷池
            </Button>
            <Button size="sm" onClick={onAddCAS} disabled={casTargets.length === 0}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加 CAS
            </Button>
          </div>

          {loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <>
              <section className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-[13px] font-medium">SSH 部署</div>
                    <div className="text-[11.5px] text-muted-foreground">
                      写入远程文件并执行命令
                    </div>
                  </div>
                </div>
                {sshTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加 SSH 机器，再配置部署路径
                  </p>
                ) : sshConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有 SSH 部署配置
                  </p>
                ) : (
                  sshConfigs.map((cfg) => {
                    const t = targetByID(sshTargets, cfg.target_id)
                    const deploying = busy === `deploy-ssh-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-3">
                          <Send className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {configTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {targetSummary(t)}
                            </div>
                            <div
                              className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground"
                              title={configPrimaryPath(cfg)}
                            >
                              {configPrimaryPath(cfg)}
                            </div>
                          </div>
                          <div className="flex gap-2 sm:contents">
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 sm:flex-none"
                              onClick={() => onCopySSH(cfg)}
                              title="基于当前配置复制一份"
                            >
                              <Copy className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 sm:flex-none"
                              onClick={() => onDeploySSH(cfg)}
                              disabled={!canDeploy}
                              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                            >
                              {deploying ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              ) : (
                                <Send className="h-3.5 w-3.5" />
                              )}
                            </Button>
                            <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditSSH(cfg)}>
                              <Edit3 className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 hover:text-destructive sm:flex-none"
                              onClick={() => onDeleteSSH(cfg)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>

              <section className="space-y-2 border-t border-border pt-5">
                <div>
                  <div className="text-[13px] font-medium">fnOS 部署</div>
                  <div className="text-[11.5px] text-muted-foreground">
                    覆盖飞牛 OS ssls 目录证书并更新 trim_connect.cert
                  </div>
                </div>
                {fnosTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加 fnOS 实例，再配置部署
                  </p>
                ) : fnosConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有 fnOS 部署配置
                  </p>
                ) : (
                  fnosConfigs.map((cfg) => {
                    const t = fnosTargetByID(fnosTargets, cfg.target_id)
                    const deploying = busy === `deploy-fnos-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-3">
                          <HardDrive className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {fnosConfigTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {fnosTargetSummary(t)}
                            </div>
                            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                              {cfg.domain_override
                                ? `域名：${cfg.domain_override}`
                                : `域名：${domain?.main_domain ?? '（主域名）'}`}
                            </div>
                          </div>
                          <div className="flex gap-2 sm:contents">
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 sm:flex-none"
                              onClick={() => onDeployFnOS(cfg)}
                              disabled={!canDeploy}
                              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                            >
                              {deploying ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              ) : (
                                <HardDrive className="h-3.5 w-3.5" />
                              )}
                            </Button>
                            <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditFnOS(cfg)}>
                              <Edit3 className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 hover:text-destructive sm:flex-none"
                              onClick={() => onDeleteFnOS(cfg)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>

              <section className="space-y-2 border-t border-border pt-5">
                <div>
                  <div className="text-[13px] font-medium">雷池部署</div>
                  <div className="text-[11.5px] text-muted-foreground">
                    上传到 WAF 证书管理
                  </div>
                </div>
                {safelineTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加雷池实例，再配置证书上传
                  </p>
                ) : safelineConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有雷池部署配置
                  </p>
                ) : (
                  safelineConfigs.map((cfg) => {
                    const t = safelineTargetByID(safelineTargets, cfg.target_id)
                    const deploying = busy === `deploy-safeline-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-3">
                          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {safelineConfigTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {safelineTargetSummary(t)}
                            </div>
                            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                              cert_id={cfg.cert_id || '新增'} · type={cfg.cert_type || 2}
                            </div>
                          </div>
                          <div className="flex gap-2 sm:contents">
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 sm:flex-none"
                              onClick={() => onDeploySafeline(cfg)}
                              disabled={!canDeploy}
                              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                            >
                              {deploying ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              ) : (
                                <ShieldCheck className="h-3.5 w-3.5" />
                              )}
                            </Button>
                            <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditSafeline(cfg)}>
                              <Edit3 className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 hover:text-destructive sm:flex-none"
                              onClick={() => onDeleteSafeline(cfg)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>

              <section className="space-y-2 border-t border-border pt-5">
                <div>
                  <div className="text-[13px] font-medium">阿里云 CAS 部署</div>
                  <div className="text-[11.5px] text-muted-foreground">
                    上传到阿里云数字证书管理（每次新增）
                  </div>
                </div>
                {casTargets.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    先添加阿里云 CAS 实例，再配置证书上传
                  </p>
                ) : casConfigs.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-[12.5px] text-muted-foreground">
                    还没有阿里云 CAS 部署配置
                  </p>
                ) : (
                  casConfigs.map((cfg) => {
                    const t = casTargetByID(casTargets, cfg.target_id)
                    const deploying = busy === `deploy-cas-config-${cfg.id}`
                    const canDeploy = hasCert && !revoked && cfg.enabled && Boolean(t?.enabled) && busy === null
                    return (
                      <Card key={cfg.id} className="px-4 py-3">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-3">
                          <AliyunIcon className="mt-0.5 h-4 w-4 shrink-0" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="truncate font-mono text-[13px] font-medium">
                                {casConfigTitle(cfg)}
                              </span>
                              <span
                                className={cn(
                                  'rounded-md px-1.5 py-0.5 text-[11px] font-medium',
                                  cfg.enabled
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                    : 'bg-muted text-muted-foreground',
                                )}
                              >
                                {cfg.enabled ? '启用' : '停用'}
                              </span>
                              {cfg.auto_deploy && (
                                <span className="rounded-md bg-sky-500/10 px-1.5 py-0.5 text-[11px] font-medium text-sky-600 dark:text-sky-400">
                                  自动部署
                                </span>
                              )}
                            </div>
                            <div className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                              {casTargetSummary(t)}
                            </div>
                            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                              上次 cert_id={cfg.cert_id || '—'}
                            </div>
                          </div>
                          <div className="flex gap-2 sm:contents">
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 sm:flex-none"
                              onClick={() => onDeployCAS(cfg)}
                              disabled={!canDeploy}
                              title={!hasCert ? '当前域名还没有证书' : revoked ? '当前证书已吊销' : undefined}
                            >
                              {deploying ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              ) : (
                                <AliyunIcon className="h-3.5 w-3.5" />
                              )}
                            </Button>
                            <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEditCAS(cfg)}>
                              <Edit3 className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="flex-1 hover:text-destructive sm:flex-none"
                              onClick={() => onDeleteCAS(cfg)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>
                      </Card>
                    )
                  })
                )}
              </section>
            </>
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

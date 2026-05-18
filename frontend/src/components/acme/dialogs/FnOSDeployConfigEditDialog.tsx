import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Label } from '../../ui/label'
import { Select } from '../../ui/select'
import { Switch } from '../../ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { Domain, FnOSDeployConfig, FnOSTarget } from '../types'
import { fnosConfigToDeployConfig } from '../utils'

export function FnOSDeployConfigEditDialog({
  open,
  onOpenChange,
  domain,
  config,
  targets,
  onSaved,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  domain: Domain | null
  config: FnOSDeployConfig | null
  targets: FnOSTarget[]
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [domainOverride, setDomainOverride] = useState('')
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setDomainOverride(config?.domain_override ?? '')
    setAutoDeploy(config?.auto_deploy ?? false)
    setEnabled(config?.enabled ?? true)
  }, [open, config, targets])

  const save = async () => {
    if (!domain) return
    const form: FnOSDeployConfig = {
      id: config?.id ?? 0,
      domain_id: domain.id,
      target_id: targetID,
      name: name.trim(),
      domain_override: domainOverride.trim().toLowerCase(),
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择 fnOS 实例')
      return
    }
    const payload = fnosConfigToDeployConfig(form)
    setSaving(true)
    try {
      if (config?.id) {
        await api.put(`/acme/deploy/configs/${config.id}`, payload)
      } else {
        await api.post(`/acme/domains/${domain.id}/deploy-configs`, payload)
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

  const selectableTargets = targets.filter((t) => t.enabled || t.id === targetID)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] overflow-y-auto p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{config ? '编辑 fnOS 部署配置' : '新增 fnOS 部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 部署到 fnOS：覆盖 ssls 时间戳目录下的 .crt/.key，
            并通过 psql 更新 trim_connect.cert（source='upload'）
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="fnos-deploy-name">配置名称</Label>
            <Input
              id="fnos-deploy-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="fnOS 部署"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fnos-deploy-target">fnOS 实例</Label>
            <Select<number>
              id="fnos-deploy-target"
              value={targetID}
              onChange={setTargetID}
              placeholder="（暂无启用的 fnOS 实例）"
              options={selectableTargets.map((t) => ({
                value: t.id,
                label: `${t.name} (${t.username}@${t.host}:${t.port || 22})`,
              }))}
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="fnos-deploy-override">fnOS 内目标域名（可选）</Label>
            <Input
              id="fnos-deploy-override"
              value={domainOverride}
              onChange={(e) => setDomainOverride(e.target.value)}
              placeholder={domain?.main_domain ?? '默认使用主域名'}
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <p className="text-[11.5px] text-muted-foreground">
              留空时按主域名匹配 ssls 目录与 cert.domain；用于在 fnOS 上写入与本地不一致的域名
            </p>
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="fnos-deploy-auto">签发/续期成功后自动部署</Label>
            <Switch id="fnos-deploy-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="fnos-deploy-enabled">启用</Label>
            <Switch id="fnos-deploy-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

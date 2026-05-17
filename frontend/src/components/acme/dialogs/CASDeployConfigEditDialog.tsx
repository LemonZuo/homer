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
import type { CASDeployConfig, CASTarget, Domain } from '../types'
import { casConfigToDeployConfig } from '../utils'

export function CASDeployConfigEditDialog({
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
  config: CASDeployConfig | null
  targets: CASTarget[]
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setAutoDeploy(config?.auto_deploy ?? false)
    setEnabled(config?.enabled ?? true)
  }, [open, config, targets])

  const save = async () => {
    if (!domain) return
    const form = {
      id: config?.id ?? 0,
      domain_id: domain.id,
      target_id: targetID,
      name: name.trim(),
      cert_id: config?.cert_id ?? 0,
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择阿里云 CAS 实例')
      return
    }
    const payload = casConfigToDeployConfig(form)
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
          <DialogTitle>{config ? '编辑阿里云 CAS 部署配置' : '新增阿里云 CAS 部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 上传到阿里云数字证书管理；CAS 每次都新增证书，
            不支持原地更新，cert_id 仅作展示
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="cas-deploy-name">配置名称</Label>
            <Input
              id="cas-deploy-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="CAS 上传"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="cas-deploy-target">CAS 实例</Label>
            <Select<number>
              id="cas-deploy-target"
              value={targetID}
              onChange={setTargetID}
              placeholder="（暂无启用的 CAS 实例）"
              options={selectableTargets.map((t) => ({
                value: t.id,
                label: `${t.name} (${t.access_key_id})`,
              }))}
            />
          </div>
          {config?.cert_id ? (
            <div className="grid gap-1.5">
              <Label>最近一次 cert_id</Label>
              <div className="rounded-md border bg-muted/30 px-2.5 py-1.5 font-mono text-[12px]">
                {config.cert_id}
              </div>
            </div>
          ) : null}
          <div className="flex items-center justify-between">
            <Label htmlFor="cas-deploy-auto">签发/续期成功后自动部署</Label>
            <Switch id="cas-deploy-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="cas-deploy-enabled">启用</Label>
            <Switch id="cas-deploy-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

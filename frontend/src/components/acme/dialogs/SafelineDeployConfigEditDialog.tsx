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
import type { Domain, SafelineDeployConfig, SafelineTarget } from '../types'
import { safelineConfigToDeployConfig } from '../utils'

export function SafelineDeployConfigEditDialog({
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
  config: SafelineDeployConfig | null
  targets: SafelineTarget[]
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [certID, setCertID] = useState('')
  const [certType, setCertType] = useState('2')
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setCertID(config?.cert_id ? String(config.cert_id) : '')
    setCertType(String(config?.cert_type || 2))
    setAutoDeploy(config?.auto_deploy ?? false)
    setEnabled(config?.enabled ?? true)
  }, [open, config, targets])

  const save = async () => {
    if (!domain) return
    const certIDNum = certID.trim() ? Number(certID) : 0
    const certTypeNum = Number(certType) || 2
    const form = {
      id: config?.id ?? 0,
      domain_id: domain.id,
      target_id: targetID,
      name: name.trim(),
      cert_id: certIDNum,
      cert_type: certTypeNum,
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择雷池实例')
      return
    }
    if (!Number.isInteger(certIDNum) || certIDNum < 0) {
      toast.error('雷池 cert_id 无效')
      return
    }
    if (!Number.isInteger(certTypeNum) || certTypeNum <= 0) {
      toast.error('雷池证书类型无效')
      return
    }
    const payload = safelineConfigToDeployConfig(form)
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
          <DialogTitle>{config ? '编辑雷池部署配置' : '新增雷池部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 上传到雷池证书管理；cert_id 留空表示首次新增，部署成功后会自动写回
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-deploy-name">配置名称</Label>
            <Input
              id="safeline-deploy-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="雷池证书"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="safeline-deploy-target">雷池实例</Label>
            <Select<number>
              id="safeline-deploy-target"
              value={targetID}
              onChange={setTargetID}
              placeholder="（暂无启用的雷池实例）"
              options={selectableTargets.map((t) => ({
                value: t.id,
                label: `${t.name} (${t.base_url})`,
              }))}
            />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-2">
            <div className="grid gap-1.5">
              <Label htmlFor="safeline-cert-id">雷池 cert_id</Label>
              <Input
                id="safeline-cert-id"
                value={certID}
                onChange={(e) => setCertID(e.target.value)}
                placeholder="留空则新增"
                autoComplete="off"
                data-lpignore="true"
                data-1p-ignore="true"
                className="font-mono text-[12px]"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="safeline-cert-type">证书类型</Label>
              <Select<string>
                id="safeline-cert-type"
                value={certType}
                onChange={setCertType}
                options={[
                  { value: '2', label: '手动上传证书（2）' },
                  { value: '1', label: '类型 1（兼容）' },
                ]}
              />
            </div>
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-deploy-auto">签发/续期成功后自动部署</Label>
            <Switch id="safeline-deploy-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="safeline-deploy-enabled">启用</Label>
            <Switch id="safeline-deploy-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

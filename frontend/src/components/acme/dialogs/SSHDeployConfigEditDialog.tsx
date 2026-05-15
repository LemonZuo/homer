import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../../api'
import { Button } from '../../ui/button'
import { Input } from '../../ui/input'
import { Textarea } from '../../ui/textarea'
import { Label } from '../../ui/label'
import { Switch } from '../../ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog'
import type { Domain, SSHDeployConfig, SSHTarget } from '../types'
import { sshConfigToDeployConfig } from '../utils'

export function SSHDeployConfigEditDialog({
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
  config: SSHDeployConfig | null
  targets: SSHTarget[]
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [targetID, setTargetID] = useState(0)
  const [fullchainPath, setFullchainPath] = useState('')
  const [keyPath, setKeyPath] = useState('')
  const [certPath, setCertPath] = useState('')
  const [chainPath, setChainPath] = useState('')
  const [deployCommand, setDeployCommand] = useState('')
  const [autoDeploy, setAutoDeploy] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const first = targets.find((t) => t.enabled)
    setName(config?.name ?? '')
    setTargetID(config?.target_id ?? first?.id ?? 0)
    setFullchainPath(config?.fullchain_path ?? '/etc/nginx/ssl/{domain}/fullchain.pem')
    setKeyPath(config?.key_path ?? '/etc/nginx/ssl/{domain}/key.pem')
    setCertPath(config?.cert_path ?? '')
    setChainPath(config?.chain_path ?? '')
    setDeployCommand(config?.deploy_command ?? 'nginx -t && systemctl reload nginx')
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
      fullchain_path: fullchainPath.trim(),
      key_path: keyPath.trim(),
      cert_path: certPath.trim(),
      chain_path: chainPath.trim(),
      deploy_command: deployCommand.trim(),
      auto_deploy: autoDeploy,
      enabled,
      created_at: config?.created_at ?? '',
      updated_at: config?.updated_at ?? '',
    }
    if (!form.target_id) {
      toast.error('请选择 SSH 机器')
      return
    }
    if (!form.key_path) {
      toast.error('远端 key.pem 路径必填')
      return
    }
    if (!form.fullchain_path && !form.cert_path) {
      toast.error('fullchain.pem 路径和 cert.pem 路径至少填写一个')
      return
    }
    const payload = sshConfigToDeployConfig(form)
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
          <DialogTitle>{config ? '编辑部署配置' : '新增部署配置'}</DialogTitle>
          <DialogDescription>
            {domain?.main_domain ?? '当前域名'} 的证书部署路径和部署命令，支持 {'{domain}'} 占位符
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3.5">
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-name">配置名称</Label>
            <Input
              id="deploy-config-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="nginx 主站"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-target">SSH 机器</Label>
            <select
              id="deploy-config-target"
              className="h-9 rounded-md border border-input bg-background px-3 text-[13px]"
              value={targetID ? String(targetID) : ''}
              onChange={(e) => setTargetID(Number(e.target.value))}
            >
              {selectableTargets.length === 0 && (
                <option value="">（暂无启用的 SSH 机器）</option>
              )}
              {selectableTargets.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.username}@{t.host}:{t.port || 22})
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-2">
            <Label>远端路径</Label>
            <Input
              value={fullchainPath}
              onChange={(e) => setFullchainPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/fullchain.pem"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={keyPath}
              onChange={(e) => setKeyPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/key.pem（必填）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={certPath}
              onChange={(e) => setCertPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/cert.pem（可选）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
            <Input
              value={chainPath}
              onChange={(e) => setChainPath(e.target.value)}
              placeholder="/etc/nginx/ssl/{domain}/chain.pem（可选）"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="deploy-config-command">部署命令（可选）</Label>
            <Textarea
              id="deploy-config-command"
              value={deployCommand}
              onChange={(e) => setDeployCommand(e.target.value)}
              placeholder="nginx -t && systemctl reload nginx"
              autoComplete="off"
              data-lpignore="true"
              data-1p-ignore="true"
              className="font-mono text-[12px]"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="deploy-config-auto">签发/续期成功后自动部署</Label>
            <Switch id="deploy-config-auto" checked={autoDeploy} onChange={(v) => setAutoDeploy(v)} />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="deploy-config-enabled">启用</Label>
            <Switch id="deploy-config-enabled" checked={enabled} onChange={(v) => setEnabled(v)} />
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

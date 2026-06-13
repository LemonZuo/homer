import { toast } from 'sonner'
import { api } from '../../api'
import { makeDeleteHandler, type DeployConfigKind } from './handlers'
import type { Domain } from './types'
import type { AcmeUiState } from './useAcmeUiState'
import { daysUntil } from './utils/date'

interface UseAcmeActionsArgs {
  ui: AcmeUiState
  reloadAll: () => Promise<void> | void
  reloadTasks: () => Promise<void> | void
  reloadCredentials: () => Promise<void> | void
  reloadDeployTargets: () => Promise<void> | void
  reloadSSHCredentials: () => Promise<void> | void
  reloadDeployConfigs: (domainID: number) => Promise<void> | void
}

interface APIErrorLike {
  response?: {
    data?: {
      error?: string
    }
  }
  message?: string
}

function apiError(e: unknown, fallback: string) {
  const err = e as APIErrorLike
  return err.response?.data?.error || err.message || fallback
}

export function useAcmeActions({
  ui,
  reloadAll,
  reloadTasks,
  reloadCredentials,
  reloadDeployTargets,
  reloadSSHCredentials,
  reloadDeployConfigs,
}: UseAcmeActionsArgs) {
  const reloadEntryConfigs = () =>
    ui.deploy.entryDomain ? reloadDeployConfigs(ui.deploy.entryDomain.id) : undefined

  const onDeleteSSHCredential = makeDeleteHandler({
    get: () => ui.sshCredentials.remove.pending,
    clear: ui.sshCredentials.remove.clear,
    url: (c) => `/acme/ssh-credentials/${c.id}`,
    reload: reloadSSHCredentials,
  })

  const onDeleteCredential = makeDeleteHandler({
    get: () => ui.credentials.remove.pending,
    clear: ui.credentials.remove.clear,
    url: (c) => `/acme/credentials/${c.id}`,
    reload: reloadCredentials,
  })

  const onDeleteAccount = makeDeleteHandler({
    get: () => ui.accounts.remove.pending,
    clear: ui.accounts.remove.clear,
    url: (a) => `/acme/accounts/${a.id}`,
    reload: reloadAll,
  })

  const onDeleteSSHTarget = makeDeleteHandler({
    get: () => ui.targets.ssh.remove.pending,
    clear: ui.targets.ssh.remove.clear,
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteSafelineTarget = makeDeleteHandler({
    get: () => ui.targets.safeline.remove.pending,
    clear: ui.targets.safeline.remove.clear,
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteCASTarget = makeDeleteHandler({
    get: () => ui.targets.cas.remove.pending,
    clear: ui.targets.cas.remove.clear,
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteFnOSTarget = makeDeleteHandler({
    get: () => ui.targets.fnos.remove.pending,
    clear: ui.targets.fnos.remove.clear,
    url: (t) => `/acme/deploy/targets/${t.id}`,
    reload: reloadDeployTargets,
  })

  const onDeleteSSHDeployConfig = makeDeleteHandler({
    get: () => ui.deploy.ssh.remove.pending,
    clear: ui.deploy.ssh.remove.clear,
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteSafelineDeployConfig = makeDeleteHandler({
    get: () => ui.deploy.safeline.remove.pending,
    clear: ui.deploy.safeline.remove.clear,
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteCASDeployConfig = makeDeleteHandler({
    get: () => ui.deploy.cas.remove.pending,
    clear: ui.deploy.cas.remove.clear,
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const onDeleteFnOSDeployConfig = makeDeleteHandler({
    get: () => ui.deploy.fnos.remove.pending,
    clear: ui.deploy.fnos.remove.clear,
    url: (cfg) => `/acme/deploy/configs/${cfg.id}`,
    reload: reloadEntryConfigs,
  })

  const startIssue = async (d: Domain) => {
    ui.domain.reissue.clear()
    ui.setBusy(`issue-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/issue`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交，任务 #${taskID}`)
      await reloadTasks()
      ui.log.setTaskID(taskID)
    } catch (e: unknown) {
      toast.error(apiError(e, '提交失败'))
    } finally {
      ui.setBusy(null)
    }
  }

  // requestIssue 是「重签」按钮的网关:有效期内的证书需要二次确认,其它情况(未签发/已过期/已吊销)直接走签发。
  const requestIssue = (d: Domain) => {
    const days = daysUntil(d.not_after)
    const hasValidCert = d.cert_status !== 'revoked' && days !== null && days > 0
    if (hasValidCert) {
      ui.domain.reissue.setPending(d)
      return
    }
    void startIssue(d)
  }

  const startRevoke = async (d: Domain) => {
    ui.domain.revoke.clear()
    ui.setBusy(`revoke-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/revoke`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交吊销，任务 #${taskID}`)
      await reloadTasks()
      ui.log.setTaskID(taskID)
    } catch (e: unknown) {
      toast.error(apiError(e, '提交吊销失败'))
    } finally {
      ui.setBusy(null)
    }
  }

  const downloadCert = (d: Domain) => {
    const a = document.createElement('a')
    a.href = `/api/acme/domains/${d.id}/cert/download`
    a.rel = 'noopener'
    a.click()
  }

  const openDeployConfigs = (d: Domain) => {
    ui.deploy.openConfigs(d)
    void reloadDeployConfigs(d.id)
  }

  const deployMeta: Record<
    DeployConfigKind,
    { word: string; reloadAfter: boolean; domain: () => Domain | null }
  > = {
    ssh: { word: ' SSH 部署', reloadAfter: false, domain: () => ui.deploy.ssh.domain },
    safeline: { word: '雷池部署', reloadAfter: true, domain: () => ui.deploy.safeline.domain },
    cas: { word: ' CAS 上传', reloadAfter: true, domain: () => ui.deploy.cas.domain },
    fnos: { word: ' fnOS 部署', reloadAfter: true, domain: () => ui.deploy.fnos.domain },
  }

  const startDeployConfig = async (kind: DeployConfigKind, cfg: { id: number }) => {
    const meta = deployMeta[kind]
    const dom = meta.domain()
    if (!dom) return
    ui.setBusy(`deploy-${kind}-config-${cfg.id}`)
    try {
      const { data } = await api.post(`/acme/deploy/configs/${cfg.id}/deploy`)
      const taskID = data?.data?.task_id as number
      toast.success(`已提交${meta.word}，任务 #${taskID}`)
      await reloadTasks()
      if (meta.reloadAfter) await reloadDeployConfigs(dom.id)
      ui.log.setTaskID(taskID)
    } catch (e: unknown) {
      toast.error(apiError(e, `提交${meta.word}失败`))
    } finally {
      ui.setBusy(null)
    }
  }

  const startDeployAllConfigs = async () => {
    const d = ui.deploy.entryDomain
    if (!d) return
    ui.setBusy(`deploy-domain-${d.id}`)
    try {
      const { data } = await api.post(`/acme/domains/${d.id}/deploy-configs/deploy`)
      const taskIDs = (data?.data?.task_ids ?? []) as number[]
      toast.success(`已提交 ${taskIDs.length} 个部署任务`)
      await reloadTasks()
      await reloadDeployConfigs(d.id)
    } catch (e: unknown) {
      toast.error(apiError(e, '提交一键部署失败'))
    } finally {
      ui.setBusy(null)
    }
  }

  const retryTask = async (taskID: number) => {
    ui.setBusy(`retry-${taskID}`)
    try {
      await api.post(`/acme/tasks/${taskID}/retry`)
      toast.success(`已重试任务 #${taskID}`)
      await reloadTasks()
      ui.log.setTaskID(taskID)
    } catch (e: unknown) {
      toast.error(apiError(e, '重试失败'))
    } finally {
      ui.setBusy(null)
    }
  }

  const deleteDomain = async () => {
    const d = ui.domain.remove.pending
    if (!d) return
    ui.domain.remove.clear()
    ui.setBusy(`del-${d.id}`)
    try {
      await api.delete(`/acme/domains/${d.id}`)
      toast.success('已删除')
      await reloadAll()
    } catch (e: unknown) {
      toast.error(apiError(e, '删除失败'))
    } finally {
      ui.setBusy(null)
    }
  }

  const testDeployTarget = async (id: number) => {
    try {
      await api.post(`/acme/deploy/targets/${id}/test`)
      toast.success('连接正常')
    } catch (e: unknown) {
      toast.error(apiError(e, '连接失败'))
    }
  }

  return {
    requestIssue,
    startIssue,
    startRevoke,
    downloadCert,
    openDeployConfigs,
    startDeployConfig,
    startDeployAllConfigs,
    retryTask,
    deleteDomain,
    testDeployTarget,
    onDeleteSSHCredential,
    onDeleteCredential,
    onDeleteAccount,
    onDeleteSSHTarget,
    onDeleteSafelineTarget,
    onDeleteCASTarget,
    onDeleteFnOSTarget,
    onDeleteSSHDeployConfig,
    onDeleteSafelineDeployConfig,
    onDeleteCASDeployConfig,
    onDeleteFnOSDeployConfig,
  }
}

export type AcmeActions = ReturnType<typeof useAcmeActions>

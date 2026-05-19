import { useState } from 'react'
import type {
  AcmeAccount,
  CASDeployConfig,
  CASTarget,
  Credential,
  Domain,
  FnOSDeployConfig,
  FnOSTarget,
  SSHCredential,
  SSHDeployConfig,
  SSHTarget,
  SafelineDeployConfig,
  SafelineTarget,
} from './types'

function useDrawerState() {
  const [open, setOpen] = useState(false)
  return {
    open,
    setOpen,
    show: () => setOpen(true),
    hide: () => setOpen(false),
  }
}

function useEditState<T>() {
  const [open, setOpen] = useState(false)
  const [target, setTarget] = useState<T | null>(null)
  return {
    open,
    setOpen,
    target,
    setTarget,
    add: () => {
      setTarget(null)
      setOpen(true)
    },
    edit: (next: T) => {
      setTarget(next)
      setOpen(true)
    },
  }
}

function usePendingState<T>() {
  const [pending, setPending] = useState<T | null>(null)
  return {
    pending,
    setPending,
    clear: () => setPending(null),
  }
}

function useDeployConfigUi<T>() {
  const [domain, setDomain] = useState<Domain | null>(null)
  return {
    domain,
    setDomain,
    edit: useEditState<T>(),
    remove: usePendingState<T>(),
  }
}

export function useAcmeUiState() {
  const [busy, setBusy] = useState<string | null>(null)
  const [logTaskID, setLogTaskID] = useState<number | null>(null)
  const [deployEntryDomain, setDeployEntryDomain] = useState<Domain | null>(null)

  const sshDeploy = useDeployConfigUi<SSHDeployConfig>()
  const safelineDeploy = useDeployConfigUi<SafelineDeployConfig>()
  const casDeploy = useDeployConfigUi<CASDeployConfig>()
  const fnosDeploy = useDeployConfigUi<FnOSDeployConfig>()

  const setDeployDomains = (domain: Domain | null) => {
    sshDeploy.setDomain(domain)
    safelineDeploy.setDomain(domain)
    casDeploy.setDomain(domain)
    fnosDeploy.setDomain(domain)
  }

  return {
    busy,
    setBusy,
    log: {
      taskID: logTaskID,
      setTaskID: setLogTaskID,
      clear: () => setLogTaskID(null),
    },
    domain: {
      edit: useEditState<Domain>(),
      remove: usePendingState<Domain>(),
      revoke: usePendingState<Domain>(),
    },
    credentials: {
      drawer: useDrawerState(),
      edit: useEditState<Credential>(),
      remove: usePendingState<Credential>(),
    },
    accounts: {
      drawer: useDrawerState(),
      edit: useEditState<AcmeAccount>(),
      remove: usePendingState<AcmeAccount>(),
    },
    targets: {
      entry: useDrawerState(),
      ssh: {
        edit: useEditState<SSHTarget>(),
        remove: usePendingState<SSHTarget>(),
      },
      safeline: {
        edit: useEditState<SafelineTarget>(),
        remove: usePendingState<SafelineTarget>(),
      },
      cas: {
        edit: useEditState<CASTarget>(),
        remove: usePendingState<CASTarget>(),
      },
      fnos: {
        edit: useEditState<FnOSTarget>(),
        remove: usePendingState<FnOSTarget>(),
      },
    },
    sshCredentials: {
      drawer: useDrawerState(),
      edit: useEditState<SSHCredential>(),
      remove: usePendingState<SSHCredential>(),
    },
    deploy: {
      entryDomain: deployEntryDomain,
      setEntryDomain: setDeployEntryDomain,
      openConfigs: (domain: Domain) => {
        setDeployEntryDomain(domain)
        setDeployDomains(domain)
      },
      closeConfigs: () => {
        setDeployEntryDomain(null)
        setDeployDomains(null)
      },
      ssh: sshDeploy,
      safeline: safelineDeploy,
      cas: casDeploy,
      fnos: fnosDeploy,
    },
  }
}

export type AcmeUiState = ReturnType<typeof useAcmeUiState>

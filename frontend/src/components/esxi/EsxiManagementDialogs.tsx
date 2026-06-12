import { EsxiCredentialEditDialog } from './EsxiCredentialEditDialog'
import { EsxiCredentialsDrawer } from './EsxiCredentialsDrawer'
import { EsxiHostEditDialog } from './EsxiHostEditDialog'
import { EsxiHostsDrawer } from './EsxiHostsDrawer'
import type { EsxiManagement } from './useEsxiManagement'

interface EsxiManagementDialogsProps {
  management: EsxiManagement
  onHostSaved: () => Promise<void> | void
}

export function EsxiManagementDialogs({ management, onHostSaved }: EsxiManagementDialogsProps) {
  return (
    <>
      <EsxiHostsDrawer
        open={management.hostsOpen}
        onOpenChange={management.setHostsOpen}
        hosts={management.hosts}
        onAdd={management.onAddHost}
        onEdit={management.onEditHost}
        onDelete={management.onDeleteHost}
        onTest={management.onTestHost}
        onManageCredentials={management.openCredsDrawer}
      />
      <EsxiHostEditDialog
        open={management.hostEditOpen}
        onOpenChange={management.setHostEditOpen}
        target={management.editingHost}
        hosts={management.hosts}
        credentials={management.credentials}
        onManageCredentials={management.openCredsDrawer}
        onSaved={() => {
          void management.loadHosts()
          void onHostSaved()
        }}
      />
      <EsxiCredentialsDrawer
        open={management.credsOpen}
        onOpenChange={management.setCredsOpen}
        credentials={management.credentials}
        onAdd={management.onAddCredential}
        onEdit={management.onEditCredential}
        onDelete={management.onDeleteCredential}
      />
      <EsxiCredentialEditDialog
        open={management.credEditOpen}
        onOpenChange={management.setCredEditOpen}
        target={management.editingCred}
        onSaved={management.loadCredentials}
      />
    </>
  )
}

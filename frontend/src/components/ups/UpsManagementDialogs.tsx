import { UpsCredentialEditDialog } from './UpsCredentialEditDialog'
import { UpsCredentialsDrawer } from './UpsCredentialsDrawer'
import { UpsHostEditDialog } from './UpsHostEditDialog'
import { UpsHostsDrawer } from './UpsHostsDrawer'
import type { UpsManagement } from './useUpsManagement'

interface UpsManagementDialogsProps {
  management: UpsManagement
  onHostSaved: () => Promise<void> | void
}

export function UpsManagementDialogs({ management, onHostSaved }: UpsManagementDialogsProps) {
  return (
    <>
      <UpsHostsDrawer
        open={management.hostsOpen}
        onOpenChange={management.setHostsOpen}
        hosts={management.hosts}
        onAdd={management.onAddHost}
        onEdit={management.onEditHost}
        onDelete={management.onDeleteHost}
        onTest={management.onTestHost}
        onManageCredentials={management.openCredsDrawer}
      />
      <UpsHostEditDialog
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
      <UpsCredentialsDrawer
        open={management.credsOpen}
        onOpenChange={management.setCredsOpen}
        credentials={management.credentials}
        onAdd={management.onAddCredential}
        onEdit={management.onEditCredential}
        onDelete={management.onDeleteCredential}
      />
      <UpsCredentialEditDialog
        open={management.credEditOpen}
        onOpenChange={management.setCredEditOpen}
        target={management.editingCred}
        onSaved={management.loadCredentials}
      />
    </>
  )
}

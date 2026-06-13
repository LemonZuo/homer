import { ConfirmDialog } from '../ConfirmDialog'
import { AccountEditDialog } from '../dialogs/AccountEditDialog'
import { CredentialEditDialog } from '../dialogs/CredentialEditDialog'
import { AccountsDrawer } from '../drawers/AccountsDrawer'
import { CredentialsDrawer } from '../drawers/CredentialsDrawer'
import type { AcmeActions } from '../useAcmeActions'
import type { AcmeUiState } from '../useAcmeUiState'
import type { AcmeAccount, Credential } from '../types'

interface AccountCredentialOverlaysProps {
  ui: AcmeUiState
  actions: AcmeActions
  accounts: AcmeAccount[]
  credentials: Credential[]
  onAccountsReload: () => Promise<void> | void
  onCredentialsReload: () => Promise<void> | void
}

export function AccountCredentialOverlays({
  ui,
  actions,
  accounts,
  credentials,
  onAccountsReload,
  onCredentialsReload,
}: AccountCredentialOverlaysProps) {
  return (
    <>
      <CredentialsDrawer
        open={ui.credentials.drawer.open}
        onOpenChange={ui.credentials.drawer.setOpen}
        credentials={credentials}
        onAdd={ui.credentials.edit.add}
        onEdit={ui.credentials.edit.edit}
        onDelete={ui.credentials.remove.setPending}
      />

      <CredentialEditDialog
        open={ui.credentials.edit.open}
        onOpenChange={ui.credentials.edit.setOpen}
        target={ui.credentials.edit.target}
        onSaved={onCredentialsReload}
      />

      <AccountsDrawer
        open={ui.accounts.drawer.open}
        onOpenChange={ui.accounts.drawer.setOpen}
        accounts={accounts}
        onAdd={ui.accounts.edit.add}
        onEdit={ui.accounts.edit.edit}
        onDelete={ui.accounts.remove.setPending}
      />

      <AccountEditDialog
        open={ui.accounts.edit.open}
        onOpenChange={ui.accounts.edit.setOpen}
        target={ui.accounts.edit.target}
        onSaved={onAccountsReload}
      />

      <ConfirmDialog
        open={!!ui.accounts.remove.pending}
        onClose={ui.accounts.remove.clear}
        onConfirm={actions.onDeleteAccount}
        title="删除 CA 账号"
      >
        即将删除{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.accounts.remove.pending?.name}
        </span>{' '}
        账号；已被域名引用的账号不能删除。
      </ConfirmDialog>

      <ConfirmDialog
        open={!!ui.credentials.remove.pending}
        onClose={ui.credentials.remove.clear}
        onConfirm={actions.onDeleteCredential}
        title="删除 DNS 凭证"
      >
        即将删除 provider{' '}
        <span className="font-mono font-medium text-foreground">
          {ui.credentials.remove.pending?.provider}
        </span>{' '}
        的凭证；已关联该 provider 的域名将无法继续签发，请确认。
      </ConfirmDialog>
    </>
  )
}

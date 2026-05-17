import { Edit3, Plus, Trash2 } from 'lucide-react'
import { Card } from '../../ui/card'
import { Button } from '../../ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '../../ui/drawer'
import type { Credential } from '../types'
import { safeParseEnvs } from '../utils'

export function CredentialsDrawer({
  open,
  onOpenChange,
  credentials,
  onAdd,
  onEdit,
  onDelete,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  credentials: Credential[]
  onAdd: () => void
  onEdit: (c: Credential) => void
  onDelete: (c: Credential) => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>DNS 凭证</DrawerTitle>
          <DrawerDescription>
            按 lego provider key 维护环境变量；保存后立刻可用于签发
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex-1 space-y-2 overflow-auto px-4 pb-4">
          <div className="flex justify-end [&>button]:h-10 [&>button]:w-full sm:[&>button]:h-8 sm:[&>button]:w-auto">
            <Button size="sm" onClick={onAdd}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              添加凭证
            </Button>
          </div>
          {credentials.length === 0 ? (
            <p className="py-8 text-center text-[12.5px] text-muted-foreground">
              还没有凭证，点击「添加凭证」开始
            </p>
          ) : (
            credentials.map((c) => (
              <Card key={c.id} className="px-4 py-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-[13px] font-medium">{c.provider}</span>
                      <span
                        className={
                          c.ref_count
                            ? 'shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[11px] font-medium text-emerald-600 dark:text-emerald-400'
                            : 'shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground'
                        }
                      >
                        {c.ref_count ? `${c.ref_count} 个域名` : '未使用'}
                      </span>
                    </div>
                    <div
                      className="mt-0.5 truncate font-mono text-[11.5px] text-muted-foreground"
                      title={c.envs_json}
                    >
                      {Object.keys(safeParseEnvs(c.envs_json)).join(', ') || '（空）'}
                    </div>
                  </div>
                  <div className="flex gap-2 sm:contents">
                    <Button size="sm" variant="outline" className="flex-1 sm:flex-none" onClick={() => onEdit(c)}>
                      <Edit3 className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1 hover:text-destructive sm:flex-none"
                      onClick={() => onDelete(c)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </Card>
            ))
          )}
        </div>
      </DrawerContent>
    </Drawer>
  )
}

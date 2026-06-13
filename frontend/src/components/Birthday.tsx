import { useMemo, useState } from 'react'
import { Plus } from 'lucide-react'
import { getColorSet } from '../colors'
import { Button } from './ui/button'
import { BirthdayCard } from './birthday/BirthdayCard'
import { BirthdayEditDialog } from './birthday/BirthdayEditDialog'
import type { Birthday as BirthdayRecord } from './birthday/types'
import { useBirthdayRecords } from './birthday/useBirthdayRecords'
import { ReminderDeleteDialog } from './reminders/ReminderDeleteDialog'
import { ReminderEmptyState } from './reminders/ReminderEmptyState'
import { ReminderToolbar } from './reminders/ReminderToolbar'

export default function Birthday() {
  const cs = getColorSet('orange')
  const { records, loading, notifying, save, remove, toggle, notify } = useBirthdayRecords()

  const [kw, setKw] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<BirthdayRecord | null>(null)
  const [pendingDelete, setPendingDelete] = useState<BirthdayRecord | null>(null)

  const filtered = useMemo(() => {
    if (!kw) return records
    const s = kw.toLowerCase()
    return records.filter((r) =>
      Object.values(r).some(
        (v) => v !== null && v !== undefined && String(v).toLowerCase().includes(s),
      ),
    )
  }, [records, kw])

  const openAdd = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (record: BirthdayRecord) => {
    setEditing(record)
    setModalOpen(true)
  }

  const confirmDelete = () => {
    const target = pendingDelete
    if (!target) return
    setPendingDelete(null)
    void remove(target)
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <ReminderToolbar
        title="生日提醒"
        count={filtered.length}
        keyword={kw}
        onKeywordChange={setKw}
        onAdd={openAdd}
        dotClassName={cs.dot}
      />

      {loading || filtered.length === 0 ? (
        <ReminderEmptyState loading={loading} hasKeyword={Boolean(kw)} />
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((record) => (
            <BirthdayCard
              key={record.id}
              record={record}
              accent={cs}
              notifying={notifying === record.id}
              onNotify={(item) => void notify(item)}
              onEdit={openEdit}
              onDelete={setPendingDelete}
              onToggle={(item, enabled) => void toggle(item, enabled)}
            />
          ))}
        </div>
      )}

      <Button
        size="icon"
        onClick={openAdd}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增"
      >
        <Plus className="h-5 w-5" />
      </Button>

      <BirthdayEditDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        target={editing}
        onSave={save}
      />

      <ReminderDeleteDialog
        open={!!pendingDelete}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null)
        }}
        onConfirm={confirmDelete}
      />
    </div>
  )
}

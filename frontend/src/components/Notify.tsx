import { useState } from 'react'
import { Loader2, Plus, RefreshCw } from 'lucide-react'
import { getColorSet } from '../colors'
import { Button } from './ui/button'
import { cn } from '../lib/utils'
import { BindingSection } from './notify/BindingSection'
import { ChannelDeleteDialog } from './notify/ChannelDeleteDialog'
import { ChannelDialog } from './notify/ChannelDialog'
import { ChannelList } from './notify/ChannelList'
import type { Channel } from './notify/types'
import { useNotifyData } from './notify/useNotifyData'

export default function Notify() {
  const cs = getColorSet('indigo')
  const {
    modules,
    types,
    channels,
    bindings,
    testingID,
    loading,
    loadChannels,
    reloadAll,
    deleteChannel,
    testChannel,
    toggleBinding,
  } = useNotifyData()

  const [editTarget, setEditTarget] = useState<Channel | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [delTarget, setDelTarget] = useState<Channel | null>(null)

  const openAdd = () => {
    setEditTarget(null)
    setEditOpen(true)
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="hidden sm:block">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full', cs.dot)} />
            <h1 className="text-[28px] font-bold leading-none tracking-tight">通知渠道</h1>
          </div>
          <p className="mt-2 text-[12.5px] text-muted-foreground">
            统一管理通道与每个模块的绑定，改动即时生效
          </p>
        </div>
        <div className="grid grid-cols-1 gap-2 sm:flex sm:shrink-0">
          <Button
            variant="outline"
            size="sm"
            className="h-10 w-full sm:h-8 sm:w-auto"
            onClick={reloadAll}
            disabled={loading}
          >
            {loading ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            )}
            刷新
          </Button>
          <Button
            size="sm"
            className="hidden sm:inline-flex"
            onClick={openAdd}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            新增通道
          </Button>
        </div>
      </div>

      <Button
        size="icon"
        onClick={openAdd}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+6rem)] right-5 z-30 h-12 w-12 rounded-full shadow-lg active:scale-95 sm:hidden"
        aria-label="新增通道"
      >
        <Plus className="h-5 w-5" />
      </Button>

      <ChannelList
        channels={channels}
        types={types}
        testingID={testingID}
        onTest={(id) => void testChannel(id)}
        onEdit={(channel) => {
          setEditTarget(channel)
          setEditOpen(true)
        }}
        onDelete={setDelTarget}
      />

      <BindingSection
        modules={modules}
        channels={channels}
        bindings={bindings}
        activeClassName={cs.picker}
        onToggle={(module, channelID) => void toggleBinding(module, channelID)}
      />

      <ChannelDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        target={editTarget}
        types={types}
        onSaved={loadChannels}
      />

      <ChannelDeleteDialog
        target={delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={() => {
          if (delTarget) void deleteChannel(delTarget)
          setDelTarget(null)
        }}
      />
    </div>
  )
}

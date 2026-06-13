import { useState } from 'react'
import { getColorSet } from '../colors'
import { Card } from './ui/card'
import { DevicePanel } from './sms/DevicePanel'
import { ForwarderManageDrawer } from './sms/ForwarderManageDrawer'
import { SendSmsPanel } from './sms/SendSmsPanel'
import { SmsPageHeader } from './sms/SmsPageHeader'
import { SmsQueryPanel } from './sms/SmsQueryPanel'
import { useSmsDeviceConfig } from './sms/useSmsDeviceConfig'
import { useSmsForwarders } from './sms/useSmsForwarders'
import { useSmsMessages } from './sms/useSmsMessages'
import { useSmsSender } from './sms/useSmsSender'

export default function Sms() {
  const accent = getColorSet('teal')
  const [manageOpen, setManageOpen] = useState(false)
  const {
    forwarders,
    enabledList,
    selectedID,
    setSelectedID,
    loadForwarders,
  } = useSmsForwarders()
  const { config, configLoading, fetchConfig } = useSmsDeviceConfig(selectedID)
  const { sendForm, setSendForm, sending, send } = useSmsSender(selectedID)
  const {
    queryType,
    setQueryType,
    keyword,
    setKeyword,
    pageNum,
    setPageNum,
    pageSize,
    setPageSize,
    queryLoading,
    queryRaw,
    runQuery,
    list,
    hasNext,
  } = useSmsMessages(selectedID)

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-4 sm:px-8 sm:pt-10">
      <SmsPageHeader
        accent={accent}
        selectedID={selectedID}
        enabledList={enabledList}
        onSelect={setSelectedID}
        onManage={() => setManageOpen(true)}
      />

      {forwarders.length === 0 && (
        <Card className="mb-4 border-amber-500/40 bg-amber-500/5 px-4 py-3 text-[12.5px] text-amber-700 dark:text-amber-300">
          还没有短信转发器，点击右上角「管理」添加一个。
        </Card>
      )}

      <DevicePanel
        config={config}
        loading={configLoading}
        disabled={selectedID == null}
        onRefresh={() => void fetchConfig()}
      />

      <SendSmsPanel
        form={sendForm}
        onFormChange={setSendForm}
        sending={sending}
        disabled={selectedID == null}
        onSend={() => void send()}
      />

      <SmsQueryPanel
        queryType={queryType}
        onQueryTypeChange={setQueryType}
        keyword={keyword}
        onKeywordChange={setKeyword}
        pageNum={pageNum}
        onPageNumChange={setPageNum}
        pageSize={pageSize}
        onPageSizeChange={setPageSize}
        queryLoading={queryLoading}
        queryRaw={queryRaw}
        list={list}
        hasNext={hasNext}
        disabled={selectedID == null}
        onQuery={(overrides) => void runQuery(overrides)}
      />

      <ForwarderManageDrawer
        open={manageOpen}
        onOpenChange={setManageOpen}
        forwarders={forwarders}
        onReload={loadForwarders}
      />
    </div>
  )
}

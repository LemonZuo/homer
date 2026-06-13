import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import type { SmsSendForm } from './types'

const initialSendForm: SmsSendForm = {
  simSlot: 1,
  phones: '',
  content: '',
}

export function useSmsSender(selectedID: number | null) {
  const [sendForm, setSendForm] = useState<SmsSendForm>(initialSendForm)
  const [sending, setSending] = useState(false)

  const send = useCallback(async () => {
    if (selectedID == null) {
      toast.error('请先选择短信转发器')
      return
    }
    const phones = sendForm.phones.trim()
    const content = sendForm.content.trim()
    if (!phones) return toast.error('请填写手机号')
    if (!content) return toast.error('请填写短信内容')
    setSending(true)
    try {
      const { data } = await api.post('/sms/send', {
        target_id: selectedID,
        sim_slot: sendForm.simSlot,
        phone_numbers: phones,
        msg_content: content,
      })
      const code = data?.code ?? data?.errcode
      const msg = data?.msg ?? data?.message ?? data?.errmsg
      if (code !== undefined && code !== 200 && code !== 0) {
        toast.error(msg || '发送失败')
      } else {
        toast.success(msg || '已下发')
        setSendForm((f) => ({ ...f, content: '' }))
      }
    } catch (e: any) {
      toast.error(e?.response?.data?.error || e?.message || '发送失败')
    } finally {
      setSending(false)
    }
  }, [selectedID, sendForm])

  return {
    sendForm,
    setSendForm,
    sending,
    send,
  }
}

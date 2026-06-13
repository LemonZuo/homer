import { Loader2, Send } from 'lucide-react'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Textarea } from '../ui/textarea'
import type { SimSlot, SmsSendForm } from './types'

interface SendSmsPanelProps {
  form: SmsSendForm
  onFormChange: React.Dispatch<React.SetStateAction<SmsSendForm>>
  sending: boolean
  disabled: boolean
  onSend: () => void
}

export function SendSmsPanel({
  form,
  onFormChange,
  sending,
  disabled,
  onSend,
}: SendSmsPanelProps) {
  return (
    <Card className="mb-4 px-4 py-4">
      <div className="mb-3 flex items-center gap-2">
        <Send className="h-4 w-4 text-muted-foreground" />
        <div className="text-[13px] font-medium">发送短信</div>
      </div>
      <div className="space-y-3">
        <div>
          <Label className="mb-1.5 block text-[12px]">SIM 卡槽</Label>
          <div className="flex gap-2">
            {[1, 2].map((slot) => (
              <Button
                key={slot}
                type="button"
                size="sm"
                variant={form.simSlot === slot ? 'default' : 'outline'}
                className="flex-1 sm:flex-none"
                onClick={() => onFormChange((f) => ({ ...f, simSlot: slot as SimSlot }))}
              >
                SIM {slot}
              </Button>
            ))}
          </div>
        </div>
        <div>
          <Label className="mb-1.5 block text-[12px]">收件人</Label>
          <Input
            placeholder="多个手机号用半角分号 ; 分隔"
            value={form.phones}
            onChange={(e) => onFormChange((f) => ({ ...f, phones: e.target.value }))}
          />
        </div>
        <div>
          <Label className="mb-1.5 block text-[12px]">
            内容
            <span className="ml-2 text-muted-foreground">{form.content.length} / 390 字</span>
          </Label>
          <Textarea
            rows={4}
            placeholder="70 字算一条，超出每 64 字递增一条，最多 6 条 / 390 字"
            maxLength={390}
            value={form.content}
            onChange={(e) => onFormChange((f) => ({ ...f, content: e.target.value }))}
          />
        </div>
        <div className="flex justify-end">
          <Button onClick={onSend} disabled={sending || disabled}>
            {sending ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Send className="mr-1.5 h-3.5 w-3.5" />
            )}
            发送
          </Button>
        </div>
      </div>
    </Card>
  )
}

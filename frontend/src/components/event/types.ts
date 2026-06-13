export interface EventItem {
  id: number
  title: string
  event_date: string
  lead_days: number
  remark: string
  enabled: boolean
}

export interface EventForm {
  title: string
  event_date: string
  lead_days: number | null
  remark: string
  enabled: boolean
}

export const blankEventForm: EventForm = {
  title: '',
  event_date: '',
  lead_days: 5,
  remark: '',
  enabled: true,
}

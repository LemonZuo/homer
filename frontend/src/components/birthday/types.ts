export interface Birthday {
  id: number
  name: string
  birthday: string
  chinese_birthday: string
  zodiac: string
  enabled: boolean
}

export interface BirthdayForm {
  name: string
  birthday: string
  enabled: boolean
}

export const blankBirthdayForm: BirthdayForm = { name: '', birthday: '', enabled: true }

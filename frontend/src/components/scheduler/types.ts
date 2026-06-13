export interface Run {
  start: string
  end: string
  ok: boolean
  skipped?: boolean
  err?: string
  trigger: string
}

export interface Job {
  name: string
  spec: string
  manual_only: boolean
  next?: string
  running: boolean
  last?: Run
  history: Run[]
}

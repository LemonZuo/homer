export interface Channel {
  id: number
  name: string
  type: string
  config_json: string
  enabled: boolean
  ref_count: number
}

export interface ModuleMeta {
  key: string
  label: string
}

export interface TypeMeta {
  type: string
  label: string
  fields: string[]
}

export type ModuleBindings = Record<string, number[]>

import { createContext, useContext } from 'react'

export interface ModuleSetting {
  key: string
  label: string
  group: string
  description: string
  impact: string
  hint?: string
  jobs: string[]
  enabled: boolean
  version: number
  updated_at: string
  active: number
}

export interface ModuleSnapshot {
  instance: string
  revision: number
  modules: ModuleSetting[]
}

export interface ModulesState {
  snapshot: ModuleSnapshot | null
  loading: boolean
  error: string
  connected: boolean
  pending: string[]
  reload: () => Promise<void>
  update: (module: ModuleSetting, enabled: boolean) => Promise<void>
  isEnabled: (key: string) => boolean
}

export const ModulesContext = createContext<ModulesState | null>(null)
export const CORE_PAGES = ['scheduler', 'notify', 'modules']

export function useModules() {
  const state = useContext(ModulesContext)
  if (!state) throw new Error('ModulesProvider is required')
  return state
}

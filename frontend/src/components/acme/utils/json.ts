export function safeParseJSON(s: string): Record<string, unknown> {
  try {
    const v = JSON.parse(s || '{}') as unknown
    return typeof v === 'object' && v !== null ? (v as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

export function safeParseEnvs(s: string): Record<string, string> {
  try {
    const v = JSON.parse(s || '{}') as unknown
    return typeof v === 'object' && v !== null ? (v as Record<string, string>) : {}
  } catch {
    return {}
  }
}

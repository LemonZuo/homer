export type FieldType = 'text' | 'password' | 'textarea'

export interface Field {
  key: string
  label: string
  type?: FieldType
  placeholder?: string
}

export interface TableDef {
  key: string
  label: string
  path: string // API 路径，不含 /api/
  icon: string // emoji 占位
  // 主题色（tailwind 色名 + 用于 dot 与高亮）
  color: string
  // 卡片标题字段（取第一个非空）
  titleKeys: string[]
  // 卡片副标题字段
  subtitleKeys: string[]
  // 表单字段
  fields: Field[]
}

// 后续业务模块在此添加（证书、域名、提醒等）
export const tables: TableDef[] = []

export function getTable(key: string): TableDef | undefined {
  return tables.find((t) => t.key === key)
}

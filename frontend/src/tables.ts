export type FieldType = 'text' | 'password' | 'textarea' | 'date' | 'switch'

export interface Field {
  key: string
  label: string
  type?: FieldType
  placeholder?: string
  // 只读字段：表单中禁用输入；多用于后端自动计算的派生值
  readonly?: boolean
}

// 卡片上的自定义动作按钮：POST /<table.path>/<id>/<path>，无 body。
// 后端可返回 {message: "..."}，前端做 toast。
export type RecordActionIcon = 'bell' | 'send' | 'refresh' | 'download'

export interface RecordAction {
  key: string
  label: string
  icon: RecordActionIcon
  path: string
  // 成功后的默认提示；如后端返回了 message 字段会优先用后端的
  successToast?: string
  // 触发前是否需要二次确认
  confirm?: string
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
  // 卡片是否禁用复制按钮（敏感数据或纯展示场景）
  noCopy?: boolean
  // 卡片右上角自定义动作
  recordActions?: RecordAction[]
}

export const tables: TableDef[] = [
  {
    key: 'birthday',
    label: '生日提醒',
    path: 'birthday',
    icon: '',
    color: 'orange',
    titleKeys: ['name'],
    subtitleKeys: ['birthday', 'zodiac'],
    noCopy: true,
    recordActions: [
      {
        key: 'notify',
        label: '发送提醒',
        icon: 'bell',
        path: 'notify',
        successToast: '已推送企业微信',
      },
    ],
    fields: [
      { key: 'name', label: '姓名', placeholder: '张三' },
      { key: 'birthday', label: '公历生日', type: 'date' },
      { key: 'chinese_birthday', label: '农历生日', readonly: true, placeholder: '保存后自动计算' },
      { key: 'zodiac', label: '生肖', readonly: true, placeholder: '保存后自动计算' },
      { key: 'is_remind', label: '启用提醒', type: 'switch' },
    ],
  },
]

export function getTable(key: string): TableDef | undefined {
  return tables.find((t) => t.key === key)
}

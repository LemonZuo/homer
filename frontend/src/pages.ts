// 所有功能页面统一登记在此（不再有声明式 CRUD 表，每个页面自包含）。
export interface PageDef {
  key: string
  label: string
  color: string
}

export const pages: PageDef[] = [
  { key: 'ups', label: 'UPS 状态', color: 'teal' },
  { key: 'esxi', label: 'ESXi 状态', color: 'esxi' },
  { key: 'acme', label: '证书签发', color: 'emerald' },
  { key: 'certstore', label: 'CAS证书', color: 'violet' },
  { key: 'cdnops', label: 'CDN域名', color: 'sky' },
  { key: 'birthday', label: '生日提醒', color: 'orange' },
  { key: 'event', label: '事项提醒', color: 'blue' },
  { key: 'sms', label: '短信转发', color: 'teal' },
  { key: 'scheduler', label: '任务调度', color: 'blue' },
  { key: 'notify', label: '通知渠道', color: 'indigo' },
]

export function getPage(key: string | undefined): PageDef | undefined {
  return pages.find((p) => p.key === key)
}

// 统一导航项 (/p/:key)，供侧栏 / 命令面板 / 移动端切换共用。
export interface NavItem {
  key: string
  label: string
  color: string
  to: string
}

export const navItems: NavItem[] = pages.map((p) => ({
  key: p.key,
  label: p.label,
  color: p.color,
  to: `/p/${p.key}`,
}))

export function findNavByPath(pathname: string): NavItem | undefined {
  const m = pathname.match(/^\/p\/([^/]+)/)
  if (!m) return undefined
  return navItems.find((n) => n.to === '/p/' + m[1])
}

import { tables } from './tables'

// 自定义页面：非标准 CRUD（云资源只读视图 + 操作按钮），不走 tables.ts。
// 后续 CAS / ACME 也在此登记。
export interface PageDef {
  key: string
  label: string
  color: string
}

export const pages: PageDef[] = [
  { key: 'cdn', label: '加速域名', color: 'sky' },
  { key: 'cas', label: '证书管理', color: 'violet' },
]

export function getPage(key: string | undefined): PageDef | undefined {
  return pages.find((p) => p.key === key)
}

// 统一导航项：CRUD 表 (/t/:key) 与自定义页面 (/p/:key) 合并，供侧栏 / 命令面板 / 移动端切换共用。
export interface NavItem {
  key: string
  label: string
  color: string
  to: string
}

export const navItems: NavItem[] = [
  ...tables.map((t) => ({ key: t.key, label: t.label, color: t.color, to: `/t/${t.key}` })),
  ...pages.map((p) => ({ key: p.key, label: p.label, color: p.color, to: `/p/${p.key}` })),
]

export function findNavByPath(pathname: string): NavItem | undefined {
  const m = pathname.match(/^\/(t|p)\/([^/]+)/)
  if (!m) return undefined
  const prefix = m[1] === 't' ? '/t/' : '/p/'
  return navItems.find((n) => n.to === prefix + m[2])
}

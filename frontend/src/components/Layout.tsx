import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { tables } from '../tables'
import { getColorSet } from '../colors'
import { Check, Command as CmdIcon, LayoutGrid } from 'lucide-react'
import { cn } from '../lib/utils'
import { Kbd } from './ui/kbd'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from './ui/drawer'
import CommandPalette from './CommandPalette'
import { Logo } from './Logo'
import { api } from '../api'

function useAppVersion() {
  const [version, setVersion] = useState('')
  useEffect(() => {
    let alive = true
    api
      .get<{ version: string }>('/version')
      .then((res) => {
        if (!alive) return
        const v = res.data?.version ?? ''
        setVersion(/^v\d/.test(v) ? v : v ? `v${v}` : '')
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])
  return version
}

function useCurrentTable() {
  const loc = useLocation()
  const m = loc.pathname.match(/^\/t\/([^/]+)/)
  const key = m?.[1]
  return tables.find((t) => t.key === key) ?? tables[0]
}

function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() =>
    typeof window === 'undefined' ? false : window.matchMedia(query).matches,
  )

  useEffect(() => {
    const media = window.matchMedia(query)
    const onChange = () => setMatches(media.matches)
    onChange()
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [query])

  return matches
}

export default function Layout() {
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const widePicker = useMediaQuery('(min-width: 430px)')
  const current = useCurrentTable()
  const displayVersion = useAppVersion()
  const navigate = useNavigate()
  const currentCs = getColorSet(current.color)
  const pickerColumns = widePicker ? 3 : 2
  const pickerTail = tables.length % pickerColumns

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="flex min-h-screen flex-col sm:flex-row">
      {/* 桌面端侧栏 */}
      <aside className="sticky top-0 hidden h-screen w-56 shrink-0 flex-col border-r border-border bg-card/40 px-3 py-6 backdrop-blur-sm sm:flex">
        <div className="mb-6 flex items-center gap-2.5 px-3">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
            <Logo className="h-5 w-5" />
          </span>
          <div className="min-w-0 leading-tight">
            <div className="truncate text-[13px] font-semibold tracking-tight">
              Homer
            </div>
            <div className="truncate text-[11px] text-muted-foreground">
              私人小管家
            </div>
          </div>
        </div>

        <nav className="flex-1 space-y-0.5 overflow-y-auto">
          {tables.map((t) => {
            const cs = getColorSet(t.color)
            return (
              <NavLink
                key={t.key}
                to={`/t/${t.key}`}
                className={({ isActive }) =>
                  cn(
                    'group flex items-center gap-2.5 rounded-md border px-3 py-1.5 text-[13px] font-medium transition-[background-color,border-color,color,box-shadow]',
                    isActive
                      ? cn(cs.picker, 'shadow-sm')
                      : 'border-transparent text-muted-foreground hover:bg-accent/60 hover:text-foreground',
                  )
                }
              >
                <span className={cn('h-1.5 w-1.5 rounded-full transition-transform group-hover:scale-125', cs.dot)} />
                <span>{t.label}</span>
              </NavLink>
            )
          })}
        </nav>

        <button
          onClick={() => setPaletteOpen(true)}
          className="mt-3 flex items-center justify-between gap-2 rounded-md border border-border bg-background/50 px-3 py-1.5 text-[12px] text-muted-foreground transition hover:border-ring/50 hover:bg-accent hover:text-foreground"
        >
          <span className="flex items-center gap-1.5">
            <CmdIcon className="h-3 w-3" />
            快速切换
          </span>
          <Kbd>⌘K</Kbd>
        </button>

        {displayVersion && (
          <div className="mt-3 px-3 text-[10.5px] font-mono tabular-nums text-muted-foreground/60">
            {displayVersion}
          </div>
        )}
      </aside>

      {/* 移动端顶栏 */}
      <header className="sticky top-0 z-20 flex items-center justify-between gap-2 border-b border-border bg-background/85 px-4 py-3 backdrop-blur-md sm:hidden">
        <div className="flex min-w-0 items-center gap-2">
          <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', currentCs.dot)} />
          <span className="truncate text-[15px] font-semibold tracking-tight">
            {current.label}
          </span>
        </div>
        {displayVersion ? (
          <span className="font-mono text-[10.5px] tabular-nums text-muted-foreground/70">
            {displayVersion}
          </span>
        ) : (
          <span />
        )}
      </header>

      {/* 移动端切换表 FAB：与右下角新增 FAB 对称 */}
      <button
        type="button"
        onClick={() => setPickerOpen(true)}
        aria-label="切换表"
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+10.5rem)] right-5 z-30 flex h-12 w-12 items-center justify-center rounded-full border border-border bg-card text-foreground shadow-lg transition-transform active:scale-95 sm:hidden"
      >
        <LayoutGrid className="h-5 w-5" />
        <span
          className={cn(
            'absolute right-1.5 top-1.5 h-2.5 w-2.5 rounded-full ring-2 ring-card',
            currentCs.dot,
          )}
        />
      </button>

      {/* 移动端表切换：底部弹起 Drawer */}
      <Drawer open={pickerOpen} onOpenChange={setPickerOpen}>
        <DrawerContent className="sm:hidden overflow-hidden">
          <DrawerHeader className="pb-2">
            <DrawerTitle>切换表</DrawerTitle>
          </DrawerHeader>
          <div className="grid min-h-0 grid-cols-6 gap-2 overflow-y-auto px-2 pb-[max(1rem,env(safe-area-inset-bottom))]">
            {tables.map((t, index) => {
              const cs = getColorSet(t.color)
              const active = t.key === current.key
              const remaining = tables.length - index
              const singleTail = pickerTail === 1 && remaining === 1
              const pairTail = pickerColumns === 3 && pickerTail === 2 && remaining <= 2
              const centered = singleTail || pairTail
              const spanClass = singleTail
                ? 'col-span-6'
                : pairTail
                  ? 'col-span-3'
                  : pickerColumns === 3
                    ? 'col-span-2'
                    : 'col-span-3'
              return (
                <button
                  key={t.key}
                  onClick={() => {
                    navigate(`/t/${t.key}`)
                    setPickerOpen(false)
                  }}
                  className={cn(
                    'flex h-11 w-full min-w-0 items-center gap-2 rounded-lg border px-3 text-left text-[14px] transition-[background-color,border-color,box-shadow,transform] active:scale-[0.98]',
                    spanClass,
                    centered && 'justify-center',
                    active
                      ? cn(cs.picker, 'font-semibold shadow-sm')
                      : 'border-transparent active:bg-accent/60',
                  )}
                >
                  <span className={cn('h-2 w-2 shrink-0 rounded-full', cs.dot)} />
                  <span className={cn('min-w-0 truncate', centered ? 'flex-none' : 'flex-1')}>
                    {t.label}
                  </span>
                  {active && (
                    <span className={cn('flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-white shadow-sm', cs.dot)}>
                      <Check className="h-3.5 w-3.5" />
                    </span>
                  )}
                </button>
              )
            })}
          </div>
        </DrawerContent>
      </Drawer>

      <main className="flex-1 overflow-x-hidden">
        <Outlet />
      </main>

      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        onPick={(key) => {
          navigate(`/t/${key}`)
          setPaletteOpen(false)
        }}
        currentKey={current.key}
      />
    </div>
  )
}

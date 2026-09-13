import { Suspense, lazy, type ReactElement } from 'react'
import { Routes, Route, Navigate, useParams } from 'react-router-dom'
import Layout from './components/Layout'
import { pages, getPage } from './pages'
import { CORE_PAGES, useModules } from './lib/modules'

const Birthday = lazy(() => import('./components/Birthday'))
const Event = lazy(() => import('./components/Event'))
const CdnOps = lazy(() => import('./components/CdnOps'))
const CertStore = lazy(() => import('./components/CertStore'))
const Acme = lazy(() => import('./components/Acme'))
const Sms = lazy(() => import('./components/Sms'))
const Scheduler = lazy(() => import('./components/Scheduler'))
const Notify = lazy(() => import('./components/Notify'))
const Ups = lazy(() => import('./components/Ups'))
const Esxi = lazy(() => import('./components/Esxi'))
const Modules = lazy(() => import('./components/Modules'))

function Empty() {
  return (
    <div className="mx-auto max-w-md px-4 py-20 text-center text-muted-foreground">
      <p className="text-[15px] font-medium">还没有任何模块</p>
      <p className="mt-2 text-[13px]">在 <code>src/pages.ts</code> 添加业务模块。</p>
    </div>
  )
}

function PageFallback() {
  return (
    <div className="mx-auto max-w-md px-4 py-16 text-center text-[13px] text-muted-foreground">
      加载中...
    </div>
  )
}

const PAGE_COMPONENTS: Record<string, () => ReactElement> = {
  birthday: () => <Birthday />,
  event: () => <Event />,
  cdnops: () => <CdnOps />,
  certstore: () => <CertStore />,
  acme: () => <Acme />,
  sms: () => <Sms />,
  scheduler: () => <Scheduler />,
  notify: () => <Notify />,
  ups: () => <Ups />,
  esxi: () => <Esxi />,
  modules: () => <Modules />,
}

function CustomPage() {
  const { pageKey } = useParams()
  const { snapshot, loading, isEnabled } = useModules()
  const def = getPage(pageKey)
  const render = def ? PAGE_COMPONENTS[def.key] : undefined
  if (!def || !render) return <Empty />
  if (!CORE_PAGES.includes(def.key)) {
    if (!snapshot && loading) return <PageFallback />
    if (!isEnabled(def.key)) return <Navigate to="/p/modules" replace />
  }
  return <Suspense fallback={<PageFallback />}>{render()}</Suspense>
}

export default function App() {
  const { snapshot, loading, isEnabled } = useModules()
  const first = pages.find((page) => !CORE_PAGES.includes(page.key) && isEnabled(page.key))?.key ?? 'modules'
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route
          index
          element={!snapshot && loading ? <PageFallback /> : <Navigate to={`/p/${first}`} replace />}
        />
        <Route path="p/:pageKey" element={<CustomPage />} />
      </Route>
    </Routes>
  )
}

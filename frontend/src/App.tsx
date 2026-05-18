import { Suspense, lazy, type ReactElement } from 'react'
import { Routes, Route, Navigate, useParams } from 'react-router-dom'
import Layout from './components/Layout'
import { pages, getPage } from './pages'

const BirthdayPage = lazy(() => import('./components/BirthdayPage'))
const EventPage = lazy(() => import('./components/EventPage'))
const CdnOpsPage = lazy(() => import('./components/CdnOpsPage'))
const CertStorePage = lazy(() => import('./components/CertStorePage'))
const AcmePage = lazy(() => import('./components/AcmePage'))
const SmsPage = lazy(() => import('./components/SmsPage'))
const SchedulerPage = lazy(() => import('./components/SchedulerPage'))
const NotifyPage = lazy(() => import('./components/NotifyPage'))

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
  birthday: () => <BirthdayPage />,
  event: () => <EventPage />,
  cdn: () => <CdnOpsPage />,
  cas: () => <CertStorePage />,
  acme: () => <AcmePage />,
  sms: () => <SmsPage />,
  scheduler: () => <SchedulerPage />,
  notify: () => <NotifyPage />,
}

function CustomPage() {
  const { pageKey } = useParams()
  const def = getPage(pageKey)
  const render = def ? PAGE_COMPONENTS[def.key] : undefined
  if (!def || !render) return <Empty />
  return <Suspense fallback={<PageFallback />}>{render()}</Suspense>
}

export default function App() {
  const first = pages[0]?.key
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route
          index
          element={first ? <Navigate to={`/p/${first}`} replace /> : <Empty />}
        />
        <Route path="p/:pageKey" element={<CustomPage />} />
      </Route>
    </Routes>
  )
}

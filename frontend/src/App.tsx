import type { ReactElement } from 'react'
import { Routes, Route, Navigate, useParams } from 'react-router-dom'
import Layout from './components/Layout'
import TableView from './components/TableView'
import CdnPage from './components/CdnPage'
import CasPage from './components/CasPage'
import AcmePage from './components/AcmePage'
import SmsPage from './components/SmsPage'
import SchedulerPage from './components/SchedulerPage'
import { tables } from './tables'
import { getPage } from './pages'

function Empty() {
  return (
    <div className="mx-auto max-w-md px-4 py-20 text-center text-muted-foreground">
      <p className="text-[15px] font-medium">还没有任何模块</p>
      <p className="mt-2 text-[13px]">在 <code>src/tables.ts</code> 添加业务模块。</p>
    </div>
  )
}

const PAGE_COMPONENTS: Record<string, () => ReactElement> = {
  cdn: () => <CdnPage />,
  cas: () => <CasPage />,
  acme: () => <AcmePage />,
  sms: () => <SmsPage />,
  scheduler: () => <SchedulerPage />,
}

function CustomPage() {
  const { pageKey } = useParams()
  const def = getPage(pageKey)
  const render = def ? PAGE_COMPONENTS[def.key] : undefined
  if (!def || !render) return <Empty />
  return render()
}

export default function App() {
  const first = tables[0]?.key
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route
          index
          element={first ? <Navigate to={`/t/${first}`} replace /> : <Empty />}
        />
        <Route path="t/:tableKey" element={<TableView />} />
        <Route path="p/:pageKey" element={<CustomPage />} />
      </Route>
    </Routes>
  )
}

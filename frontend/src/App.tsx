import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import TableView from './components/TableView'
import { tables } from './tables'

function Empty() {
  return (
    <div className="mx-auto max-w-md px-4 py-20 text-center text-muted-foreground">
      <p className="text-[15px] font-medium">还没有任何模块</p>
      <p className="mt-2 text-[13px]">在 <code>src/tables.ts</code> 添加业务模块。</p>
    </div>
  )
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
      </Route>
    </Routes>
  )
}

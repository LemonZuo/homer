import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api } from '../../api'
import type { QueryType } from './types'
import { extractList } from './utils'

export function useSmsMessages(selectedID: number | null) {
  const [queryType, setQueryType] = useState<QueryType>(1)
  const [keyword, setKeyword] = useState('')
  const [pageNum, setPageNum] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [queryLoading, setQueryLoading] = useState(false)
  const [queryRaw, setQueryRaw] = useState<any>(null)

  const runQuery = useCallback(
    async (overrides?: Partial<{ pageNum: number; type: QueryType; keyword: string; pageSize: number }>) => {
      if (selectedID == null) {
        toast.error('请先选择短信转发器')
        return
      }
      setQueryLoading(true)
      try {
        const { data } = await api.post('/sms/query', {
          target_id: selectedID,
          type: overrides?.type ?? queryType,
          page_num: overrides?.pageNum ?? pageNum,
          page_size: overrides?.pageSize ?? pageSize,
          keyword: overrides?.keyword ?? keyword,
        })
        setQueryRaw(data)
      } catch (e: any) {
        toast.error(e?.response?.data?.error || e?.message || '查询失败')
      } finally {
        setQueryLoading(false)
      }
    },
    [selectedID, queryType, pageNum, pageSize, keyword],
  )

  useEffect(() => {
    if (selectedID == null) {
      setQueryRaw(null)
      return
    }
    let cancelled = false
    setQueryLoading(true)
    setPageNum(1)
    api
      .post('/sms/query', {
        target_id: selectedID,
        type: queryType,
        page_num: 1,
        page_size: pageSize,
        keyword: '',
      })
      .then(({ data }) => {
        if (!cancelled) setQueryRaw(data)
      })
      .catch((e) => {
        if (!cancelled) toast.error(e?.response?.data?.error || e?.message || '查询失败')
      })
      .finally(() => {
        if (!cancelled) setQueryLoading(false)
      })
    return () => {
      cancelled = true
    }
    // 只在切换转发器时自动刷新，保持原页面行为。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedID])

  const list = useMemo(() => extractList(queryRaw), [queryRaw])
  const hasNext = list.length >= pageSize

  return {
    queryType,
    setQueryType,
    keyword,
    setKeyword,
    pageNum,
    setPageNum,
    pageSize,
    setPageSize,
    queryLoading,
    queryRaw,
    runQuery,
    list,
    hasNext,
  }
}

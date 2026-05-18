import { useEffect, useState } from 'react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { Maximize2, Minimize2 } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import { StreamLanguage } from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { cn } from '../../lib/utils'

const shellLang = StreamLanguage.define(shell)

function usePrefersDark() {
  const [dark, setDark] = useState(
    () =>
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches,
  )
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = (e: MediaQueryListEvent) => setDark(e.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])
  return dark
}

export function CodeEditor({
  id,
  value,
  onChange,
  placeholder,
  className,
}: {
  id?: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
  className?: string
}) {
  const [fs, setFs] = useState(false)
  const dark = usePrefersDark()

  const editor = (full: boolean) => (
    <div
      className={cn(
        'relative overflow-hidden bg-card',
        full
          ? 'h-full border-t border-input'
          : cn(
              'h-[76px] rounded-md border border-input shadow-sm',
              'focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30',
              className,
            ),
      )}
    >
      <CodeMirror
        id={id}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        theme={dark ? 'dark' : 'light'}
        extensions={[shellLang]}
        height={full ? '100%' : '76px'}
        maxHeight={full ? '100%' : '76px'}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLine: !full ? false : true,
          foldGutter: false,
          autocompletion: false,
        }}
        className={cn(
          'text-[12px]',
          full && 'h-full [&_.cm-editor]:h-full [&_.cm-scroller]:overflow-auto',
        )}
      />
      <button
        type="button"
        onClick={() => setFs((v) => !v)}
        title={full ? '退出全屏 (Esc)' : '全屏编辑'}
        className={cn(
          'absolute right-2 top-2 z-10 rounded-md border border-input bg-card/80 p-1.5',
          'text-muted-foreground backdrop-blur transition-colors hover:text-foreground hover:bg-accent',
        )}
      >
        {full ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
      </button>
    </div>
  )

  return (
    <>
      {fs ? (
        <div className="h-[76px] rounded-md border border-input bg-card shadow-sm" />
      ) : (
        editor(false)
      )}
      <DialogPrimitive.Root open={fs} onOpenChange={setFs}>
        <DialogPrimitive.Portal>
          <DialogPrimitive.Overlay className="fixed inset-0 z-[60] bg-background/95 backdrop-blur" />
          <DialogPrimitive.Content
            aria-describedby={undefined}
            onWheel={(e) => e.stopPropagation()}
            className="fixed inset-0 z-[60] flex flex-col outline-none"
          >
            <div className="px-3 py-1">
              <DialogPrimitive.Title className="text-xs font-medium text-muted-foreground">
                部署命令 —— 全屏编辑（Esc 退出）
              </DialogPrimitive.Title>
            </div>
            <div className="min-h-0 flex-1">{fs && editor(true)}</div>
          </DialogPrimitive.Content>
        </DialogPrimitive.Portal>
      </DialogPrimitive.Root>
    </>
  )
}

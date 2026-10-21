import { Component, type ReactNode } from "react"

// 顶层错误边界: 渲染崩溃时显示错误信息而不是整页白屏。
export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children
    return (
      <div className="flex-1 flex flex-col items-center justify-center gap-3 p-8 text-center">
        <p className="text-sm font-medium">界面出错了,没有白屏兜底</p>
        <pre className="max-w-2xl text-xs text-muted-foreground whitespace-pre-wrap border rounded-md p-3 text-left overflow-auto">
          {error.stack ?? error.message}
        </pre>
        <button
          className="h-8 px-3 rounded-md border text-sm hover:bg-accent"
          onClick={() => window.location.reload()}
        >
          重新加载
        </button>
      </div>
    )
  }
}

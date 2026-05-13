import { parseDiff, type DiffLine } from '../lib/diff'

interface DiffViewProps {
  diff: string
}

function Line({ line, isLast }: { line: DiffLine; isLast: boolean }) {
  const bgColor =
    line.type === 'added'
      ? 'bg-green-950/40'
      : line.type === 'removed'
        ? 'bg-red-950/40'
        : ''

  const gutterColor =
    line.type === 'added'
      ? 'text-green-400'
      : line.type === 'removed'
        ? 'text-red-400'
        : 'text-muted-foreground'

  const gutter = line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' '

  return (
    <div className={`flex ${bgColor} ${isLast ? '' : 'border-b border-border/30'}`}>
      <span className={`w-8 shrink-0 text-right pr-2 select-none ${gutterColor}`}>
        {gutter}
      </span>
      <span className="flex-1 whitespace-pre">{line.content}</span>
    </div>
  )
}

export function DiffView({ diff }: DiffViewProps) {
  const files = parseDiff(diff)

  if (files.length === 0) {
    return <p className="text-sm text-muted-foreground">No code changes yet.</p>
  }

  return (
    <div className="space-y-4">
      {files.map((file, fi) => (
        <div key={fi} className="rounded-lg overflow-hidden border border-border">
          <div className="bg-muted px-3 py-1.5 text-xs font-mono text-foreground border-b border-border">
            {file.fileName}
          </div>
          <div className="text-xs font-mono leading-5 overflow-x-auto">
            {file.lines.map((line, li) => (
              <Line key={li} line={line} isLast={li === file.lines.length - 1} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

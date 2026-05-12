import { Badge } from './ui/badge'

const labels: Record<string, string> = {
  running: 'Running',
  needs_attention: 'Needs Attention',
  done: 'Done',
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <Badge variant={status as keyof typeof Badge}>
      <span
        className={`mr-1.5 h-1.5 w-1.5 rounded-full ${
          status === 'running'
            ? 'bg-blue-400 animate-pulse'
            : status === 'needs_attention'
              ? 'bg-amber-400'
              : 'bg-emerald-400'
        }`}
      />
      {labels[status] || status}
    </Badge>
  )
}

import type { HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

const badgeVariants = {
  running: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  needs_attention: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  done: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
} as const

interface BadgeProps extends HTMLAttributes<HTMLDivElement> {
  variant?: keyof typeof badgeVariants
}

export function Badge({ className, variant = 'running', ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium',
        badgeVariants[variant],
        className
      )}
      {...props}
    />
  )
}

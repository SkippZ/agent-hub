import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { ReactNode } from 'react'

const components: Components = {
  p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
  ul: ({ children }) => <ul className="list-disc pl-5 mb-2 space-y-1">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal pl-5 mb-2 space-y-1">{children}</ol>,
  li: ({ children }) => <li>{children}</li>,
  code: ({ className, children, ...props }) => {
    const isInline = !className
    if (isInline) {
      return (
        <code className="bg-muted rounded px-1 py-0.5 text-xs font-mono" {...props}>
          {children}
        </code>
      )
    }
    return (
      <pre className="bg-muted rounded-lg p-3 my-2 overflow-x-auto text-xs font-mono">
        <code className={className} {...props}>
          {children}
        </code>
      </pre>
    )
  },
  pre: ({ children }) => <>{children}</>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-muted-foreground/30 pl-3 my-2 text-muted-foreground italic">
      {children}
    </blockquote>
  ),
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary underline hover:no-underline">
      {children}
    </a>
  ),
  h1: ({ children }) => <h1 className="text-lg font-bold mb-2 mt-3">{children}</h1>,
  h2: ({ children }) => <h2 className="text-base font-bold mb-2 mt-3">{children}</h2>,
  h3: ({ children }) => <h3 className="text-sm font-bold mb-1 mt-2">{children}</h3>,
  table: ({ children }) => (
    <div className="overflow-x-auto my-2">
      <table className="text-xs border-collapse border border-border">{children}</table>
    </div>
  ),
  th: ({ children }) => <th className="border border-border px-2 py-1 bg-muted font-medium">{children}</th>,
  td: ({ children }) => <td className="border border-border px-2 py-1">{children}</td>,
  hr: () => <hr className="my-3 border-border" />,
}

export function Markdown({ content }: { content: string }) {
  return (
    <ReactMarkdown components={components} remarkPlugins={[remarkGfm]}>
      {content}
    </ReactMarkdown>
  )
}

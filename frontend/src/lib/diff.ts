export type DiffLineType = 'added' | 'removed' | 'context'

export interface DiffLine {
  type: DiffLineType
  content: string
}

export interface FileDiff {
  fileName: string
  lines: DiffLine[]
}

function parseFileName(line: string): string {
  const match = line.match(/^diff --git a\/(.+?) b\//)
  if (match) return match[1]
  const m2 = line.match(/^\+\+\+ b\/(.+)/)
  if (m2) return m2[1]
  return ''
}

export function parseDiff(diffText: string): FileDiff[] {
  const files: FileDiff[] = []
  let currentFile: FileDiff | null = null

  const lines = diffText.split('\n')

  for (const raw of lines) {
    if (raw.startsWith('diff --git')) {
      if (currentFile) {
        files.push(currentFile)
      }
      currentFile = {
        fileName: parseFileName(raw),
        lines: [],
      }
      continue
    }

    if (!currentFile) continue

    if (raw.startsWith('--- ') || raw.startsWith('+++ ') || raw.startsWith('@@ ') || raw.startsWith('index ')) {
      continue
    }

    if (raw.startsWith('+')) {
      currentFile.lines.push({ type: 'added', content: raw.slice(1) })
    } else if (raw.startsWith('-')) {
      currentFile.lines.push({ type: 'removed', content: raw.slice(1) })
    } else if (raw.startsWith(' ')) {
      currentFile.lines.push({ type: 'context', content: raw.slice(1) })
    } else if (raw.startsWith('\\ ')) {
      currentFile.lines.push({ type: 'context', content: raw.slice(2) })
    }
  }

  if (currentFile) {
    files.push(currentFile)
  }

  return files.filter((f) => f.lines.length > 0)
}

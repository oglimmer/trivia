export function errMsg(e: unknown, fallback = ''): string {
  if (e instanceof Error) return e.message
  if (typeof e === 'string') return e
  return fallback
}

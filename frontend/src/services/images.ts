export function imageUrl(
  id?: string | null,
  variant: 'thumb' | 'medium' | 'orig' = 'orig',
) {
  if (!id) return ''
  return `/api/images/${id}${variant === 'orig' ? '' : '/' + variant}`
}

/**
 * Converts a human-readable flow name into a URL-safe id suitable for use as
 * a DSL definition.id.
 *
 * Examples:
 *   "My New Flow"  → "my-new-flow"
 *   "  Order Processing 2 " → "order-processing-2"
 *   "!!!" → "new-flow"   (fallback when the result would be empty)
 */
export function slugify(name: string): string {
  const slug = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return slug || 'new-flow'
}

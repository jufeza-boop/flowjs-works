/**
 * Converts an unknown caught error to a human-readable string.
 * Use this instead of the inline `err instanceof Error ? err.message : String(err)` pattern.
 */
export function toErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

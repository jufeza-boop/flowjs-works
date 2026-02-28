import { describe, it, expect } from 'vitest'
import { slugify } from './slugify'

describe('slugify', () => {
  it('lowercases and hyphenates a simple name', () => {
    expect(slugify('My New Flow')).toBe('my-new-flow')
  })

  it('handles leading/trailing whitespace', () => {
    expect(slugify('  Order Processing  ')).toBe('order-processing')
  })

  it('collapses multiple consecutive special chars into one hyphen', () => {
    expect(slugify('flow -- 2.0')).toBe('flow-2-0')
  })

  it('strips leading and trailing hyphens from the result', () => {
    expect(slugify('---hello---')).toBe('hello')
  })

  it('falls back to "new-flow" for empty input', () => {
    expect(slugify('')).toBe('new-flow')
  })

  it('falls back to "new-flow" when only special chars remain', () => {
    expect(slugify('!!!')).toBe('new-flow')
  })

  it('preserves numbers', () => {
    expect(slugify('Flow 42')).toBe('flow-42')
  })

  it('handles already-slug input unchanged', () => {
    expect(slugify('my-flow-id')).toBe('my-flow-id')
  })
})

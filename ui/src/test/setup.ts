import '@testing-library/jest-dom'
import { vi } from 'vitest'

// Mock window.alert for testing
Object.defineProperty(window, 'alert', {
  value: vi.fn(),
  writable: true,
})

// Mock console methods to avoid noise in tests
Object.defineProperty(console, 'log', { value: vi.fn(), writable: true })
Object.defineProperty(console, 'error', { value: vi.fn(), writable: true })
Object.defineProperty(console, 'warn', { value: vi.fn(), writable: true })
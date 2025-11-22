import '@testing-library/jest-dom'
import { vi } from 'vitest'
import { fetch, Headers, Request, Response, FormData } from 'undici'

// Polyfill fetch and related APIs for jsdom environment
// This is required for MSW to work properly with RTK Query
Object.assign(globalThis, {
  fetch,
  Headers,
  Request,
  Response,
  FormData,
})

// Setup a base URL for tests
Object.defineProperty(window, 'location', {
  value: {
    href: 'http://localhost:3000',
    origin: 'http://localhost:3000',
    protocol: 'http:',
    host: 'localhost:3000',
    hostname: 'localhost',
    port: '3000',
    pathname: '/',
    search: '',
    hash: '',
  },
  writable: true,
})

// Mock window.alert for testing
Object.defineProperty(window, 'alert', {
  value: vi.fn(),
  writable: true,
})

// Mock console methods to avoid noise in tests
Object.defineProperty(console, 'log', { value: vi.fn(), writable: true })
Object.defineProperty(console, 'error', { value: vi.fn(), writable: true })
Object.defineProperty(console, 'warn', { value: vi.fn(), writable: true })
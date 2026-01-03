import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import RSSComponent from './RSSComponent'
import * as api from '../../services/api'

// Mock the API hook
vi.mock('../../services/api', async () => {
  const actual = await vi.importActual('../../services/api')
  return {
    ...actual,
    useGetRSSQuery: vi.fn(),
  }
})

describe('RSSComponent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  })

  it('renders loading state', () => {
    (api.useGetRSSQuery as any).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
    })
    render(<RSSComponent />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
  })

  it('renders error state', () => {
    (api.useGetRSSQuery as any).mockReturnValue({
      data: undefined,
      error: { status: 500 },
      isLoading: false,
    })
    render(<RSSComponent />)
    expect(screen.getByText('Failed to load RSS feed.')).toBeInTheDocument()
  })

  it('renders data state', () => {
    (api.useGetRSSQuery as any).mockReturnValue({
      data: { url: 'https://example.com/feed.rss' },
      error: undefined,
      isLoading: false,
    })
    render(<RSSComponent />)
    expect(screen.getByText('https://example.com/feed.rss')).toBeInTheDocument()
  })

  it('renders API error message', () => {
    (api.useGetRSSQuery as any).mockReturnValue({
      data: { error: 'RSS feed not found' },
      error: undefined,
      isLoading: false,
    })
    render(<RSSComponent />)
    expect(screen.getByText('RSS feed not found')).toBeInTheDocument()
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import RSSComponent from './RSSComponent'
import * as api from '../../services/api'

// Mock Auth0
const mockLoginWithRedirect = vi.fn();
vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({
    loginWithRedirect: mockLoginWithRedirect,
  }),
}))

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

  it('renders google auth error state', () => {
    (api.useGetRSSQuery as any).mockReturnValue({
      data: undefined,
      error: { status: 424 },
      isLoading: false,
    })
    render(<RSSComponent />)
    expect(screen.getByText(/Google authentication failed/)).toBeInTheDocument()
    
    const loginButton = screen.getByText(/log in again/);
    expect(loginButton).toBeInTheDocument()
    
    fireEvent.click(loginButton);
    expect(mockLoginWithRedirect).toHaveBeenCalled();
  })
})

import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Login from '../Login';
import * as setupApi from '../../api/setup';

// Mock AuthContext so useAuth works in tests
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({
    login: vi.fn(),
    logout: vi.fn(),
    isAuthenticated: false,
    isLoading: false,
    user: null,
  }),
}));

// Mock API client
vi.mock('../../api/client', () => ({
  default: {
    post: vi.fn().mockResolvedValue({ data: {} }),
    get: vi.fn().mockResolvedValue({ data: {} }),
  },
}));

// Mock react-router-dom
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock the API module
vi.mock('../../api/setup', () => ({
  getSetupStatus: vi.fn(),
  performSetup: vi.fn(),
}));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

const renderWithProviders = (ui: React.ReactNode) => {
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('Login Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient.clear();
  });

  it('renders login form and logo when setup is not required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: false });

    renderWithProviders(<Login />);

    // The page will redirect to setup if setup is required; for our test we mock it as not required
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Sign In' })).toBeTruthy();
    });

    // Verify logo is present
    expect(screen.getAllByAltText('Charon').length).toBeGreaterThan(0);
  });
});

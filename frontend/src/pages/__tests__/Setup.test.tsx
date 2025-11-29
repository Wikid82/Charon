import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Setup from '../Setup';
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

describe('Setup Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient.clear();
  });

  it('renders setup form when setup is required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to Charon')).toBeTruthy();
    });

    // Verify logo is present
    expect(screen.getAllByAltText('Charon').length).toBeGreaterThan(0);

    expect(screen.getByLabelText('Name')).toBeTruthy();
    expect(screen.getByLabelText('Email Address')).toBeTruthy();
    expect(screen.getByLabelText('Password')).toBeTruthy();
  });

  it('does not render form when setup is not required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.queryByText('Welcome to Charon')).toBeNull();
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login');
    });
  });

  it('submits form successfully', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true });
    vi.mocked(setupApi.performSetup).mockResolvedValue();

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to Charon')).toBeTruthy();
    });

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Name'), 'Admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText('Password'), 'password123')
    await user.click(screen.getByRole('button', { name: 'Create Admin Account' }))

    await waitFor(() => {
      expect(setupApi.performSetup).toHaveBeenCalledWith({
        name: 'Admin',
        email: 'admin@example.com',
        password: 'password123',
      });
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
  });

  it('displays error on submission failure', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true });
    vi.mocked(setupApi.performSetup).mockRejectedValue({
      response: { data: { error: 'Setup failed' } }
    });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to Charon')).toBeTruthy();
    });

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Name'), 'Admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText('Password'), 'password123')
    await user.click(screen.getByRole('button', { name: 'Create Admin Account' }))

    await waitFor(() => {
      expect(screen.getByText('Setup failed')).toBeTruthy();
    });
  });
});

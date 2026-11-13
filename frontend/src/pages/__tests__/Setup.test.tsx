import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AxiosError, type AxiosResponse } from 'axios';
import { MemoryRouter } from 'react-router';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import * as setupApi from '../../api/setup';
import Setup from '../Setup';

// Mock AuthContext so useAuth works in tests
let mockIsAuthenticated = false;
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({
    login: vi.fn(),
    logout: vi.fn(),
    isAuthenticated: mockIsAuthenticated,
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

// Mock react-router
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
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
    mockIsAuthenticated = false;
  });

  it('renders setup form when setup is required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true, setup_code_required: false });

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
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: false, setup_code_required: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.queryByText('Welcome to Charon')).toBeNull();
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login');
    });
  });

  it('redirects an authenticated user to the dashboard when setup is not required', async () => {
    mockIsAuthenticated = true;
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: false, setup_code_required: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
    expect(mockNavigate).not.toHaveBeenCalledWith('/login');
  });

  it('submits form successfully', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true, setup_code_required: false });
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
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true, setup_code_required: false });
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

  it('has proper autocomplete attributes for password managers', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true, setup_code_required: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to Charon')).toBeTruthy();
    });

    const emailInput = screen.getByLabelText('Email Address')
    const passwordInput = screen.getByLabelText('Password')

    expect(emailInput).toHaveAttribute('autocomplete', 'email')
    expect(passwordInput).toHaveAttribute('autocomplete', 'new-password')
  });
});

describe('Setup Page setup code', () => {
  const codeStatus = { setupRequired: true, setup_code_required: true };

  const axiosRefusal = (status: number, data: unknown): AxiosError => {
    const response = { status, statusText: '', headers: {}, config: {}, data } as AxiosResponse;
    return new AxiosError(`Request failed with status code ${status}`, 'ERR_BAD_REQUEST', undefined, undefined, response);
  };

  const fillAdminForm = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.type(screen.getByLabelText('Name'), 'Admin');
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
  };

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient.clear();
    mockIsAuthenticated = false;
  });

  it('hides the setup code field when the server does not require it', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true, setup_code_required: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to Charon')).toBeTruthy();
    });
    expect(screen.queryByLabelText('Setup code')).toBeNull();
  });

  it('shows a required setup code field with a hint and autocomplete off', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue(codeStatus);

    renderWithProviders(<Setup />);

    const input = await screen.findByLabelText('Setup code');
    expect(input).toBeRequired();
    expect(input).toHaveAttribute('autocomplete', 'off');
    expect(input).toHaveAttribute('type', 'text');
    expect(input).toHaveAccessibleDescription('Find it in the Charon logs. For example: docker logs charon');
  });

  it('sends the setup code with the setup payload when it is required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue(codeStatus);
    vi.mocked(setupApi.performSetup).mockResolvedValue();

    renderWithProviders(<Setup />);

    const user = userEvent.setup();
    await screen.findByLabelText('Setup code');
    await fillAdminForm(user);
    await user.type(screen.getByLabelText('Setup code'), 'ABC123');
    await user.click(screen.getByRole('button', { name: 'Create Admin Account' }));

    await waitFor(() => {
      expect(setupApi.performSetup).toHaveBeenCalledWith({
        name: 'Admin',
        email: 'admin@example.com',
        password: 'password123',
        setup_code: 'ABC123',
      });
    });
  });

  it('shows the fixed text for a wrong setup code', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue(codeStatus);
    vi.mocked(setupApi.performSetup).mockRejectedValue(
      axiosRefusal(403, { error: 'Invalid setup code', error_code: 'setup_code_invalid' }),
    );

    renderWithProviders(<Setup />);

    const user = userEvent.setup();
    await screen.findByLabelText('Setup code');
    await fillAdminForm(user);
    await user.type(screen.getByLabelText('Setup code'), 'WRONG');
    await user.click(screen.getByRole('button', { name: 'Create Admin Account' }));

    expect(await screen.findByText('The setup code is incorrect. Check the Charon logs and try again.')).toBeTruthy();
    expect(screen.getByRole('alert')).toHaveTextContent('The setup code is incorrect. Check the Charon logs and try again.');
    expect(mockNavigate).not.toHaveBeenCalledWith('/');
  });

  it('shows the fixed text when attempts are throttled', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue(codeStatus);
    vi.mocked(setupApi.performSetup).mockRejectedValue(
      axiosRefusal(429, { error: 'Too many requests. Please wait before trying again.' }),
    );

    renderWithProviders(<Setup />);

    const user = userEvent.setup();
    await screen.findByLabelText('Setup code');
    await fillAdminForm(user);
    await user.type(screen.getByLabelText('Setup code'), 'ABC123');
    await user.click(screen.getByRole('button', { name: 'Create Admin Account' }));

    expect(await screen.findByText('Too many attempts. Wait a few minutes and try again.')).toBeTruthy();
  });
});

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Setup from '../Setup';
import * as setupApi from '../../api/setup';

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
      expect(screen.getByText('Welcome to CPM+')).toBeTruthy();
    });

    expect(screen.getByLabelText('Name')).toBeTruthy();
    expect(screen.getByLabelText('Email Address')).toBeTruthy();
    expect(screen.getByLabelText('Password')).toBeTruthy();
  });

  it('does not render form when setup is not required', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: false });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.queryByText('Welcome to CPM+')).toBeNull();
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
      expect(screen.getByText('Welcome to CPM+')).toBeTruthy();
    });

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Admin' } });
    fireEvent.change(screen.getByLabelText('Email Address'), { target: { value: 'admin@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } });

    fireEvent.click(screen.getByRole('button', { name: 'Create Admin Account' }));

    await waitFor(() => {
      expect(setupApi.performSetup).toHaveBeenCalledWith({
        name: 'Admin',
        email: 'admin@example.com',
        password: 'password123',
      }, expect.anything());
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login');
    });
  });

  it('displays error on submission failure', async () => {
    vi.mocked(setupApi.getSetupStatus).mockResolvedValue({ setupRequired: true });
    vi.mocked(setupApi.performSetup).mockRejectedValue({
      response: { data: { error: 'Setup failed' } }
    });

    renderWithProviders(<Setup />);

    await waitFor(() => {
      expect(screen.getByText('Welcome to CPM+')).toBeTruthy();
    });

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Admin' } });
    fireEvent.change(screen.getByLabelText('Email Address'), { target: { value: 'admin@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } });

    fireEvent.click(screen.getByRole('button', { name: 'Create Admin Account' }));

    await waitFor(() => {
      expect(screen.getByText('Setup failed')).toBeTruthy();
    });
  });
});

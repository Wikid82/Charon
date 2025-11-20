import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Setup from '../Setup';
import * as setupApi from '../../api/setup';

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
  });
});

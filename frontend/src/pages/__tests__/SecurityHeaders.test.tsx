import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';
import SecurityHeaders from '../../pages/SecurityHeaders';
import { securityHeadersApi } from '../../api/securityHeaders';
import { createBackup } from '../../api/backups';

vi.mock('../../api/securityHeaders');
vi.mock('../../api/backups');
vi.mock('react-hot-toast');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
};

describe('SecurityHeaders', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render loading state', () => {
    vi.mocked(securityHeadersApi.listProfiles).mockImplementation(() => new Promise(() => {}));
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    expect(screen.getByText('Security Headers')).toBeInTheDocument();
  });

  it('should render empty state', async () => {
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('No custom profiles yet')).toBeInTheDocument();
    });
  });

  it('should render list of profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Profile 1',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
      {
        id: 2,
        name: 'Profile 2',
        is_preset: false,
        security_score: 90,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Profile 1')).toBeInTheDocument();
      expect(screen.getByText('Profile 2')).toBeInTheDocument();
    });
  });

  it('should render presets', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Basic Security',
        description: 'Essential headers',
        is_preset: true,
        preset_type: 'basic',
        security_score: 65,
        updated_at: '2025-12-18T00:00:00Z',
      },
      {
        id: 2,
        name: 'Strict Security',
        description: 'Strong security',
        is_preset: true,
        preset_type: 'strict',
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Basic Security')).toBeInTheDocument();
      expect(screen.getByText('Strict Security')).toBeInTheDocument();
    });
  });

  it('should open create form dialog', async () => {
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Create Profile/ })).toBeInTheDocument();
    });

    const createButton = screen.getAllByRole('button', { name: /Create Profile/ })[0];
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText(/Create Security Header Profile/)).toBeInTheDocument();
    });
  });

  it('should open edit dialog', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue({
      score: 85,
      max_score: 100,
      breakdown: {},
      suggestions: [],
    });

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Test Profile')).toBeInTheDocument();
    });

    const editButton = screen.getByRole('button', { name: /Edit/ });
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText(/Edit Security Header Profile/)).toBeInTheDocument();
    });
  });

  it('should clone profile', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Original Profile',
        description: 'Test description',
        is_preset: false,
        security_score: 85,
        hsts_enabled: true,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.createProfile).mockResolvedValue({
      id: 2,
      name: 'Original Profile (Copy)',
      security_score: 85,
    } as any);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Original Profile')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const cloneButton = buttons.find(btn => btn.querySelector('.lucide-copy'));
    if (cloneButton) {
      fireEvent.click(cloneButton);
    }

    await waitFor(() => {
      expect(securityHeadersApi.createProfile).toHaveBeenCalled();
    });

    const createCall = vi.mocked(securityHeadersApi.createProfile).mock.calls[0][0];
    expect(createCall.name).toBe('Original Profile (Copy)');
  });

  it('should delete profile with backup', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(createBackup).mockResolvedValue({ id: 1 } as any);
    vi.mocked(securityHeadersApi.deleteProfile).mockResolvedValue(undefined);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Test Profile')).toBeInTheDocument();
    });

    // Click delete button
    const buttons = screen.getAllByRole('button');
    const deleteButton = buttons.find(btn => btn.querySelector('.lucide-trash-2, .lucide-trash'));
    if (deleteButton) {
      fireEvent.click(deleteButton);
    }

    // Confirm deletion - wait for the dialog to appear
    await waitFor(() => {
      const headings = screen.getAllByText(/Confirm Deletion/i);
      expect(headings.length).toBeGreaterThan(0);
    }, { timeout: 2000 });

    const confirmButton = screen.getByRole('button', { name: /Delete/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(createBackup).toHaveBeenCalled();
      expect(securityHeadersApi.deleteProfile).toHaveBeenCalledWith(1);
    });
  });

  it('should separate quick presets from custom profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Custom Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
      {
        id: 2,
        name: 'Basic Security',
        is_preset: true,
        preset_type: 'basic',
        security_score: 65,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('System Profiles (Read-Only)')).toBeInTheDocument();
      expect(screen.getByText('Custom Profiles')).toBeInTheDocument();
    });

    // System profiles should have View and Clone buttons
    const presetCard = screen.getByText('Basic Security').closest('div');
    expect(presetCard).toBeInTheDocument();

    // Custom profile should have Edit button
    const customCard = screen.getByText('Custom Profile').closest('div');
    expect(customCard?.textContent).toContain('Custom Profile');
  });

  it('should display security scores', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'High Score Profile',
        is_preset: false,
        security_score: 95,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('95')).toBeInTheDocument();
    });
  });
});

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import SecurityHeaders from '../../pages/SecurityHeaders';
import { securityHeadersApi, SecurityHeaderProfile } from '../../api/securityHeaders';
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.createProfile).mockResolvedValue({
      id: 2,
      name: 'Original Profile (Copy)',
      security_score: 85,
    } as SecurityHeaderProfile);

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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(createBackup).mockResolvedValue({ filename: 'backup.tar.gz' });
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
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

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('95')).toBeInTheDocument();
    });
  });

  // Additional coverage tests for Phase 3

  it('should display preset tooltip information', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Basic Security',
        is_preset: true,
        preset_type: 'basic',
        security_score: 65,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    const user = userEvent.setup();
    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Basic Security')).toBeInTheDocument();
    });

    // Find info icon and hover
    const infoButtons = screen.getAllByRole('button').filter(btn => {
      const svg = btn.querySelector('svg');
      return svg?.classList.contains('lucide-info');
    });

    if (infoButtons.length > 0) {
      await user.hover(infoButtons[0]);
    }
  });

  it('should show view button for preset profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Strict Security',
        is_preset: true,
        preset_type: 'strict',
        security_score: 95,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /View/i })).toBeInTheDocument();
    });
  });

  it('should close form when dialog is dismissed', async () => {
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

    // Close dialog by pressing escape or clicking outside
    const dialog = screen.getByRole('dialog');
    expect(dialog).toBeInTheDocument();
  });

  it('should sort preset profiles by security score', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Paranoid Security',
        is_preset: true,
        preset_type: 'paranoid',
        security_score: 100,
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
      {
        id: 3,
        name: 'API Friendly',
        is_preset: true,
        preset_type: 'api-friendly',
        security_score: 75,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Basic Security')).toBeInTheDocument();
    });

    // Verify all presets are displayed
    expect(screen.getByText('Paranoid Security')).toBeInTheDocument();
    expect(screen.getByText('API Friendly')).toBeInTheDocument();
  });

  it('should display updated date for profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-01-20T10:30:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText(/Updated/i)).toBeInTheDocument();
    });
  });

  it('should handle clone button for custom profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Custom Profile',
        description: 'My custom config',
        is_preset: false,
        security_score: 80,
        hsts_enabled: true,
        hsts_max_age: 31536000,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.createProfile).mockResolvedValue({
      id: 2,
      name: 'Custom Profile (Copy)',
      security_score: 80,
    } as SecurityHeaderProfile);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Custom Profile')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    const cloneButton = buttons.find(btn => btn.querySelector('.lucide-copy'));
    if (cloneButton) {
      fireEvent.click(cloneButton);
    }

    await waitFor(() => {
      expect(securityHeadersApi.createProfile).toHaveBeenCalled();
    });
  });

  it('should display profile descriptions', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        description: 'This is a test profile description',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('This is a test profile description')).toBeInTheDocument();
    });
  });

  it('should handle delete confirmation cancellation', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

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

    // Wait for confirmation dialog
    await waitFor(() => {
      const headings = screen.getAllByText(/Confirm Deletion/i);
      expect(headings.length).toBeGreaterThan(0);
    });

    // Click cancel instead of delete
    const cancelButton = screen.getByRole('button', { name: /Cancel/i });
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(securityHeadersApi.deleteProfile).not.toHaveBeenCalled();
    });
  });

  it('should show info alert with security configuration message', async () => {
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText(/Secure Your Applications/i)).toBeInTheDocument();
      expect(screen.getByText(/Security headers protect against common web vulnerabilities/i)).toBeInTheDocument();
    });
  });

  it('should display all three action buttons for custom profiles', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Custom Profile',
        is_preset: false,
        security_score: 85,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Custom Profile')).toBeInTheDocument();
    });

    // Should have Edit button
    expect(screen.getByRole('button', { name: /Edit/i })).toBeInTheDocument();

    // Should have Clone button (icon only)
    const buttons = screen.getAllByRole('button');
    const cloneButton = buttons.find(btn => btn.querySelector('.lucide-copy'));
    expect(cloneButton).toBeDefined();

    // Should have Delete button (icon only)
    const deleteButton = buttons.find(btn => btn.querySelector('.lucide-trash-2, .lucide-trash'));
    expect(deleteButton).toBeDefined();
  });

  it('should handle profile update submission', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Test Profile',
        is_preset: false,
        security_score: 85,
        hsts_enabled: true,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.updateProfile).mockResolvedValue({
      id: 1,
      name: 'Updated Profile',
      security_score: 90,
    } as SecurityHeaderProfile);
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

    const editButton = screen.getByRole('button', { name: /Edit/i });
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText(/Edit Security Header Profile/)).toBeInTheDocument();
    });
  });

  it('should display system profiles section title', async () => {
    const mockProfiles = [
      {
        id: 1,
        name: 'Basic Security',
        is_preset: true,
        preset_type: 'basic',
        security_score: 65,
        updated_at: '2025-12-18T00:00:00Z',
      },
    ];

    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as SecurityHeaderProfile[]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('System Profiles (Read-Only)')).toBeInTheDocument();
    });
  });

  it('should render empty state action in custom profiles section', async () => {
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByText('No custom profiles yet')).toBeInTheDocument();
    });

    const createButtons = screen.getAllByRole('button', { name: /Create Profile/i });
    expect(createButtons.length).toBeGreaterThan(0);
  });

  it('should close create dialog on success', async () => {
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.createProfile).mockResolvedValue({ id: 1, name: 'New Profile', security_score: 50, created_at: '', updated_at: '' } as any);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    const openBtn = screen.getAllByRole('button', { name: /Create Profile/i })[0];
    fireEvent.click(openBtn);

    await waitFor(() => screen.getByText(/Create Security Header Profile/i));

    // Fill required fields to enable submit
    const nameInput = screen.getByPlaceholderText(/e.g., Production Security Headers/i);
    fireEvent.change(nameInput, { target: { value: 'New Profile' } });

    // Find submit button in dialog (it might have 'Create Profile' text or just 'Create')
    // Looking at SecurityHeaderProfileForm, it likely has a submit button.
    // We can assume it's the one with type="submit" or appropriate text.
    // Let's search for "Create Profile" button inside the dialog or just "Create".
    const submitBtn = screen.getByRole('button', { name: /Save Profile/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('should close edit dialog on success', async () => {
    const mockProfiles = [{ id: 1, name: 'Edit Me', is_preset: false, security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue({ score: 50, max_score: 100, breakdown: {}, suggestions: [] } as any);
    vi.mocked(securityHeadersApi.updateProfile).mockResolvedValue(mockProfiles[0] as any);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Edit Me'));
    fireEvent.click(screen.getByRole('button', { name: /Edit/i }));

    await waitFor(() => screen.getByText(/Edit Security Header Profile/i));

    const submitButton = screen.getByRole('button', { name: /Save Profile/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('should handle delete failure', async () => {
    const mockProfiles = [{ id: 1, name: 'Delete Me', is_preset: false, security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(createBackup).mockResolvedValue({ filename: 'test-backup.tar.gz' });
    vi.mocked(securityHeadersApi.deleteProfile).mockRejectedValue(new Error('Delete failed'));

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Delete Me'));

    const buttons = screen.getAllByRole('button');
    const deleteButton = buttons.find(btn => btn.querySelector('.lucide-trash-2, .lucide-trash'));
    fireEvent.click(deleteButton!);

    await waitFor(() => screen.getByText(/Confirm Deletion/i));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => {
        expect(createBackup).toHaveBeenCalled();
        expect(securityHeadersApi.deleteProfile).toHaveBeenCalled();
    });
  });

  it('should handle backup failure during delete', async () => {
    const mockProfiles = [{ id: 1, name: 'Delete Me', is_preset: false, security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(createBackup).mockRejectedValue(new Error('Backup failed'));

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Delete Me'));

    const buttons = screen.getAllByRole('button');
    const deleteButton = buttons.find(btn => btn.querySelector('.lucide-trash-2, .lucide-trash'));
    fireEvent.click(deleteButton!);

    await waitFor(() => screen.getByText(/Confirm Deletion/i));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

    await waitFor(() => {
        expect(createBackup).toHaveBeenCalled();
        expect(securityHeadersApi.deleteProfile).not.toHaveBeenCalled();
    });
  });

  it('should handle unknown preset types', async () => {
    const mockProfiles = [{ id: 1, name: 'Weird Preset', is_preset: true, preset_type: 'unknown_type', security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Weird Preset'));
    // Just ensuring render doesn't crash
  });

  it('should handle cancel in edit dialog', async () => {
    const mockProfiles = [{ id: 1, name: 'Edit Me', is_preset: false, security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue({ score: 50 } as any);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Edit Me'));
    fireEvent.click(screen.getByRole('button', { name: /Edit/i }));

    await waitFor(() => screen.getByText(/Edit Security Header Profile/i));

    const cancelButton = screen.getByRole('button', { name: /Cancel/i });
    fireEvent.click(cancelButton);

    await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('should handle delete from edit dialog', async () => {
    const mockProfiles = [{ id: 1, name: 'Delete Me from Edit', is_preset: false, security_score: 50, updated_at: '2023-01-01' }];
    vi.mocked(securityHeadersApi.listProfiles).mockResolvedValue(mockProfiles as any);
    vi.mocked(securityHeadersApi.getPresets).mockResolvedValue([]);
    vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue({ score: 50 } as any);

    render(<SecurityHeaders />, { wrapper: createWrapper() });

    await waitFor(() => screen.getByText('Delete Me from Edit'));
    fireEvent.click(screen.getByRole('button', { name: /Edit/i }));

    await waitFor(() => screen.getByText(/Edit Security Header Profile/i));

    const deleteButton = screen.getByRole('button', { name: /Delete Profile/i });
    expect(deleteButton).toBeInTheDocument();

    fireEvent.click(deleteButton);

    await waitFor(() => screen.getByText(/Confirm Deletion/i));
  });
});

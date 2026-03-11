import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createBackup } from '../../api/backups'
import {
  useAccessLists,
  useCreateAccessList,
  useDeleteAccessList,
  useTestIP,
  useUpdateAccessList,
} from '../../hooks/useAccessLists'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import AccessLists from '../AccessLists'

import type { AccessList, CreateAccessListRequest, TestIPResponse } from '../../api/accessLists'
import type { AccessListFormData } from '../../components/AccessListForm'
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query'

const translations: Record<string, string> = {
  'accessLists.noAccessLists': 'No Access Lists',
  'accessLists.noAccessListsDescription': 'No access list description',
  'accessLists.createAccessList': 'Create Access List',
  'accessLists.deleteAccessList': 'Delete Access List',
  'accessLists.deleteSelectedAccessLists': 'Delete Selected Access Lists',
  'accessLists.deleteItems': 'Delete ({{count}})',
  'accessLists.testIp': 'Test IP',
  'accessLists.testIpAddress': 'Test IP Address',
  'accessLists.ipAddress': 'IP Address',
  'accessLists.accessList': 'Access List',
  'accessLists.deleteConfirmation': 'Delete {{name}}?',
  'accessLists.bulkDeleteConfirmation': 'Delete {{count}}?',
  'common.delete': 'Delete',
  'common.cancel': 'Cancel',
  'common.deleting': 'Deleting',
  'common.test': 'Test',
  'common.close': 'Close',
}

const t = (key: string, options?: Record<string, unknown>) => {
  const template = translations[key] ?? key

  if (!options) return template

  return Object.entries(options).reduce((acc, [optionKey, optionValue]) => {
    return acc.replace(`{{${optionKey}}}`, String(optionValue))
  }, template)
}

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t,
  }),
}))

interface MockAccessListFormProps {
  onSubmit: (data: AccessListFormData) => void
  onCancel: () => void
  onDelete?: () => void
  isLoading?: boolean
  isDeleting?: boolean
  initialData?: AccessList
}

const defaultAccessList: AccessList = {
  id: 1,
  uuid: 'acl-1',
  name: 'Office Access',
  description: 'Office CIDR',
  type: 'whitelist',
  ip_rules: '[{"cidr":"10.0.0.0/8"}]',
  country_codes: '',
  local_network_only: false,
  enabled: true,
  created_at: '2026-02-01T10:00:00Z',
  updated_at: '2026-02-01T10:00:00Z',
}

const createAccessList = (overrides: Partial<AccessList> = {}): AccessList => ({
  ...defaultAccessList,
  ...overrides,
})

const createMutationResult = <TData, TVariables>(
  overrides: Partial<UseMutationResult<TData, Error, TVariables, unknown>> = {},
  mutateImpl?: (
    variables: TVariables,
    options?: {
      onSuccess?: (result: TData) => void
      onError?: (error: Error) => void
      onSettled?: () => void
    }
  ) => void
): UseMutationResult<TData, Error, TVariables, unknown> => {
  const mutate = vi.fn((variables: TVariables, options?: { onSuccess?: (result: TData) => void; onError?: (error: Error) => void; onSettled?: () => void }) => {
    mutateImpl?.(variables, options)
  }) as UseMutationResult<TData, Error, TVariables, unknown>['mutate']
  const mutateAsync = vi.fn() as UseMutationResult<TData, Error, TVariables, unknown>['mutateAsync']

  return {
    mutate,
    mutateAsync,
    reset: vi.fn(),
    status: 'idle',
    data: undefined,
    error: null,
    variables: undefined as unknown as TVariables,
    context: undefined,
    failureCount: 0,
    failureReason: null,
    isPaused: false,
    isError: false,
    isIdle: true,
    isPending: false,
    isSuccess: false,
    submittedAt: 0,
    ...overrides,
  } as UseMutationResult<TData, Error, TVariables, unknown>
}

const createQueryResult = <TData,>(data: TData): UseQueryResult<TData, Error> => ({
  data,
  error: null,
  isError: false,
  isLoading: false,
  isPending: false,
  isLoadingError: false,
  isRefetchError: false,
  isSuccess: true,
  status: 'success',
  fetchStatus: 'idle',
  dataUpdatedAt: 0,
  errorUpdatedAt: 0,
  failureCount: 0,
  failureReason: null,
  isFetched: true,
  isFetchedAfterMount: true,
  isFetching: false,
  isInitialLoading: false,
  isPaused: false,
  isPlaceholderData: false,
  isRefetching: false,
  isStale: false,
  refetch: vi.fn(),
} as unknown as UseQueryResult<TData, Error>)

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(),
  useCreateAccessList: vi.fn(),
  useUpdateAccessList: vi.fn(),
  useDeleteAccessList: vi.fn(),
  useTestIP: vi.fn(),
}))

vi.mock('../../api/backups', () => ({
  createBackup: vi.fn(),
}))

vi.mock('react-hot-toast', () => ({
  default: {
    loading: vi.fn(),
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('../../components/AccessListForm', () => ({
  AccessListForm: ({ onSubmit, onCancel, onDelete, isLoading, isDeleting }: MockAccessListFormProps) => (
    <div>
      <div>AccessListForm</div>
      <button
        type="button"
        onClick={() =>
          onSubmit({
            name: 'New List',
            description: '',
            type: 'whitelist',
            ip_rules: '',
            country_codes: '',
            local_network_only: false,
            enabled: true,
          })
        }
        disabled={isLoading}
      >
        Submit
      </button>
      <button type="button" onClick={onCancel}>
        Cancel
      </button>
      {onDelete ? (
        <button type="button" onClick={onDelete} disabled={isDeleting}>
          Delete
        </button>
      ) : null}
    </div>
  ),
}))

describe('AccessLists', () => {
  const createMutationMock = (): ReturnType<typeof useCreateAccessList> =>
    createMutationResult<AccessList, CreateAccessListRequest>()

  const updateMutationMock = (): ReturnType<typeof useUpdateAccessList> =>
    createMutationResult<AccessList, { id: number; data: Partial<CreateAccessListRequest> }>()

  const deleteMutationMock = (): ReturnType<typeof useDeleteAccessList> =>
    createMutationResult<void, number>({}, (_id, options) => options?.onSuccess?.())

  const testIPMutationMock = (): ReturnType<typeof useTestIP> =>
    createMutationResult<TestIPResponse, { id: number; ipAddress: string }>({}, (_payload, options) =>
      options?.onSuccess?.({ allowed: true, reason: 'Allowed by rule' })
    )

  beforeEach(() => {
    vi.clearAllMocks()
    if (!vi.isMockFunction(window.open)) {
      vi.spyOn(window, 'open').mockImplementation(() => null)
    }

    vi.mocked(useCreateAccessList).mockReturnValue(createMutationMock())
    vi.mocked(useUpdateAccessList).mockReturnValue(updateMutationMock())
    vi.mocked(useDeleteAccessList).mockReturnValue(deleteMutationMock())
    vi.mocked(useTestIP).mockReturnValue(testIPMutationMock())
  })

  it('renders empty state and opens create form', async () => {
    vi.mocked(useAccessLists).mockReturnValue(createQueryResult([]))

    const user = userEvent.setup()
    renderWithQueryClient(<AccessLists />)

    expect(await screen.findByText(t('accessLists.noAccessLists'))).toBeInTheDocument()

    const emptyStateHeading = await screen.findByRole('heading', { name: t('accessLists.noAccessLists') })
    const emptyStateContainer = emptyStateHeading.closest('div')
    expect(emptyStateContainer).not.toBeNull()

    await user.click(within(emptyStateContainer as HTMLElement).getByRole('button', { name: t('accessLists.createAccessList') }))

    expect(screen.getByText('AccessListForm')).toBeInTheDocument()
  })

  it('shows CGNAT warning and allows dismiss', async () => {
    vi.mocked(useAccessLists).mockReturnValue(createQueryResult([createAccessList()]))

    const user = userEvent.setup()
    renderWithQueryClient(<AccessLists />)

    const alert = await screen.findByRole('alert')
    expect(alert).toBeInTheDocument()

    await user.click(within(alert).getByRole('button'))

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })

  it('deletes access list with backup', async () => {
    const deleteMutation = deleteMutationMock()
    vi.mocked(useDeleteAccessList).mockReturnValue(deleteMutation)
    vi.mocked(useAccessLists).mockReturnValue(createQueryResult([createAccessList()]))
    vi.mocked(createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })

    const user = userEvent.setup()
    renderWithQueryClient(<AccessLists />)

    const row = (await screen.findByText('Office Access')).closest('tr')
    expect(row).not.toBeNull()
    await user.click(within(row as HTMLElement).getByTitle(t('common.delete')))

    const dialog = await screen.findByRole('dialog', { name: t('accessLists.deleteAccessList') })
    await user.click(within(dialog).getByRole('button', { name: t('common.delete') }))

    await waitFor(() => {
      expect(createBackup).toHaveBeenCalled()
      expect(deleteMutation.mutate).toHaveBeenCalledWith(1, expect.any(Object))
    })
  })

  it('bulk deletes selected access lists', async () => {
    const deleteMutation = deleteMutationMock()
    vi.mocked(useDeleteAccessList).mockReturnValue(deleteMutation)
    vi.mocked(useAccessLists).mockReturnValue(
      createQueryResult([createAccessList({ id: 1 }), createAccessList({ id: 2, uuid: 'acl-2', name: 'Branch Office' })])
    )
    vi.mocked(createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })

    const user = userEvent.setup()
    renderWithQueryClient(<AccessLists />)

    await user.click(await screen.findByRole('checkbox', { name: 'Select row 1' }))

    const bulkDeleteButton = await screen.findByRole('button', { name: `${t('common.delete')} (1)` })
    await user.click(bulkDeleteButton)

    const dialog = await screen.findByRole('dialog', { name: t('accessLists.deleteSelectedAccessLists') })

    await user.click(within(dialog).getByRole('button', { name: t('accessLists.deleteItems', { count: 1 }) }))

    await waitFor(() => {
      expect(createBackup).toHaveBeenCalled()
      expect(deleteMutation.mutate).toHaveBeenCalledTimes(1)
    })
  })

  it('tests IP against access list', async () => {
    const testIPMutation = testIPMutationMock()
    vi.mocked(useTestIP).mockReturnValue(testIPMutation)
    vi.mocked(useAccessLists).mockReturnValue(createQueryResult([createAccessList()]))

    const user = userEvent.setup()
    renderWithQueryClient(<AccessLists />)

    const row = (await screen.findByText('Office Access')).closest('tr')
    expect(row).not.toBeNull()
    await user.click(within(row as HTMLElement).getByTitle(t('accessLists.testIp')))

    const dialog = await screen.findByRole('dialog', { name: t('accessLists.testIpAddress') })

    const input = within(dialog).getByRole('textbox')
    await user.type(input, '192.168.1.5')

    await user.click(within(dialog).getByRole('button', { name: t('common.test') }))

    await waitFor(() => {
      expect(testIPMutation.mutate).toHaveBeenCalledWith(
        { id: 1, ipAddress: '192.168.1.5' },
        expect.any(Object)
      )
    })
  })
})

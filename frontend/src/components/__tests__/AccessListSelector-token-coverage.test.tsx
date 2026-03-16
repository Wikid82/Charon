import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as useAccessListsHook from '../../hooks/useAccessLists';
import AccessListSelector from '../AccessListSelector';

vi.mock('../../hooks/useAccessLists');

vi.mock('../ui/Select', () => {
  const findText = (children: React.ReactNode): string => {
    if (typeof children === 'string') {
      return children;
    }

    if (Array.isArray(children)) {
      return children.map((child) => findText(child)).join(' ');
    }

    if (children && typeof children === 'object' && 'props' in children) {
      const node = children as { props?: { children?: React.ReactNode } };
      return findText(node.props?.children);
    }

    return '';
  };

  const Select = ({ value, onValueChange, children }: { value?: string; onValueChange?: (value: string) => void; children?: React.ReactNode }) => {
    const text = findText(children);
    const isAccessList = text.includes('No Access Control (Public)');

    return (
      <div>
        {isAccessList && (
          <>
            <div data-testid="access-list-select-value">{value}</div>
            <button type="button" onClick={() => onValueChange?.('uuid:acl-uuid-7')}>emit-uuid-token</button>
            <button type="button" onClick={() => onValueChange?.('123')}>emit-numeric-token</button>
            <button type="button" onClick={() => onValueChange?.('custom-token')}>emit-custom-token</button>
          </>
        )}
        {children}
      </div>
    );
  };

  const SelectTrigger = ({ children, ...rest }: React.ComponentProps<'button'>) => <button type="button" {...rest}>{children}</button>;
  const SelectContent = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  const SelectItem = ({ children }: { value: string; children?: React.ReactNode }) => <div>{children}</div>;
  const SelectValue = ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>;

  return {
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
  };
});

describe('AccessListSelector token coverage branches', () => {
  beforeEach(() => {
    vi.mocked(useAccessListsHook.useAccessLists).mockReturnValue({
      data: [
        {
          id: 7,
          uuid: 'acl-uuid-7',
          name: 'ACL Seven',
          description: 'Coverage ACL',
          type: 'whitelist',
          enabled: true,
        },
      ],
    } as unknown as ReturnType<typeof useAccessListsHook.useAccessLists>);
  });

  it('normalizes whitespace and prefixed UUID values in resolver', () => {
    const onChange = vi.fn();
    const { rerender } = render(<AccessListSelector value={'   '} onChange={onChange} />);

    expect(screen.getByTestId('access-list-select-value')).toHaveTextContent('none');

    rerender(<AccessListSelector value={'uuid:acl-uuid-7'} onChange={onChange} />);
    expect(screen.getByTestId('access-list-select-value')).toHaveTextContent('id:7');
  });

  it('maps emitted UUID, numeric, and fallback tokens through handleValueChange', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();

    render(<AccessListSelector value={null} onChange={onChange} />);

    await user.click(screen.getByRole('button', { name: 'emit-uuid-token' }));
    await user.click(screen.getByRole('button', { name: 'emit-numeric-token' }));
    await user.click(screen.getByRole('button', { name: 'emit-custom-token' }));

    expect(onChange).toHaveBeenNthCalledWith(1, 7);
    expect(onChange).toHaveBeenNthCalledWith(2, 123);
    expect(onChange).toHaveBeenNthCalledWith(3, 'custom-token');
  });
});

import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';

import { ProxyGroupBadge } from '../ProxyGroupBadge';

describe('ProxyGroupBadge', () => {
  it('renders group name', () => {
    render(<ProxyGroupBadge group={{ name: 'Production', color: '#6366f1' }} />);
    expect(screen.getByText('Production')).toBeInTheDocument();
  });

  it('renders color dot with correct background color', () => {
    const { container } = render(
      <ProxyGroupBadge group={{ name: 'Staging', color: '#ef4444' }} />,
    );
    const dot = container.querySelector('[aria-hidden="true"]');
    expect(dot).toBeInTheDocument();
    expect((dot as HTMLElement).style.backgroundColor).toBe('rgb(239, 68, 68)');
  });

  it('applies aria-hidden to the color dot', () => {
    const { container } = render(
      <ProxyGroupBadge group={{ name: 'Dev', color: '#10b981' }} />,
    );
    const dot = container.querySelector('[aria-hidden="true"]');
    expect(dot).toHaveAttribute('aria-hidden', 'true');
  });

  it('applies className prop to the wrapper', () => {
    const { container } = render(
      <ProxyGroupBadge group={{ name: 'Test', color: '#6b7280' }} className="custom-class" />,
    );
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper).toHaveClass('custom-class');
  });
});

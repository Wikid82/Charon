import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';

import { SecurityScoreDisplay } from '../SecurityScoreDisplay';

describe('SecurityScoreDisplay', () => {
  const mockBreakdown = {
    hsts: 25,
    csp: 20,
    x_frame_options: 10,
    x_content_type_options: 10,
  };

  const mockSuggestions = [
    'Enable HSTS to enforce HTTPS',
    'Add Content-Security-Policy',
  ];

  it('should render with basic score', () => {
    render(<SecurityScoreDisplay score={85} />);

    expect(screen.getByText('85')).toBeInTheDocument();
    expect(screen.getByText('/100')).toBeInTheDocument();
  });

  it('should render small size variant', () => {
    render(<SecurityScoreDisplay score={50} size="sm" showDetails={false} />);

    expect(screen.getByText('50')).toBeInTheDocument();
    expect(screen.queryByText('Security Score')).not.toBeInTheDocument();
  });

  it('should show correct color for high score', () => {
    const { container } = render(<SecurityScoreDisplay score={85} maxScore={100} />);

    const scoreElement = container.querySelector('.text-green-600');
    expect(scoreElement).toBeInTheDocument();
  });

  it('should show correct color for medium score', () => {
    const { container } = render(<SecurityScoreDisplay score={60} maxScore={100} />);

    const scoreElement = container.querySelector('.text-yellow-600');
    expect(scoreElement).toBeInTheDocument();
  });

  it('should show correct color for low score', () => {
    const { container } = render(<SecurityScoreDisplay score={30} maxScore={100} />);

    const scoreElement = container.querySelector('.text-red-600');
    expect(scoreElement).toBeInTheDocument();
  });

  it('should display breakdown when provided', () => {
    render(
      <SecurityScoreDisplay
        score={65}
        breakdown={mockBreakdown}
        showDetails={true}
      />
    );

    expect(screen.getByText('Score Breakdown by Category')).toBeInTheDocument();
  });

  it('should toggle breakdown visibility', () => {
    render(
      <SecurityScoreDisplay
        score={65}
        breakdown={mockBreakdown}
        showDetails={true}
      />
    );

    const breakdownButton = screen.getByText('Score Breakdown by Category');
    expect(screen.queryByText('HSTS')).not.toBeInTheDocument();

    fireEvent.click(breakdownButton);
    expect(screen.getByText('HSTS')).toBeInTheDocument();
  });

  it('should display suggestions when provided', () => {
    render(
      <SecurityScoreDisplay
        score={50}
        suggestions={mockSuggestions}
        showDetails={true}
      />
    );

    expect(screen.getByText(/Security Suggestions \(2\)/)).toBeInTheDocument();
  });

  it('should toggle suggestions visibility', () => {
    render(
      <SecurityScoreDisplay
        score={50}
        suggestions={mockSuggestions}
        showDetails={true}
      />
    );

    const suggestionsButton = screen.getByText(/Security Suggestions/);
    expect(screen.queryByText('Enable HSTS to enforce HTTPS')).not.toBeInTheDocument();

    fireEvent.click(suggestionsButton);
    expect(screen.getByText('Enable HSTS to enforce HTTPS')).toBeInTheDocument();
  });

  it('should not show details when showDetails is false', () => {
    render(
      <SecurityScoreDisplay
        score={75}
        breakdown={mockBreakdown}
        suggestions={mockSuggestions}
        showDetails={false}
      />
    );

    expect(screen.queryByText('Score Breakdown by Category')).not.toBeInTheDocument();
    expect(screen.queryByText('Security Suggestions')).not.toBeInTheDocument();
  });

  it('should display custom max score', () => {
    render(<SecurityScoreDisplay score={40} maxScore={50} />);

    expect(screen.getByText('40')).toBeInTheDocument();
    expect(screen.getByText('/50')).toBeInTheDocument();
  });

  it('should calculate percentage correctly', () => {
    render(<SecurityScoreDisplay score={75} maxScore={100} />);

    expect(screen.getByText('75%')).toBeInTheDocument();
  });

  it('should render all breakdown categories', () => {
    render(
      <SecurityScoreDisplay
        score={65}
        breakdown={mockBreakdown}
        showDetails={true}
      />
    );

    fireEvent.click(screen.getByText('Score Breakdown by Category'));

    expect(screen.getByText('HSTS')).toBeInTheDocument();
    expect(screen.getByText('Content Security Policy')).toBeInTheDocument();
    expect(screen.getByText('X-Frame-Options')).toBeInTheDocument();
    expect(screen.getByText('X-Content-Type-Options')).toBeInTheDocument();
  });
});

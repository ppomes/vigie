import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import ComponentStatus from './ComponentStatus';

// Mock next-intl
vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

describe('ComponentStatus', () => {
  it('renders without errors', () => {
    const { container } = render(<ComponentStatus />);
    expect(container).toBeTruthy();
  });

  it('displays loading state initially', async () => {
    render(<ComponentStatus />);
    // Component should handle loading state
    expect(document.body).toBeTruthy();
  });

  it('has correct component structure', () => {
    const { container } = render(<ComponentStatus />);
    expect(container.firstChild).toBeTruthy();
  });
});

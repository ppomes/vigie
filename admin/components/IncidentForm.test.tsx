import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import IncidentForm from './IncidentForm';

// Mock next-intl
vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

describe('IncidentForm', () => {
  const mockComponents = [
    { id: 1, name: 'API', status: 'operational' },
    { id: 2, name: 'Website', status: 'operational' },
  ];

  const mockOnSubmit = vi.fn();
  const mockOnCancel = vi.fn();

  it('renders without errors', () => {
    const { container } = render(
      <IncidentForm
        components={mockComponents}
        onSubmit={mockOnSubmit}
        onCancel={mockOnCancel}
      />
    );
    expect(container).toBeTruthy();
  });

  it('contains form element', () => {
    render(
      <IncidentForm
        components={mockComponents}
        onSubmit={mockOnSubmit}
        onCancel={mockOnCancel}
      />
    );
    const forms = document.getElementsByTagName('form');
    expect(forms.length).toBeGreaterThan(0);
  });

  it('has name input field', () => {
    render(
      <IncidentForm
        components={mockComponents}
        onSubmit={mockOnSubmit}
        onCancel={mockOnCancel}
      />
    );
    const inputs = document.querySelectorAll('input');
    expect(inputs.length).toBeGreaterThan(0);
  });

  it('has component selector', () => {
    render(
      <IncidentForm
        components={mockComponents}
        onSubmit={mockOnSubmit}
        onCancel={mockOnCancel}
      />
    );
    const selects = document.querySelectorAll('select');
    expect(selects.length).toBeGreaterThan(0);
  });

  it('has submit button', () => {
    render(
      <IncidentForm
        components={mockComponents}
        onSubmit={mockOnSubmit}
        onCancel={mockOnCancel}
      />
    );
    const buttons = document.querySelectorAll('button');
    expect(buttons.length).toBeGreaterThan(0);
  });
});

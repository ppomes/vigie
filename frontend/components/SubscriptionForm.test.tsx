import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import SubscriptionForm from './SubscriptionForm';

// Mock next-intl
vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

describe('SubscriptionForm', () => {
  it('renders without errors', () => {
    const { container } = render(<SubscriptionForm />);
    expect(container).toBeTruthy();
  });

  it('contains form element', () => {
    render(<SubscriptionForm />);
    const forms = document.getElementsByTagName('form');
    expect(forms.length).toBeGreaterThan(0);
  });

  it('has email input field', () => {
    render(<SubscriptionForm />);
    const inputs = document.querySelectorAll('input[type="email"]');
    expect(inputs.length).toBeGreaterThan(0);
  });

  it('has submit button', () => {
    render(<SubscriptionForm />);
    const buttons = document.querySelectorAll('button[type="submit"]');
    expect(buttons.length).toBeGreaterThan(0);
  });
});

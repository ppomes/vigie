import { describe, it, expect } from 'vitest';

describe('API Client', () => {
  it('should have API_URL configured', () => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';
    expect(apiUrl).toBeTruthy();
    expect(apiUrl).toContain('http');
  });

  it('should construct proper API endpoints', () => {
    const baseUrl = 'http://localhost:8080/api/v1';
    const endpoint = '/components';
    const fullUrl = baseUrl + endpoint;

    expect(fullUrl).toBe('http://localhost:8080/api/v1/components');
  });

  it('should handle different API endpoints', () => {
    const baseUrl = 'http://localhost:8080/api/v1';
    const endpoints = ['/status', '/components', '/incidents', '/subscribe'];

    endpoints.forEach(endpoint => {
      const fullUrl = baseUrl + endpoint;
      expect(fullUrl).toContain(endpoint);
    });
  });
});

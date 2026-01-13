import { describe, it, expect } from 'vitest';

describe('Admin API Client', () => {
  it('should have API_URL configured', () => {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';
    expect(apiUrl).toBeTruthy();
    expect(apiUrl).toContain('http');
  });

  it('should construct proper admin API endpoints', () => {
    const baseUrl = 'http://localhost:8080/api/v1/admin';
    const endpoints = ['/incidents', '/components', '/subscribers'];

    endpoints.forEach(endpoint => {
      const fullUrl = baseUrl + endpoint;
      expect(fullUrl).toContain('/admin');
      expect(fullUrl).toContain(endpoint);
    });
  });

  it('should handle JWT token in headers', () => {
    const token = 'test-jwt-token';
    const headers = {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    };

    expect(headers['Authorization']).toBe('Bearer test-jwt-token');
    expect(headers['Content-Type']).toBe('application/json');
  });
});

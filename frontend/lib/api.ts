// API client for frontend application
// Uses NEXT_PUBLIC_API_URL environment variable to connect to the backend API

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

// Type definitions for API responses
export interface Component {
  id: number;
  name: string;
  status: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage';
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface Incident {
  id: number;
  name: string;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  message: string;
  component_id: number;
  component?: Component;
  created_at: string;
  updated_at: string;
}

export interface SystemStatus {
  status: 'operational' | 'degraded' | 'partial_outage' | 'major_outage';
  message: string;
}

export interface SubscribeRequest {
  email: string;
}

export interface SubscribeResponse {
  message: string;
  subscriber_id?: number;
}

export interface ApiError {
  error: string;
  details?: string;
}

// Generic fetch wrapper with error handling
async function fetchAPI<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  const defaultHeaders: HeadersInit = {
    'Content-Type': 'application/json',
  };

  // Add Accept-Language header if locale is set
  if (typeof window !== 'undefined') {
    const locale = window.location.pathname.split('/')[1];
    if (locale === 'en' || locale === 'fr') {
      defaultHeaders['Accept-Language'] = locale;
    }
  }

  const config: RequestInit = {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
  };

  try {
    const response = await fetch(url, config);

    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
      }));
      throw new Error(error.error || `Request failed with status ${response.status}`);
    }

    return await response.json();
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error('An unexpected error occurred');
  }
}

// API Client functions

/**
 * Get overall system status
 */
export async function getStatus(): Promise<SystemStatus> {
  return fetchAPI<SystemStatus>('/status');
}

/**
 * Get all components with their current status
 */
export async function getComponents(): Promise<Component[]> {
  return fetchAPI<Component[]>('/components');
}

/**
 * Get all incidents
 * @param includeResolved - Optional flag to include resolved incidents (default: false)
 */
export async function getIncidents(includeResolved = false): Promise<Incident[]> {
  const query = includeResolved ? '?include_resolved=true' : '';
  return fetchAPI<Incident[]>(`/incidents${query}`);
}

/**
 * Subscribe to status updates
 * @param email - Email address to subscribe
 */
export async function subscribe(email: string): Promise<SubscribeResponse> {
  return fetchAPI<SubscribeResponse>('/subscribe', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

/**
 * Confirm subscription
 * @param token - Confirmation token from email
 */
export async function confirmSubscription(token: string): Promise<{ message: string }> {
  return fetchAPI<{ message: string }>(`/subscribe/confirm/${token}`, {
    method: 'GET',
  });
}

/**
 * Unsubscribe from status updates
 * @param token - Unsubscribe token from email
 */
export async function unsubscribe(token: string): Promise<{ message: string }> {
  return fetchAPI<{ message: string }>(`/unsubscribe/${token}`, {
    method: 'GET',
  });
}

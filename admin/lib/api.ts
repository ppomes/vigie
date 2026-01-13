// API client for admin application
// Uses NEXT_PUBLIC_API_URL environment variable to connect to the backend API
// Includes JWT authentication for protected admin endpoints

import { getToken, setAuthData, clearAuthData, type AuthData } from './auth';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

// Type definitions for API responses

export interface Component {
  id: number;
  name: string;
  status: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage';
  order: number;
  translations?: Translation[];
  created_at: string;
  updated_at: string;
}

export interface Incident {
  id: number;
  name: string;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  message?: string;
  component_id: number;
  component?: Component;
  translations?: Translation[];
  created_at: string;
  updated_at: string;
}

export interface Subscriber {
  id: number;
  email: string;
  confirmed: boolean;
  created_at: string;
  updated_at: string;
}

export interface Translation {
  id: number;
  entity_id: number;
  entity: string;
  locale: 'en' | 'fr';
  field: string;
  value: string;
}

export interface TranslationRequest {
  locale: 'en' | 'fr';
  field: string;
  value: string;
}

// Request types
export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: {
    id: number;
    username: string;
    email: string;
  };
}

export interface CreateComponentRequest {
  name: string;
  status: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage';
  order?: number;
  translations?: TranslationRequest[];
}

export interface UpdateComponentRequest {
  name?: string;
  status?: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage';
  order?: number;
  translations?: TranslationRequest[];
}

export interface CreateIncidentRequest {
  name: string;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  component_id: number;
  translations?: TranslationRequest[];
}

export interface UpdateIncidentRequest {
  name?: string;
  status?: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  component_id?: number;
  translations?: TranslationRequest[];
}

export interface ApiError {
  error: string;
  details?: string;
}

// Generic fetch wrapper with error handling and JWT authentication
async function fetchAPI<T>(
  endpoint: string,
  options: RequestInit = {},
  requireAuth = true
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  const defaultHeaders: HeadersInit = {
    'Content-Type': 'application/json',
  };

  // Add JWT token for authenticated requests
  if (requireAuth) {
    const token = getToken();
    if (!token) {
      throw new Error('Authentication required. Please login.');
    }
    defaultHeaders['Authorization'] = `Bearer ${token}`;
  }

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

    // Handle 401 Unauthorized - clear auth data and redirect to login
    if (response.status === 401) {
      clearAuthData();
      if (typeof window !== 'undefined') {
        window.location.href = '/login';
      }
      throw new Error('Session expired. Please login again.');
    }

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

// Authentication API

/**
 * Login to admin interface
 * @param username - Admin username
 * @param password - Admin password
 * @returns Authentication data with JWT token
 */
export async function login(username: string, password: string): Promise<AuthData> {
  const response = await fetchAPI<LoginResponse>(
    '/admin/login',
    {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    },
    false // Don't require auth for login endpoint
  );

  const authData: AuthData = {
    token: response.token,
    expiresAt: response.expires_at,
    user: response.user,
  };

  // Store auth data in localStorage
  setAuthData(authData);

  return authData;
}

// Component API

/**
 * Get all components
 * @returns List of all components with translations
 */
export async function getComponents(): Promise<Component[]> {
  return fetchAPI<Component[]>('/admin/components');
}

/**
 * Create a new component
 * @param data - Component data
 * @returns Created component
 */
export async function createComponent(
  data: CreateComponentRequest
): Promise<{ message: string; component: Component }> {
  return fetchAPI('/admin/components', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * Update an existing component
 * @param id - Component ID
 * @param data - Updated component data
 * @returns Updated component
 */
export async function updateComponent(
  id: number,
  data: UpdateComponentRequest
): Promise<{ message: string; component: Component }> {
  return fetchAPI(`/admin/components/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * Delete a component
 * @param id - Component ID
 * @returns Success message
 */
export async function deleteComponent(id: number): Promise<{ message: string }> {
  return fetchAPI(`/admin/components/${id}`, {
    method: 'DELETE',
  });
}

// Incident API

/**
 * Get all incidents
 * @returns List of all incidents with translations and component info
 */
export async function getIncidents(): Promise<Incident[]> {
  return fetchAPI<Incident[]>('/admin/incidents');
}

/**
 * Create a new incident
 * @param data - Incident data
 * @returns Created incident
 */
export async function createIncident(
  data: CreateIncidentRequest
): Promise<{ message: string; incident: Incident }> {
  return fetchAPI('/admin/incidents', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * Update an existing incident
 * @param id - Incident ID
 * @param data - Updated incident data
 * @returns Updated incident
 */
export async function updateIncident(
  id: number,
  data: UpdateIncidentRequest
): Promise<{ message: string; incident: Incident }> {
  return fetchAPI(`/admin/incidents/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * Delete an incident
 * @param id - Incident ID
 * @returns Success message
 */
export async function deleteIncident(id: number): Promise<{ message: string }> {
  return fetchAPI(`/admin/incidents/${id}`, {
    method: 'DELETE',
  });
}

// Subscriber API

/**
 * Get all subscribers
 * @returns List of all subscribers
 */
export async function getSubscribers(): Promise<Subscriber[]> {
  return fetchAPI<Subscriber[]>('/admin/subscribers');
}

/**
 * Delete a subscriber
 * @param id - Subscriber ID
 * @returns Success message
 */
export async function deleteSubscriber(id: number): Promise<{ message: string }> {
  return fetchAPI(`/admin/subscribers/${id}`, {
    method: 'DELETE',
  });
}

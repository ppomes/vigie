const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

export interface Component {
  id: number;
  name: string;
  status: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage' | 'under_maintenance';
  description?: string;
  translations?: Translation[];
}

export interface IncidentUpdate {
  id: number;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  translations?: Translation[];
  created_at: string;
}

export interface Incident {
  id: number;
  name: string;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  component_id: number;
  component?: Component;
  translations?: Translation[];
  // Timeline, newest first
  updates?: IncidentUpdate[];
  created_at: string;
  updated_at: string;
}

export interface Translation {
  locale: string;
  field: string;
  value: string;
}

export interface SystemStatus {
  status: string;
  message: string;
}

export interface Maintenance {
  id: number;
  name: string;
  message: string;
  status: 'scheduled' | 'in_progress' | 'completed';
  component_id: number;
  component: string;
  start_at: string;
  end_at: string;
}

async function fetchAPI<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

export async function getComponents(locale: string = 'en'): Promise<Component[]> {
  const response = await fetchAPI<{ components: Component[] }>(`/components?locale=${locale}`);
  return response.components || [];
}

export async function getIncidents(locale: string = 'en'): Promise<Incident[]> {
  const response = await fetchAPI<{ incidents: Incident[] }>(`/incidents?locale=${locale}`);
  return response.incidents || [];
}

export async function getStatus(): Promise<SystemStatus> {
  return fetchAPI<SystemStatus>('/status');
}

export async function subscribe(email: string, locale: string = 'en'): Promise<{ message: string }> {
  return fetchAPI<{ message: string }>('/subscribe', {
    method: 'POST',
    body: JSON.stringify({ email, locale }),
  });
}

export async function getTheme(): Promise<string> {
  const response = await fetchAPI<{ theme: string }>('/settings/theme');
  return response.theme || 'default';
}

export interface BrandingSettings {
  title_en: string;
  title_fr: string;
  logo: string;
}

export async function getBranding(): Promise<BrandingSettings> {
  return fetchAPI<BrandingSettings>('/settings/branding');
}

export async function getMaintenances(locale: string = 'en'): Promise<Maintenance[]> {
  const response = await fetchAPI<{ maintenances: Maintenance[] }>(`/maintenances?locale=${locale}`);
  return response.maintenances || [];
}

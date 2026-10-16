const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

// Public API fetch (no auth required, no redirect on 401)
async function fetchPublicAPI<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_URL}${endpoint}`, {
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

async function fetchAPI<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('token');
  const response = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  if (response.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

export interface LoginResponse {
  token: string;
  user: { id: number; username: string; email: string; role: 'admin' | 'viewer' };
  force_password_change: boolean;
}

export interface User {
  id: number;
  username: string;
  email: string;
  role: 'admin' | 'viewer';
  active: boolean;
  last_login?: string;
  created_at: string;
}

export interface APIKey {
  id: number;
  name: string;
  prefix: string;
  user_id: number;
  user?: User;
  last_used_at?: string;
  revoked_at?: string;
  created_at: string;
}

export interface AuditLog {
  id: number;
  user_id?: number;
  username: string;
  action: string;
  resource?: string;
  resource_id?: number;
  details?: string;
  ip_address: string;
  user_agent?: string;
  success: boolean;
  created_at: string;
}

export interface Component {
  id: number;
  name: string;
  status: string;
  order: number;
}

export interface Translation {
  id?: number;
  locale: string;
  field: string;
  value: string;
}

// Fields accepted when creating or updating an incident. component_status,
// when set, changes the status of the affected component; when omitted, the
// component status is left as it is.
export type IncidentInput = Partial<Incident> & { component_status?: string };

export interface Incident {
  id: number;
  name: string;
  status: string;
  component_id: number;
  translations?: Translation[];
  created_at: string;
}

export interface Subscriber {
  id: number;
  email: string;
  locale: string;
  confirmed: boolean;
  created_at: string;
}

export interface Setting {
  id: number;
  key: string;
  value: string;
  updated_at: string;
}

export interface EmailConfig {
  provider: string;
  from_address: string;
  from_name: string;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  smtp_tls: boolean;
  sendgrid_api_key: string;
  mailgun_api_key: string;
  mailgun_domain: string;
  aws_region: string;
  aws_access_key: string;
  aws_secret_key: string;
}

export interface BrandingConfig {
  title_en: string;
  title_fr: string;
  logo: string;
}

export interface TemplateVariable {
  name: string;
  label_en: string;
  label_fr: string;
  type: 'text' | 'number' | 'datetime' | 'select';
  options?: string[];
  default?: string;
  required: boolean;
}

export interface IncidentTemplate {
  id: number;
  name: string;
  default_status: string;
  default_component_id?: number;
  name_en: string;
  name_fr: string;
  message_en: string;
  message_fr: string;
  variables: string;
  parsed_variables?: TemplateVariable[];
  created_at: string;
}

export interface TemplatePreview {
  name_en: string;
  name_fr: string;
  message_en: string;
  message_fr: string;
}

export const api = {
  login: (username: string, password: string) =>
    fetchPublicAPI<LoginResponse>('/admin/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  forgotPassword: (email: string) =>
    fetchPublicAPI<{ message: string }>('/forgot-password', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  resetPassword: (token: string, password: string) =>
    fetchPublicAPI<{ message: string }>('/reset-password', {
      method: 'POST',
      body: JSON.stringify({ token, password }),
    }),

  getComponents: () => fetchAPI<{ components: Component[] }>('/admin/components').then(r => r.components || []),
  createComponent: (data: Partial<Component>) =>
    fetchAPI<Component>('/admin/components', { method: 'POST', body: JSON.stringify(data) }),
  updateComponent: (id: number, data: Partial<Component>) =>
    fetchAPI<Component>(`/admin/components/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteComponent: (id: number) =>
    fetchAPI<void>(`/admin/components/${id}`, { method: 'DELETE' }),

  getIncidents: () => fetchAPI<{ incidents: Incident[] }>('/admin/incidents').then(r => r.incidents || []),
  createIncident: (data: IncidentInput) =>
    fetchAPI<Incident>('/admin/incidents', { method: 'POST', body: JSON.stringify(data) }),
  updateIncident: (id: number, data: IncidentInput) =>
    fetchAPI<Incident>(`/admin/incidents/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteIncident: (id: number) =>
    fetchAPI<void>(`/admin/incidents/${id}`, { method: 'DELETE' }),

  getSubscribers: () => fetchAPI<{ subscribers: Subscriber[] }>('/admin/subscribers').then(r => r.subscribers || []),
  deleteSubscriber: (id: number) =>
    fetchAPI<void>(`/admin/subscribers/${id}`, { method: 'DELETE' }),

  getSettings: () => fetchAPI<{ settings: Setting[] }>('/admin/settings').then(r => r.settings || []),
  updateSetting: (key: string, value: string) =>
    fetchAPI<Setting>(`/admin/settings/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }),

  getEmailConfig: () => fetchAPI<{ config: EmailConfig }>('/admin/email').then(r => r.config),
  updateEmailConfig: (config: Partial<EmailConfig>) =>
    fetchAPI<{ message: string; config: EmailConfig }>('/admin/email', { method: 'PUT', body: JSON.stringify(config) }),
  testEmailConfig: (email: string) =>
    fetchAPI<{ message: string }>('/admin/email/test', { method: 'POST', body: JSON.stringify({ email }) }),

  // Branding settings
  getBranding: () => fetchAPI<BrandingConfig>('/settings/branding'),
  updateBranding: (config: BrandingConfig) =>
    fetchAPI<{ message: string; config: BrandingConfig }>('/admin/settings/branding', { method: 'PUT', body: JSON.stringify(config) }),

  // Incident templates
  getTemplates: () => fetchAPI<{ templates: IncidentTemplate[] }>('/admin/templates').then(r => r.templates || []),
  getTemplate: (id: number) => fetchAPI<{ template: IncidentTemplate; parsed_variables: TemplateVariable[] }>(`/admin/templates/${id}`),
  createTemplate: (data: Omit<Partial<IncidentTemplate>, 'variables'> & { variables?: TemplateVariable[] }) =>
    fetchAPI<{ template: IncidentTemplate }>('/admin/templates', { method: 'POST', body: JSON.stringify(data) }),
  updateTemplate: (id: number, data: Omit<Partial<IncidentTemplate>, 'variables'> & { variables?: TemplateVariable[] }) =>
    fetchAPI<{ template: IncidentTemplate }>(`/admin/templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTemplate: (id: number) =>
    fetchAPI<void>(`/admin/templates/${id}`, { method: 'DELETE' }),
  previewTemplate: (id: number, variableValues: Record<string, string>) =>
    fetchAPI<{ preview: TemplatePreview }>(`/admin/templates/${id}/preview`, { method: 'POST', body: JSON.stringify({ variable_values: variableValues }) }),

  // Notification queue monitoring
  getNotificationStats: () => fetchAPI<{ stats: NotificationQueueStats }>('/admin/notifications/stats').then(r => r.stats),
  getRecentNotifications: (limit = 50) => fetchAPI<{ notifications: QueuedNotification[] }>(`/admin/notifications/recent?limit=${limit}`).then(r => r.notifications || []),
  retryFailedNotifications: () => fetchAPI<{ message: string; count: number }>('/admin/notifications/retry', { method: 'POST' }),

  // Maintenances
  getMaintenances: () => fetchAPI<{ maintenances: Maintenance[] }>('/admin/maintenances').then(r => r.maintenances || []),
  createMaintenance: (data: Partial<Maintenance> & { translations?: { locale: string; field: string; value: string }[] }) =>
    fetchAPI<{ maintenance: Maintenance }>('/admin/maintenances', { method: 'POST', body: JSON.stringify(data) }),
  updateMaintenance: (id: number, data: Partial<Maintenance> & { translations?: { locale: string; field: string; value: string }[] }) =>
    fetchAPI<{ maintenance: Maintenance }>(`/admin/maintenances/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteMaintenance: (id: number) =>
    fetchAPI<void>(`/admin/maintenances/${id}`, { method: 'DELETE' }),

  // Maintenance templates
  getMaintenanceTemplates: () => fetchAPI<{ templates: MaintenanceTemplate[] }>('/admin/maintenance-templates').then(r => r.templates || []),
  getMaintenanceTemplate: (id: number) => fetchAPI<{ template: MaintenanceTemplate; parsed_variables: TemplateVariable[] }>(`/admin/maintenance-templates/${id}`),
  createMaintenanceTemplate: (data: Omit<Partial<MaintenanceTemplate>, 'variables'> & { variables?: TemplateVariable[] }) =>
    fetchAPI<{ template: MaintenanceTemplate }>('/admin/maintenance-templates', { method: 'POST', body: JSON.stringify(data) }),
  updateMaintenanceTemplate: (id: number, data: Omit<Partial<MaintenanceTemplate>, 'variables'> & { variables?: TemplateVariable[] }) =>
    fetchAPI<{ template: MaintenanceTemplate }>(`/admin/maintenance-templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteMaintenanceTemplate: (id: number) =>
    fetchAPI<void>(`/admin/maintenance-templates/${id}`, { method: 'DELETE' }),
  previewMaintenanceTemplate: (id: number, variableValues: Record<string, string>) =>
    fetchAPI<{ preview: TemplatePreview }>(`/admin/maintenance-templates/${id}/preview`, { method: 'POST', body: JSON.stringify({ variable_values: variableValues }) }),

  // Current user
  getCurrentUser: () => fetchAPI<{ user: User }>('/admin/me').then(r => r.user),
  changeOwnPassword: (oldPassword: string, newPassword: string) =>
    fetchAPI<{ message: string }>('/admin/me/password', {
      method: 'PUT',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),

  // User management (admin only)
  getUsers: () => fetchAPI<{ users: User[] }>('/admin/users').then(r => r.users || []),
  getUser: (id: number) => fetchAPI<{ user: User }>(`/admin/users/${id}`).then(r => r.user),
  createUser: (data: { username: string; email: string; password: string; role: 'admin' | 'viewer' }) =>
    fetchAPI<{ user: User }>('/admin/users', { method: 'POST', body: JSON.stringify(data) }).then(r => r.user),
  updateUser: (id: number, data: { email?: string; role?: 'admin' | 'viewer'; active?: boolean }) =>
    fetchAPI<{ user: User }>(`/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }).then(r => r.user),
  deleteUser: (id: number) =>
    fetchAPI<{ message: string }>(`/admin/users/${id}`, { method: 'DELETE' }),
  resetUserPassword: (id: number, password: string) =>
    fetchAPI<{ message: string }>(`/admin/users/${id}/password`, { method: 'PUT', body: JSON.stringify({ password }) }),

  // API keys (admin only). The raw key is only returned on creation.
  getAPIKeys: () => fetchAPI<{ api_keys: APIKey[] }>('/admin/api-keys').then(r => r.api_keys || []),
  createAPIKey: (data: { name: string; user_id?: number }) =>
    fetchAPI<{ api_key: APIKey; key: string }>('/admin/api-keys', { method: 'POST', body: JSON.stringify(data) }),
  revokeAPIKey: (id: number) =>
    fetchAPI<{ message: string }>(`/admin/api-keys/${id}`, { method: 'DELETE' }),

  // Audit logs (admin only)
  getAuditLogs: (params?: { user_id?: number; action?: string; resource?: string; from?: string; to?: string; page?: number; limit?: number }) => {
    const query = new URLSearchParams();
    if (params?.user_id) query.set('user_id', params.user_id.toString());
    if (params?.action) query.set('action', params.action);
    if (params?.resource) query.set('resource', params.resource);
    if (params?.from) query.set('from', params.from);
    if (params?.to) query.set('to', params.to);
    if (params?.page) query.set('page', params.page.toString());
    if (params?.limit) query.set('limit', params.limit.toString());
    const queryStr = query.toString();
    return fetchAPI<{ logs: AuditLog[]; total: number; page: number; limit: number }>(`/admin/audit${queryStr ? '?' + queryStr : ''}`);
  },
  exportAuditLogs: async (params?: { user_id?: number; action?: string; resource?: string; from?: string; to?: string }) => {
    const query = new URLSearchParams();
    if (params?.user_id) query.set('user_id', params.user_id.toString());
    if (params?.action) query.set('action', params.action);
    if (params?.resource) query.set('resource', params.resource);
    if (params?.from) query.set('from', params.from);
    if (params?.to) query.set('to', params.to);
    const queryStr = query.toString();
    const token = localStorage.getItem('token');
    const response = await fetch(`${API_URL}/admin/audit/export${queryStr ? '?' + queryStr : ''}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    return response.blob();
  },
};

export interface NotificationQueueStats {
  pending: number;
  processing: number;
  sent: number;
  failed: number;
  total: number;
}

export interface QueuedNotification {
  id: number;
  incident_id: number;
  subscriber_id: number;
  email: string;
  locale: string;
  subject: string;
  status: 'pending' | 'processing' | 'sent' | 'failed';
  attempts: number;
  max_attempts: number;
  last_error?: string;
  created_at: string;
  processed_at?: string;
  sent_at?: string;
}

export interface Maintenance {
  id: number;
  name: string;
  status: 'scheduled' | 'in_progress' | 'completed';
  component_id: number;
  component?: Component;
  start_at: string;
  end_at: string;
  translations?: Translation[];
  created_at: string;
}

export interface MaintenanceTemplate {
  id: number;
  name: string;
  default_component_id?: number;
  name_en: string;
  name_fr: string;
  message_en: string;
  message_fr: string;
  variables: string;
  parsed_variables?: TemplateVariable[];
  created_at: string;
}

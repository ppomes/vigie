export type ThemeLayout = 'centered' | 'sidebar' | 'full-width';

export interface ThemeColors {
  background: string;
  card: string;
  cardBorder: string;
  text: string;
  textMuted: string;
  primary: string;
  primaryHover: string;
  header: string;
  headerText: string;
  // Status colors
  operational: string;
  degraded: string;
  partialOutage: string;
  majorOutage: string;
  // Incident status colors
  investigating: string;
  identified: string;
  monitoring: string;
  resolved: string;
}

export interface ThemeStyles {
  cardRadius: string;
  shadow: string;
  headerStyle: string;
}

export interface Theme {
  id: string;
  name: string;
  description: string;
  layout: ThemeLayout;
  colors: ThemeColors;
  styles: ThemeStyles;
}

export const themes: Record<string, Theme> = {
  default: {
    id: 'default',
    name: 'Default',
    description: 'Clean and minimal design',
    layout: 'centered',
    colors: {
      background: '#f9fafb',
      card: '#ffffff',
      cardBorder: '#e5e7eb',
      text: '#111827',
      textMuted: '#6b7280',
      primary: '#3b82f6',
      primaryHover: '#2563eb',
      header: '#ffffff',
      headerText: '#111827',
      operational: '#22c55e',
      degraded: '#eab308',
      partialOutage: '#f97316',
      majorOutage: '#ef4444',
      investigating: '#ef4444',
      identified: '#f97316',
      monitoring: '#eab308',
      resolved: '#22c55e',
    },
    styles: {
      cardRadius: '0.5rem',
      shadow: '0 1px 3px 0 rgb(0 0 0 / 0.1)',
      headerStyle: 'border-b border-gray-200',
    },
  },
  dark: {
    id: 'dark',
    name: 'Dark',
    description: 'Dark mode with muted colors',
    layout: 'centered',
    colors: {
      background: '#0f172a',
      card: '#1e293b',
      cardBorder: '#334155',
      text: '#f1f5f9',
      textMuted: '#94a3b8',
      primary: '#60a5fa',
      primaryHover: '#3b82f6',
      header: '#1e293b',
      headerText: '#f1f5f9',
      operational: '#4ade80',
      degraded: '#facc15',
      partialOutage: '#fb923c',
      majorOutage: '#f87171',
      investigating: '#f87171',
      identified: '#fb923c',
      monitoring: '#facc15',
      resolved: '#4ade80',
    },
    styles: {
      cardRadius: '0.5rem',
      shadow: '0 4px 6px -1px rgb(0 0 0 / 0.3)',
      headerStyle: 'border-b border-slate-700',
    },
  },
  corporate: {
    id: 'corporate',
    name: 'Corporate',
    description: 'Professional sidebar layout',
    layout: 'sidebar',
    colors: {
      background: '#f8fafc',
      card: '#ffffff',
      cardBorder: '#e2e8f0',
      text: '#1e293b',
      textMuted: '#64748b',
      primary: '#0369a1',
      primaryHover: '#075985',
      header: '#0c4a6e',
      headerText: '#ffffff',
      operational: '#16a34a',
      degraded: '#ca8a04',
      partialOutage: '#ea580c',
      majorOutage: '#dc2626',
      investigating: '#dc2626',
      identified: '#ea580c',
      monitoring: '#ca8a04',
      resolved: '#16a34a',
    },
    styles: {
      cardRadius: '0.25rem',
      shadow: '0 1px 2px 0 rgb(0 0 0 / 0.05)',
      headerStyle: 'shadow-md',
    },
  },
  modern: {
    id: 'modern',
    name: 'Modern',
    description: 'Gradient accents and rounded cards',
    layout: 'full-width',
    colors: {
      background: '#fafafa',
      card: '#ffffff',
      cardBorder: '#e4e4e7',
      text: '#18181b',
      textMuted: '#71717a',
      primary: '#8b5cf6',
      primaryHover: '#7c3aed',
      header: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      headerText: '#ffffff',
      operational: '#10b981',
      degraded: '#f59e0b',
      partialOutage: '#f97316',
      majorOutage: '#ef4444',
      investigating: '#ef4444',
      identified: '#f97316',
      monitoring: '#f59e0b',
      resolved: '#10b981',
    },
    styles: {
      cardRadius: '1rem',
      shadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
      headerStyle: 'shadow-lg',
    },
  },
};

export function getThemeById(id: string): Theme {
  return themes[id] || themes.default;
}

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { getComponents, getComponentGroups, getIncidents, getMaintenances, getStatus, subscribe, Component, ComponentGroup, Incident, Maintenance, SystemStatus } from './lib/api';
import { useTheme } from './themes/ThemeContext';
import { useBranding } from './themes/BrandingContext';

const statusClasses: Record<string, string> = {
  operational: 'status-operational',
  degraded_performance: 'status-degraded',
  partial_outage: 'status-partial-outage',
  major_outage: 'status-major-outage',
  under_maintenance: 'status-maintenance',
};

const incidentStatusClasses: Record<string, string> = {
  investigating: 'status-investigating',
  identified: 'status-identified',
  monitoring: 'status-monitoring',
  resolved: 'status-resolved',
};

const maintenanceStatusClasses: Record<string, string> = {
  scheduled: 'status-scheduled',
  in_progress: 'status-maintenance',
  completed: 'status-operational',
};

function LanguageSwitcher() {
  const { i18n, t } = useTranslation();

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm text-muted">{t('language.switchTo')}:</span>
      <button
        onClick={() => i18n.changeLanguage('en')}
        className={`px-3 py-1 text-sm rounded ${i18n.language === 'en' ? 'btn-primary' : 'bg-gray-200 hover:bg-gray-300'}`}
        style={i18n.language === 'en' ? {} : { color: 'var(--color-text)' }}
      >
        EN
      </button>
      <button
        onClick={() => i18n.changeLanguage('fr')}
        className={`px-3 py-1 text-sm rounded ${i18n.language === 'fr' ? 'btn-primary' : 'bg-gray-200 hover:bg-gray-300'}`}
        style={i18n.language === 'fr' ? {} : { color: 'var(--color-text)' }}
      >
        FR
      </button>
    </div>
  );
}

function StatusBanner({ centered = false }: { centered?: boolean }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<SystemStatus | null>(null);

  useEffect(() => {
    getStatus().then(setStatus).catch(console.error);
  }, []);

  const isOperational = status?.status === 'operational';
  const statusClass = isOperational ? 'status-operational' : 'status-major-outage';
  const statusMessage = isOperational
    ? t('status.allSystemsOperational')
    : t('status.systemsExperiencingIssues');

  return (
    <div className={`flex items-center gap-3 ${centered ? 'justify-center' : ''}`}>
      <div className={`w-4 h-4 ${statusClass} rounded-full`} />
      <span className="text-xl font-semibold">{statusMessage}</span>
    </div>
  );
}

// Worst status first, to summarise a collapsed group.
const statusSeverity: Record<string, number> = {
  major_outage: 4,
  partial_outage: 3,
  degraded_performance: 2,
  under_maintenance: 1,
  operational: 0,
};

function worstStatus(components: Component[]): Component['status'] {
  return components.reduce<Component['status']>(
    (worst, c) => ((statusSeverity[c.status] ?? 0) > (statusSeverity[worst] ?? 0) ? c.status : worst),
    'operational',
  );
}

function groupName(group: ComponentGroup, locale: string): string {
  return group.translations?.find(t => t.field === 'name' && t.locale === locale)?.value || group.name;
}

function initiallyCollapsed(group: ComponentGroup): boolean {
  if (group.collapse === 'always') return true;
  if (group.collapse === 'operational') return (group.components || []).every(c => c.status === 'operational');
  return false;
}

// A component's name in the reader's language, falling back to its name.
function componentName(component: Component, locale: string): string {
  return component.translations?.find(tr => tr.field === 'name' && tr.locale === locale)?.value || component.name;
}

// A component name, linked to the service when the component has a link.
function ComponentName({ component, name, className }: { component: Component; name: string; className: string }) {
  if (component.link) {
    return (
      <a href={component.link} target="_blank" rel="noopener noreferrer" className={`${className} hover:underline`}>
        {name}
      </a>
    );
  }
  return <span className={className}>{name}</span>;
}

function ComponentRow({ component, asSidebar }: { component: Component; asSidebar: boolean }) {
  const { t, i18n } = useTranslation();
  const name = componentName(component, i18n.language);
  if (asSidebar) {
    return (
      <div className="flex items-center gap-2 py-2">
        <div className={`w-2 h-2 rounded-full ${statusClasses[component.status]}`} />
        <ComponentName component={component} name={name} className="text-sm" />
      </div>
    );
  }
  return (
    <div className="flex items-center justify-between py-3 border-b last:border-0" style={{ borderColor: 'var(--color-card-border)' }}>
      <div className="flex items-center gap-3">
        <div className={`w-3 h-3 rounded-full ${statusClasses[component.status]}`} />
        <ComponentName component={component} name={name} className="font-medium" />
      </div>
      <span className="text-sm text-muted">{t(`componentStatus.${component.status}`)}</span>
    </div>
  );
}

function GroupBlock({ group, asSidebar }: { group: ComponentGroup; asSidebar: boolean }) {
  const { t, i18n } = useTranslation();
  const components = group.components || [];
  const [collapsed, setCollapsed] = useState(() => initiallyCollapsed(group));
  const worst = worstStatus(components);

  return (
    <div className={asSidebar ? 'mt-3' : 'mt-4'}>
      <button
        type="button"
        onClick={() => setCollapsed(!collapsed)}
        aria-expanded={!collapsed}
        className="w-full flex items-center justify-between py-2 text-left"
      >
        <span className="flex items-center gap-2">
          <span aria-hidden="true">{collapsed ? '▸' : '▾'}</span>
          <span className={asSidebar ? 'text-sm font-semibold' : 'font-semibold'}>{groupName(group, i18n.language)}</span>
        </span>
        {collapsed && !asSidebar && <span className="text-sm text-muted">{t(`componentStatus.${worst}`)}</span>}
        {collapsed && asSidebar && <span className={`w-2 h-2 rounded-full ${statusClasses[worst]}`} />}
      </button>
      {!collapsed && (
        <div className={asSidebar ? 'pl-4' : 'pl-6'}>
          {components.map(c => <ComponentRow key={c.id} component={c} asSidebar={asSidebar} />)}
        </div>
      )}
    </div>
  );
}

function ComponentsList({ asSidebar = false }: { asSidebar?: boolean }) {
  const { t, i18n } = useTranslation();
  const [components, setComponents] = useState<Component[]>([]);
  const [groups, setGroups] = useState<ComponentGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([getComponents(i18n.language), getComponentGroups(i18n.language)])
      .then(([c, g]) => {
        setComponents(c);
        setGroups(g);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [i18n.language]);

  if (loading) return <p className="text-muted">{t('common.loading')}</p>;
  if (error) return <p className="text-red-600">{error}</p>;
  if (components.length === 0) return <p className="text-muted">{t('components.noComponents')}</p>;

  // Components outside any group come first, then each group in order.
  const ungrouped = components.filter(c => !c.group_id);
  const visibleGroups = groups.filter(g => (g.components || []).length > 0);

  return (
    <div className={asSidebar ? 'space-y-2' : 'space-y-3'}>
      {ungrouped.map(c => <ComponentRow key={c.id} component={c} asSidebar={asSidebar} />)}
      {visibleGroups.map(g => <GroupBlock key={g.id} group={g} asSidebar={asSidebar} />)}
    </div>
  );
}

function IncidentsList() {
  const { t, i18n } = useTranslation();
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getIncidents(i18n.language)
      .then(setIncidents)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [i18n.language]);

  // Helper to get localized translation with fallback
  const getTranslation = (incident: Incident, field: string): string => {
    const locale = i18n.language;
    // Try current locale
    const translation = incident.translations?.find(t => t.field === field && t.locale === locale);
    if (translation?.value) return translation.value;
    // Fall back to English
    const enTranslation = incident.translations?.find(t => t.field === field && t.locale === 'en');
    if (enTranslation?.value) return enTranslation.value;
    // Fall back to default (for name only)
    return field === 'name' ? incident.name : '';
  };

  if (loading) return <p className="text-muted">{t('common.loading')}</p>;
  if (error) return <p className="text-red-600">{error}</p>;
  if (incidents.length === 0) return <p className="text-muted">{t('incidents.noIncidents')}</p>;

  return (
    <div className="space-y-4">
      {incidents.map(incident => {
        const incidentName = getTranslation(incident, 'name');
        const incidentMessage = getTranslation(incident, 'message');

        return (
          <div key={incident.id} className="card p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className={`w-2 h-2 rounded-full ${incidentStatusClasses[incident.status]}`} />
              <span className="font-medium">{incidentName}</span>
              <span className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-card-border)' }}>
                {t(`incidentStatus.${incident.status}`)}
              </span>
              {incident.stickied && (
                <span className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-card-border)' }}>
                  {t('incidents.pinned')}
                </span>
              )}
            </div>
            {(incident.updates || []).length > 0 && (
              <ol className="space-y-2 mb-3 border-l-2 pl-3" style={{ borderColor: 'var(--color-card-border)' }}>
                {(incident.updates || []).map(update => (
                  <li key={update.id} className="text-sm">
                    <span className="font-medium">{t(`incidentStatus.${update.status}`)}</span>
                    <span className="text-muted"> · {new Date(update.created_at).toLocaleString()}</span>
                    {update.translations?.find(tr => tr.field === 'message')?.value && (
                      <p>{update.translations.find(tr => tr.field === 'message')?.value}</p>
                    )}
                  </li>
                ))}
              </ol>
            )}
            {incidentMessage && (
              <p className="text-sm mb-2">{incidentMessage}</p>
            )}
            <p className="text-sm text-muted">
              {new Date(incident.occurred_at || incident.created_at).toLocaleString()}
            </p>
          </div>
        );
      })}
    </div>
  );
}

function MaintenancesList() {
  const { t, i18n } = useTranslation();
  const [maintenances, setMaintenances] = useState<Maintenance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getMaintenances(i18n.language)
      .then(setMaintenances)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [i18n.language]);

  const formatDate = (dateStr: string): string => {
    const date = new Date(dateStr);
    return date.toLocaleString(i18n.language === 'fr' ? 'fr-FR' : 'en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  if (loading) return <p className="text-muted">{t('common.loading')}</p>;
  if (error) return <p className="text-red-600">{error}</p>;
  if (maintenances.length === 0) return <p className="text-muted">{t('maintenances.noMaintenances')}</p>;

  return (
    <div className="space-y-4">
      {maintenances.map(maintenance => (
        <div key={maintenance.id} className="card p-4">
          <div className="flex items-center gap-2 mb-2">
            <div className={`w-2 h-2 rounded-full ${maintenanceStatusClasses[maintenance.status]}`} />
            <span className="font-medium">{maintenance.name}</span>
            <span className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-card-border)' }}>
              {t(`maintenanceStatus.${maintenance.status}`)}
            </span>
          </div>
          {maintenance.message && (
            <p className="text-sm mb-2">{maintenance.message}</p>
          )}
          <div className="text-sm text-muted">
            <span>{t('maintenances.component')}: {maintenance.component}</span>
            <span className="mx-2">|</span>
            <span>{formatDate(maintenance.start_at)} - {formatDate(maintenance.end_at)}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

function SubscriptionForm() {
  const { t, i18n } = useTranslation();
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [message, setMessage] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStatus('loading');
    try {
      // Subscribe with current language preference
      const result = await subscribe(email, i18n.language);
      setStatus('success');
      setMessage(result.message || t('subscription.success'));
      setEmail('');
    } catch (err) {
      setStatus('error');
      setMessage(err instanceof Error ? err.message : t('subscription.error'));
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="flex gap-2">
        <input
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          placeholder={t('subscription.emailPlaceholder')}
          className="flex-1 px-4 py-2 rounded-lg focus:outline-none focus:ring-2"
          style={{
            backgroundColor: 'var(--color-card)',
            border: '1px solid var(--color-card-border)',
            color: 'var(--color-text)',
          }}
          required
        />
        <button
          type="submit"
          disabled={status === 'loading'}
          className="px-6 py-2 btn-primary rounded-lg disabled:opacity-50"
        >
          {status === 'loading' ? t('subscription.subscribing') : t('subscription.subscribe')}
        </button>
      </div>
      {message && (
        <p className={status === 'success' ? 'text-green-600' : 'text-red-600'}>{message}</p>
      )}
    </form>
  );
}

// Centered Layout (Default, Dark)
function CenteredLayout() {
  const { t, i18n } = useTranslation();
  const { branding } = useBranding();
  const title = (i18n.language === 'fr' ? branding.title_fr : branding.title_en) || t('status.title');

  return (
    <div className="min-h-screen">
      <div className="max-w-4xl mx-auto px-4 py-12">
        <div className="flex justify-end mb-6">
          <LanguageSwitcher />
        </div>

        <div className="text-center mb-12">
          {branding.logo && (
            <img src={branding.logo} alt="Logo" className="h-16 mx-auto mb-4" />
          )}
          <h1 className="text-4xl font-bold mb-4">{title}</h1>
          <p className="text-lg text-muted">{t('status.description')}</p>
        </div>

        <div className="card p-6 mb-8">
          <StatusBanner />
        </div>

        <div className="card p-6 mb-8">
          <h2 className="text-2xl font-bold mb-4">{t('maintenances.title')}</h2>
          <MaintenancesList />
        </div>

        <div className="card p-6 mb-8">
          <h2 className="text-2xl font-bold mb-4">{t('components.title')}</h2>
          <ComponentsList />
        </div>

        <div className="card p-6 mb-8">
          <h2 className="text-2xl font-bold mb-4">{t('incidents.title')}</h2>
          <IncidentsList />
        </div>

        <div className="card p-6">
          <h2 className="text-2xl font-bold mb-2">{t('subscription.title')}</h2>
          <p className="text-muted mb-4">{t('subscription.description')}</p>
          <SubscriptionForm />
        </div>
      </div>
    </div>
  );
}

// Sidebar Layout (Corporate)
function SidebarLayout() {
  const { t, i18n } = useTranslation();
  const { branding } = useBranding();
  const title = (i18n.language === 'fr' ? branding.title_fr : branding.title_en) || t('status.title');

  return (
    <div className="min-h-screen">
      <header className="header-gradient py-4 px-6 shadow-md">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <div className="flex items-center gap-4">
            {branding.logo && (
              <img src={branding.logo} alt="Logo" className="h-10" />
            )}
            <h1 className="text-2xl font-bold">{title}</h1>
          </div>
          <LanguageSwitcher />
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 py-8 flex gap-8">
        <aside className="w-64 flex-shrink-0">
          <div className="card p-4 sticky top-8">
            <h3 className="font-bold mb-4 text-sm uppercase text-muted">{t('components.title')}</h3>
            <ComponentsList asSidebar />
          </div>
        </aside>

        <main className="flex-1 space-y-6">
          <div className="card p-6">
            <StatusBanner />
          </div>

          <div className="card p-6">
            <h2 className="text-xl font-bold mb-4">{t('maintenances.title')}</h2>
            <MaintenancesList />
          </div>

          <div className="card p-6">
            <h2 className="text-xl font-bold mb-4">{t('incidents.title')}</h2>
            <IncidentsList />
          </div>

          <div className="card p-6">
            <h2 className="text-xl font-bold mb-2">{t('subscription.title')}</h2>
            <p className="text-muted mb-4">{t('subscription.description')}</p>
            <SubscriptionForm />
          </div>
        </main>
      </div>
    </div>
  );
}

// Full-width Layout (Modern)
function FullWidthLayout() {
  const { t, i18n } = useTranslation();
  const { branding } = useBranding();
  const title = (i18n.language === 'fr' ? branding.title_fr : branding.title_en) || t('status.title');

  return (
    <div className="min-h-screen">
      <header className="header-gradient py-8 px-6 shadow-lg">
        <div className="max-w-6xl mx-auto text-center">
          {branding.logo && (
            <img src={branding.logo} alt="Logo" className="h-16 mx-auto mb-4" />
          )}
          <h1 className="text-4xl font-bold mb-2">{title}</h1>
          <p className="opacity-90">{t('status.description')}</p>
          <div className="mt-4 flex justify-center">
            <LanguageSwitcher />
          </div>
        </div>
      </header>

      <div className="max-w-6xl mx-auto px-4 py-8 -mt-4">
        <div className="card p-6 mb-8">
          <StatusBanner centered />
        </div>

        <div className="card p-6 mb-8">
          <h2 className="text-xl font-bold mb-4">{t('maintenances.title')}</h2>
          <MaintenancesList />
        </div>

        <div className="grid md:grid-cols-2 gap-6 mb-8">
          <div className="card p-6">
            <h2 className="text-xl font-bold mb-4">{t('components.title')}</h2>
            <ComponentsList />
          </div>

          <div className="card p-6">
            <h2 className="text-xl font-bold mb-4">{t('incidents.title')}</h2>
            <IncidentsList />
          </div>
        </div>

        <div className="card p-6">
          <h2 className="text-xl font-bold mb-2 text-center">{t('subscription.title')}</h2>
          <p className="text-muted mb-4 text-center">{t('subscription.description')}</p>
          <div className="max-w-md mx-auto">
            <SubscriptionForm />
          </div>
        </div>
      </div>
    </div>
  );
}

export default function App() {
  const { theme, loading } = useTheme();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-pulse text-lg">Loading...</div>
      </div>
    );
  }

  switch (theme.layout) {
    case 'sidebar':
      return <SidebarLayout />;
    case 'full-width':
      return <FullWidthLayout />;
    default:
      return <CenteredLayout />;
  }
}

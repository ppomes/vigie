import { useState, useEffect, createContext, useContext, Fragment } from 'react';
import { useNavigate, Routes, Route, Link, useLocation } from 'react-router-dom';
import { api, Component, Incident, Subscriber, EmailConfig, BrandingConfig, IncidentTemplate, TemplateVariable, NotificationQueueStats, QueuedNotification, Maintenance, MaintenanceTemplate, User, AuditLog, APIKey, IncidentUpdateInput, ComponentGroup } from './lib/api';
import TwoFactorModal from './pages/TwoFactor';
import CustomizationSection from './pages/Customization';
import PageLanguageSection from './pages/PageLanguage';

// User context for role-based access
interface UserContextType {
  user: { id: number; username: string; email: string; role: 'admin' | 'viewer' } | null;
  setUser: (user: UserContextType['user']) => void;
  isAdmin: boolean;
}

const UserContext = createContext<UserContextType>({ user: null, setUser: () => {}, isAdmin: false });

export const useUser = () => useContext(UserContext);

const themes = [
  { id: 'default', name: 'Default', description: 'Clean, minimal design with centered layout', preview: 'bg-gray-100' },
  { id: 'dark', name: 'Dark', description: 'Dark mode with muted colors', preview: 'bg-slate-800' },
  { id: 'corporate', name: 'Corporate', description: 'Professional sidebar layout', preview: 'bg-sky-900' },
  { id: 'modern', name: 'Modern', description: 'Gradient accents, rounded cards', preview: 'bg-gradient-to-r from-indigo-500 to-purple-500' },
];

const emptyComponentForm = { name: '', status: 'operational', order: 0, group_id: 0, link: '', enabled: true, tags: '' };

function ComponentsPage() {
  const [components, setComponents] = useState<Component[]>([]);
  const [groups, setGroups] = useState<ComponentGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyComponentForm);

  useEffect(() => { loadComponents(); }, []);

  const loadComponents = async () => {
    try {
      const [data, g] = await Promise.all([api.getComponents(), api.getComponentGroups()]);
      setComponents(data);
      setGroups(g);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      // group_id 0 means "no group"; tags are typed comma-separated
      const data = { ...form, tags: form.tags.split(',').map(t => t.trim()).filter(Boolean) };
      if (editId) {
        await api.updateComponent(editId, data);
      } else {
        await api.createComponent(data);
      }
      setShowForm(false);
      setEditId(null);
      setForm(emptyComponentForm);
      loadComponents();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error saving component');
    }
  };

  const handleEdit = (c: Component) => {
    setForm({
      name: c.name, status: c.status, order: c.order, group_id: c.group_id || 0,
      link: c.link || '', enabled: c.enabled, tags: (c.tags || []).map(t => t.name).join(', '),
    });
    setEditId(c.id);
    setShowForm(true);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this component?')) return;
    await api.deleteComponent(id);
    loadComponents();
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Components</h2>
        <button onClick={() => { setShowForm(true); setEditId(null); setForm(emptyComponentForm); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
          Add Component
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-gray-50 p-4 rounded mb-4">
          <div className="grid grid-cols-4 gap-4">
            <input type="text" placeholder="Name" value={form.name} onChange={e => setForm({...form, name: e.target.value})}
              className="border rounded px-3 py-2" required />
            <select value={form.status} onChange={e => setForm({...form, status: e.target.value})} className="border rounded px-3 py-2">
              <option value="operational">Operational</option>
              <option value="degraded_performance">Degraded performance</option>
              <option value="partial_outage">Partial Outage</option>
              <option value="major_outage">Major Outage</option>
            </select>
            <input type="number" placeholder="Order" value={form.order} onChange={e => setForm({...form, order: parseInt(e.target.value) || 0})}
              className="border rounded px-3 py-2" />
            <select value={form.group_id} onChange={e => setForm({...form, group_id: parseInt(e.target.value)})} className="border rounded px-3 py-2">
              <option value={0}>No group</option>
              {groups.map(g => <option key={g.id} value={g.id}>{g.name}</option>)}
            </select>
            <input type="url" placeholder="Link (https://...)" value={form.link} onChange={e => setForm({...form, link: e.target.value})}
              className="border rounded px-3 py-2 col-span-2" maxLength={1024} />
            <input type="text" placeholder="Tags, comma-separated" value={form.tags} onChange={e => setForm({...form, tags: e.target.value})}
              className="border rounded px-3 py-2" />
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={form.enabled} onChange={e => setForm({...form, enabled: e.target.checked})} />
              Shown on the status page
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => setShowForm(false)} className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Status</th>
            <th className="border p-2 text-left">Order</th>
            <th className="border p-2 text-left">Group</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {components.map(c => (
            <tr key={c.id}>
              <td className="border p-2">
                {c.name}
                {!c.enabled && <span className="ml-2 text-xs px-2 py-1 rounded bg-gray-200 text-gray-600">hidden</span>}
                {(c.tags || []).map(t => <span key={t.id} className="ml-1 text-xs px-2 py-1 rounded bg-blue-50 text-blue-700">{t.name}</span>)}
              </td>
              <td className="border p-2">{c.status}</td>
              <td className="border p-2">{c.order}</td>
              <td className="border p-2">{groups.find(g => g.id === c.group_id)?.name || '-'}</td>
              <td className="border p-2">
                <button onClick={() => handleEdit(c)} className="text-blue-500 mr-2">Edit</button>
                <button onClick={() => handleDelete(c.id)} className="text-red-500">Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <ComponentGroupsSection groups={groups} onChange={loadComponents} />
    </div>
  );
}

const emptyGroupForm = { name: '', name_en: '', name_fr: '', order: 0, collapse: 'never' as ComponentGroup['collapse'], visibility: 'public' as ComponentGroup['visibility'] };

// Component groups: create, edit, delete. Deleting a group keeps its
// components, ungrouped.
function ComponentGroupsSection({ groups, onChange }: { groups: ComponentGroup[]; onChange: () => void }) {
  const [editId, setEditId] = useState<number | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState(emptyGroupForm);

  const translation = (g: ComponentGroup, locale: string) =>
    g.translations?.find(t => t.field === 'name' && t.locale === locale)?.value || '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const translations: { locale: string; field: 'name'; value: string }[] = [];
    if (form.name_en.trim()) translations.push({ locale: 'en', field: 'name', value: form.name_en });
    if (form.name_fr.trim()) translations.push({ locale: 'fr', field: 'name', value: form.name_fr });
    const data = { name: form.name, order: form.order, collapse: form.collapse, visibility: form.visibility, translations };
    try {
      if (editId) {
        await api.updateComponentGroup(editId, data);
      } else {
        await api.createComponentGroup(data);
      }
      setShowForm(false);
      setEditId(null);
      setForm(emptyGroupForm);
      onChange();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error saving group');
    }
  };

  const handleEdit = (g: ComponentGroup) => {
    setForm({ name: g.name, name_en: translation(g, 'en'), name_fr: translation(g, 'fr'), order: g.order, collapse: g.collapse, visibility: g.visibility });
    setEditId(g.id);
    setShowForm(true);
  };

  const handleDelete = async (g: ComponentGroup) => {
    if (!confirm(`Delete the group "${g.name}"? Its components are kept, without a group.`)) return;
    try {
      await api.deleteComponentGroup(g.id);
      onChange();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error deleting group');
    }
  };

  const collapseLabel = { never: 'Always expanded', always: 'Always collapsed', operational: 'Collapsed while operational' };

  return (
    <div className="mt-8">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Component groups</h2>
        <button onClick={() => { setShowForm(true); setEditId(null); setForm(emptyGroupForm); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
          Add Group
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-gray-50 p-4 rounded mb-4">
          <div className="grid grid-cols-3 gap-4">
            <input type="text" placeholder="Name (default)" value={form.name} onChange={e => setForm({...form, name: e.target.value})}
              className="border rounded px-3 py-2" required maxLength={255} />
            <input type="text" placeholder="Name (English)" value={form.name_en} onChange={e => setForm({...form, name_en: e.target.value})}
              className="border rounded px-3 py-2" />
            <input type="text" placeholder="Name (French)" value={form.name_fr} onChange={e => setForm({...form, name_fr: e.target.value})}
              className="border rounded px-3 py-2" />
            <input type="number" placeholder="Order" value={form.order} onChange={e => setForm({...form, order: parseInt(e.target.value) || 0})}
              className="border rounded px-3 py-2" />
            <select value={form.collapse} onChange={e => setForm({...form, collapse: e.target.value as ComponentGroup['collapse']})} className="border rounded px-3 py-2">
              <option value="never">{collapseLabel.never}</option>
              <option value="always">{collapseLabel.always}</option>
              <option value="operational">{collapseLabel.operational}</option>
            </select>
            <select value={form.visibility} onChange={e => setForm({...form, visibility: e.target.value as ComponentGroup['visibility']})} className="border rounded px-3 py-2">
              <option value="public">Public</option>
              <option value="authenticated">Logged-in users only</option>
            </select>
          </div>
          <div className="mt-4 flex gap-2">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => setShowForm(false)} className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Order</th>
            <th className="border p-2 text-left">Display</th>
            <th className="border p-2 text-left">Visibility</th>
            <th className="border p-2 text-left">Components</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {groups.map(g => (
            <tr key={g.id}>
              <td className="border p-2">{g.name}</td>
              <td className="border p-2">{g.order}</td>
              <td className="border p-2">{collapseLabel[g.collapse]}</td>
              <td className="border p-2">{g.visibility === 'public' ? 'Public' : 'Logged-in users only'}</td>
              <td className="border p-2">{g.components?.length || 0}</td>
              <td className="border p-2">
                <button onClick={() => handleEdit(g)} className="text-blue-500 mr-2">Edit</button>
                <button onClick={() => handleDelete(g)} className="text-red-500">Delete</button>
              </td>
            </tr>
          ))}
          {groups.length === 0 && (
            <tr><td colSpan={6} className="border p-4 text-center text-gray-500">No groups. Components are listed without grouping.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

interface IncidentForm {
  name: string;
  status: string;
  component_id: number;
  // Empty means "leave the component status as it is"
  component_status: string;
  // Other affected components, besides component_id
  extra_component_ids: number[];
  visible: boolean;
  stickied: boolean;
  // datetime-local value; empty means "now" on creation
  occurred_at: string;
  notify: boolean;
  name_en: string;
  name_fr: string;
  message_en: string;
  message_fr: string;
}

const defaultIncidentForm: IncidentForm = {
  name: '',
  status: 'investigating',
  component_id: 0,
  component_status: '',
  extra_component_ids: [],
  visible: true,
  stickied: false,
  occurred_at: '',
  notify: true,
  name_en: '',
  name_fr: '',
  message_en: '',
  message_fr: '',
};

// Checkboxes for the components an incident or maintenance also affects,
// besides its main component.
// mainComponentChange is the form change for a new main component. "No
// component (general)" also drops the extra components, which the API would
// otherwise promote to the main one.
function mainComponentChange(value: string): { component_id: number; extra_component_ids?: number[] } {
  const id = parseInt(value);
  return id === 0 ? { component_id: 0, extra_component_ids: [] } : { component_id: id };
}

function ExtraComponentsPicker({ components, mainId, value, onChange }: {
  components: Component[];
  mainId: number;
  value: number[];
  onChange: (ids: number[]) => void;
}) {
  const others = components.filter(c => c.id !== mainId);
  if (others.length === 0) return null;
  const toggle = (id: number) => onChange(value.includes(id) ? value.filter(v => v !== id) : [...value, id]);
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">Also affects</label>
      <div className="flex flex-wrap gap-3">
        {others.map(c => (
          <label key={c.id} className="flex items-center gap-1 text-sm">
            <input type="checkbox" checked={value.includes(c.id)} onChange={() => toggle(c.id)} />
            {c.name}
          </label>
        ))}
      </div>
    </div>
  );
}

// The incident form only changes the component status when asked to.
function ComponentStatusSelect({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">Component status</label>
      <select value={value} onChange={e => onChange(e.target.value)} className="w-full border rounded px-3 py-2">
        <option value="">Leave unchanged</option>
        <option value="operational">Operational</option>
        <option value="degraded_performance">Degraded performance</option>
        <option value="partial_outage">Partial outage</option>
        <option value="major_outage">Major outage</option>
      </select>
    </div>
  );
}

// Timeline of an incident: its updates, newest first, and a form to post one.
// Posting sets the incident status and notifies subscribers.
function IncidentTimeline({ incident, onChange }: { incident: Incident; onChange: () => void }) {
  const { isAdmin } = useUser();
  const [status, setStatus] = useState(incident.status);
  const [messageEn, setMessageEn] = useState('');
  const [messageFr, setMessageFr] = useState('');
  const [componentStatus, setComponentStatus] = useState('');
  const [saving, setSaving] = useState(false);

  const message = (u: { translations?: { locale: string; field: string; value: string }[] }, locale: string) =>
    u.translations?.find(t => t.field === 'message' && t.locale === locale)?.value;

  const handlePost = async (e: React.FormEvent) => {
    e.preventDefault();
    const translations: IncidentUpdateInput['translations'] = [];
    if (messageEn.trim()) translations.push({ locale: 'en', field: 'message', value: messageEn });
    if (messageFr.trim()) translations.push({ locale: 'fr', field: 'message', value: messageFr });
    if (translations.length === 0) {
      alert('Write a message in at least one language.');
      return;
    }
    setSaving(true);
    try {
      await api.createIncidentUpdate(incident.id, { status, translations, component_status: componentStatus || undefined });
      setMessageEn('');
      setMessageFr('');
      setComponentStatus('');
      onChange();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to post update');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (updateId: number) => {
    if (!confirm('Delete this update? The incident status falls back to the previous update.')) return;
    try {
      await api.deleteIncidentUpdate(incident.id, updateId);
      onChange();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete update');
    }
  };

  return (
    <div className="bg-gray-50 p-4">
      <h4 className="font-medium mb-2">Timeline</h4>
      {(incident.updates || []).length === 0 && <p className="text-sm text-gray-500 mb-2">No updates yet.</p>}
      <ul className="space-y-2 mb-4">
        {(incident.updates || []).map(u => (
          <li key={u.id} className="bg-white border rounded p-2 text-sm">
            <div className="flex justify-between">
              <span>
                <span className="font-medium">{u.status}</span>
                <span className="text-gray-500 ml-2">{new Date(u.created_at).toLocaleString()}</span>
                {u.user && <span className="text-gray-500 ml-2">by {u.user.username}</span>}
              </span>
              {isAdmin && <button onClick={() => handleDelete(u.id)} className="text-red-500">Delete</button>}
            </div>
            {message(u, 'en') && <p><span className="text-gray-500">EN:</span> {message(u, 'en')}</p>}
            {message(u, 'fr') && <p><span className="text-gray-500">FR:</span> {message(u, 'fr')}</p>}
          </li>
        ))}
      </ul>
      {isAdmin && (
        <form onSubmit={handlePost} className="space-y-2">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">New status</label>
              <select value={status} onChange={e => setStatus(e.target.value)} className="w-full border rounded px-3 py-2">
                <option value="investigating">Investigating</option>
                <option value="identified">Identified</option>
                <option value="monitoring">Monitoring</option>
                <option value="resolved">Resolved</option>
              </select>
            </div>
            <ComponentStatusSelect value={componentStatus} onChange={setComponentStatus} />
            <textarea placeholder="Message (English)" value={messageEn} onChange={e => setMessageEn(e.target.value)}
              className="border rounded px-3 py-2" rows={2} />
            <textarea placeholder="Message (French)" value={messageFr} onChange={e => setMessageFr(e.target.value)}
              className="border rounded px-3 py-2" rows={2} />
          </div>
          <button type="submit" disabled={saving} className="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-50">
            Post update
          </button>
        </form>
      )}
    </div>
  );
}

// Converts an API timestamp to a datetime-local input value, in local time.
function toLocalInput(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// The affected components of an incident form: the main component first,
// with the chosen status, then the others, whose status is left unchanged.
function affectedComponents(form: IncidentForm) {
  const main = form.component_id ? [{ component_id: form.component_id, status: form.component_status || undefined }] : [];
  return [
    ...main,
    ...form.extra_component_ids.filter(id => id !== form.component_id).map(id => ({ component_id: id })),
  ];
}

// Optional incident fields, as sent to the API. notify only applies on creation.
function incidentOptions(form: IncidentForm, creating: boolean) {
  return {
    visible: form.visible,
    stickied: form.stickied,
    occurred_at: form.occurred_at ? new Date(form.occurred_at).toISOString() : null,
    notify: creating ? form.notify : undefined,
  };
}

function IncidentOptionsFields({ form, setForm, creating }: {
  form: IncidentForm;
  setForm: (f: IncidentForm) => void;
  creating: boolean;
}) {
  return (
    <div className="grid grid-cols-2 gap-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Started at</label>
        <input type="datetime-local" value={form.occurred_at} onChange={e => setForm({...form, occurred_at: e.target.value})}
          className="w-full border rounded px-3 py-2" />
        <p className="text-xs text-gray-500 mt-1">Leave empty for now.</p>
      </div>
      <div className="space-y-1 pt-6">
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.visible} onChange={e => setForm({...form, visible: e.target.checked})} />
          Shown on the status page (unchecked: internal, no notifications)
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.stickied} onChange={e => setForm({...form, stickied: e.target.checked})} />
          Pinned at the top
        </label>
        {creating && (
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={form.notify} onChange={e => setForm({...form, notify: e.target.checked})} />
            Notify subscribers
          </label>
        )}
      </div>
    </div>
  );
}

function IncidentsPage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [templates, setTemplates] = useState<IncidentTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<IncidentForm>(defaultIncidentForm);
  const [openTimeline, setOpenTimeline] = useState<number | null>(null);

  // Template mode state
  const [mode, setMode] = useState<'freeform' | 'template'>('freeform');
  const [selectedTemplateId, setSelectedTemplateId] = useState<number | null>(null);
  const [variableValues, setVariableValues] = useState<Record<string, string>>({});
  const [preview, setPreview] = useState<{ name_en: string; name_fr: string; message_en: string; message_fr: string } | null>(null);

  useEffect(() => {
    Promise.all([api.getIncidents(), api.getComponents(), api.getTemplates()]).then(([i, c, t]) => {
      setIncidents(i);
      setComponents(c);
      setTemplates(t);
      if (c.length > 0) setForm(f => ({ ...f, component_id: c[0].id }));
      setLoading(false);
    });
  }, []);

  const loadIncidents = async () => {
    const data = await api.getIncidents();
    setIncidents(data);
  };

  const selectedTemplate = templates.find(t => t.id === selectedTemplateId);

  const updatePreview = async () => {
    if (!selectedTemplateId) return;
    try {
      const result = await api.previewTemplate(selectedTemplateId, variableValues);
      setPreview(result.preview);
    } catch {
      // Ignore preview errors
    }
  };

  useEffect(() => {
    if (selectedTemplateId) {
      updatePreview();
    }
  }, [selectedTemplateId, variableValues]);

  const buildTranslations = () => {
    const translations: { locale: string; field: string; value: string }[] = [];
    if (form.name_en) translations.push({ locale: 'en', field: 'name', value: form.name_en });
    if (form.name_fr) translations.push({ locale: 'fr', field: 'name', value: form.name_fr });
    if (form.message_en) translations.push({ locale: 'en', field: 'message', value: form.message_en });
    if (form.message_fr) translations.push({ locale: 'fr', field: 'message', value: form.message_fr });
    return translations;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      let data;

      if (mode === 'template' && preview) {
        // Use preview values from template
        data = {
          name: preview.name_en || selectedTemplate?.name || 'Incident',
          status: selectedTemplate?.default_status || 'investigating',
          component_id: form.component_id,
          component_status: form.component_status || undefined,
          components: affectedComponents(form),
          ...incidentOptions(form, !editId),
          translations: [
            { locale: 'en', field: 'name', value: preview.name_en },
            { locale: 'fr', field: 'name', value: preview.name_fr },
            { locale: 'en', field: 'message', value: preview.message_en },
            { locale: 'fr', field: 'message', value: preview.message_fr },
          ].filter(t => t.value),
        };
      } else {
        data = {
          name: form.name,
          status: form.status,
          component_id: form.component_id,
          component_status: form.component_status || undefined,
          components: affectedComponents(form),
          ...incidentOptions(form, !editId),
          translations: buildTranslations(),
        };
      }

      if (editId) {
        await api.updateIncident(editId, data);
      } else {
        await api.createIncident(data);
      }
      setShowForm(false);
      setEditId(null);
      setMode('freeform');
      setSelectedTemplateId(null);
      setVariableValues({});
      setPreview(null);
      setForm({ ...defaultIncidentForm, component_id: components[0]?.id || 0 });
      loadIncidents();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error saving incident');
    }
  };

  const handleEdit = (i: Incident) => {
    const getTranslation = (field: string, locale: string) =>
      i.translations?.find(t => t.field === field && t.locale === locale)?.value || '';

    setForm({
      name: i.name,
      status: i.status,
      component_id: i.component_id ?? 0,
      component_status: '',
      extra_component_ids: (i.components || []).map(ac => ac.component_id).filter(id => id !== i.component_id),
      visible: i.visible,
      stickied: i.stickied,
      occurred_at: i.occurred_at ? toLocalInput(i.occurred_at) : '',
      notify: i.notifications,
      name_en: getTranslation('name', 'en'),
      name_fr: getTranslation('name', 'fr'),
      message_en: getTranslation('message', 'en'),
      message_fr: getTranslation('message', 'fr'),
    });
    setEditId(i.id);
    setMode('freeform');
    setShowForm(true);
  };

  const handleSelectTemplate = (templateId: number) => {
    setSelectedTemplateId(templateId);
    setVariableValues({});
    const template = templates.find(t => t.id === templateId);
    if (template?.default_component_id) {
      setForm(f => ({ ...f, component_id: template.default_component_id! }));
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this incident?')) return;
    await api.deleteIncident(id);
    loadIncidents();
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Incidents</h2>
        <button onClick={() => { setShowForm(true); setEditId(null); setForm({ ...defaultIncidentForm, component_id: components[0]?.id || 0 }); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
          Add Incident
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-gray-50 p-4 rounded mb-4 space-y-4">
          {/* Mode toggle - only show for new incidents */}
          {!editId && templates.length > 0 && (
            <div className="flex gap-2 mb-4">
              <button
                type="button"
                onClick={() => { setMode('freeform'); setSelectedTemplateId(null); setPreview(null); }}
                className={`px-4 py-2 rounded ${mode === 'freeform' ? 'bg-blue-500 text-white' : 'bg-white border hover:bg-gray-50'}`}
              >
                Free Form
              </button>
              <button
                type="button"
                onClick={() => setMode('template')}
                className={`px-4 py-2 rounded ${mode === 'template' ? 'bg-blue-500 text-white' : 'bg-white border hover:bg-gray-50'}`}
              >
                Use Template
              </button>
            </div>
          )}

          {/* Template selection - show when in template mode */}
          {mode === 'template' && !editId && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Select Template</label>
                <select
                  value={selectedTemplateId || ''}
                  onChange={e => handleSelectTemplate(parseInt(e.target.value))}
                  className="w-full border rounded px-3 py-2"
                >
                  <option value="">-- Choose a template --</option>
                  {templates.map(t => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                </select>
              </div>

              {/* Component selection */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Component</label>
                <select
                  value={form.component_id}
                  onChange={e => setForm({...form, ...mainComponentChange(e.target.value)})}
                  className="w-full border rounded px-3 py-2"
                >
                  <option value={0}>No component (general)</option>
                  {components.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
                </select>
              </div>

              <ComponentStatusSelect value={form.component_status} onChange={v => setForm({...form, component_status: v})} />
              <ExtraComponentsPicker components={components} mainId={form.component_id} value={form.extra_component_ids}
                onChange={ids => setForm({...form, extra_component_ids: ids})} />
              <IncidentOptionsFields form={form} setForm={setForm} creating={!editId} />

              {/* Variable inputs */}
              {selectedTemplate && selectedTemplate.parsed_variables && selectedTemplate.parsed_variables.length > 0 && (
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-3">Fill in Variables</h4>
                  <div className="grid grid-cols-2 gap-4">
                    {selectedTemplate.parsed_variables.map(v => (
                      <div key={v.name}>
                        <label className="block text-sm text-gray-600 mb-1">
                          {v.label_en}
                          {v.required && <span className="text-red-500 ml-1">*</span>}
                        </label>
                        {v.type === 'text' && (
                          <input
                            type="text"
                            value={variableValues[v.name] || v.default || ''}
                            onChange={e => setVariableValues({...variableValues, [v.name]: e.target.value})}
                            className="w-full border rounded px-3 py-2"
                            placeholder={v.label_en}
                            required={v.required}
                          />
                        )}
                        {v.type === 'number' && (
                          <input
                            type="number"
                            value={variableValues[v.name] || v.default || ''}
                            onChange={e => setVariableValues({...variableValues, [v.name]: e.target.value})}
                            className="w-full border rounded px-3 py-2"
                            required={v.required}
                          />
                        )}
                        {v.type === 'datetime' && (
                          <input
                            type="datetime-local"
                            value={variableValues[v.name] || ''}
                            onChange={e => setVariableValues({...variableValues, [v.name]: e.target.value})}
                            className="w-full border rounded px-3 py-2"
                            required={v.required}
                          />
                        )}
                        {v.type === 'select' && v.options && (
                          <select
                            value={variableValues[v.name] || v.default || ''}
                            onChange={e => setVariableValues({...variableValues, [v.name]: e.target.value})}
                            className="w-full border rounded px-3 py-2"
                            required={v.required}
                          >
                            <option value="">-- Select --</option>
                            {v.options.map(opt => (
                              <option key={opt} value={opt}>{opt}</option>
                            ))}
                          </select>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Preview */}
              {preview && (
                <div className="border-t pt-4">
                  <h4 className="font-medium mb-3">Preview</h4>
                  <div className="bg-white border rounded p-4 space-y-3">
                    <div>
                      <span className="text-xs text-gray-500 uppercase">Name (EN)</span>
                      <p className="font-medium">{preview.name_en || <span className="text-gray-400">-</span>}</p>
                    </div>
                    <div>
                      <span className="text-xs text-gray-500 uppercase">Name (FR)</span>
                      <p className="font-medium">{preview.name_fr || <span className="text-gray-400">-</span>}</p>
                    </div>
                    <div>
                      <span className="text-xs text-gray-500 uppercase">Message (EN)</span>
                      <p className="text-sm text-gray-700">{preview.message_en || <span className="text-gray-400">-</span>}</p>
                    </div>
                    <div>
                      <span className="text-xs text-gray-500 uppercase">Message (FR)</span>
                      <p className="text-sm text-gray-700">{preview.message_fr || <span className="text-gray-400">-</span>}</p>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Freeform mode or editing */}
          {(mode === 'freeform' || editId) && (
            <>
              <div className="grid grid-cols-3 gap-4">
                <input type="text" placeholder="Name (default)" value={form.name} onChange={e => setForm({...form, name: e.target.value})}
                  className="border rounded px-3 py-2" required />
                <select value={form.status} onChange={e => setForm({...form, status: e.target.value})} className="border rounded px-3 py-2">
                  <option value="investigating">Investigating</option>
                  <option value="identified">Identified</option>
                  <option value="monitoring">Monitoring</option>
                  <option value="resolved">Resolved</option>
                </select>
                <select value={form.component_id} onChange={e => setForm({...form, ...mainComponentChange(e.target.value)})} className="border rounded px-3 py-2">
                  <option value={0}>No component (general)</option>
                  {components.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
                </select>
              </div>

              <ComponentStatusSelect value={form.component_status} onChange={v => setForm({...form, component_status: v})} />
              <ExtraComponentsPicker components={components} mainId={form.component_id} value={form.extra_component_ids}
                onChange={ids => setForm({...form, extra_component_ids: ids})} />
              <IncidentOptionsFields form={form} setForm={setForm} creating={!editId} />

              <div className="border-t pt-4">
                <h4 className="font-medium mb-2">Translations</h4>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm text-gray-600 mb-1">Name (English)</label>
                    <input type="text" placeholder="English name" value={form.name_en} onChange={e => setForm({...form, name_en: e.target.value})}
                      className="w-full border rounded px-3 py-2" />
                  </div>
                  <div>
                    <label className="block text-sm text-gray-600 mb-1">Name (French)</label>
                    <input type="text" placeholder="French name" value={form.name_fr} onChange={e => setForm({...form, name_fr: e.target.value})}
                      className="w-full border rounded px-3 py-2" />
                  </div>
                  <div>
                    <label className="block text-sm text-gray-600 mb-1">Message (English)</label>
                    <textarea placeholder="English message" value={form.message_en} onChange={e => setForm({...form, message_en: e.target.value})}
                      className="w-full border rounded px-3 py-2" rows={3} />
                  </div>
                  <div>
                    <label className="block text-sm text-gray-600 mb-1">Message (French)</label>
                    <textarea placeholder="French message" value={form.message_fr} onChange={e => setForm({...form, message_fr: e.target.value})}
                      className="w-full border rounded px-3 py-2" rows={3} />
                  </div>
                </div>
              </div>
            </>
          )}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={mode === 'template' && !selectedTemplateId}
              className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => { setShowForm(false); setMode('freeform'); setSelectedTemplateId(null); setPreview(null); }} className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Status</th>
            <th className="border p-2 text-left">Component</th>
            <th className="border p-2 text-left">Created</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {incidents.map(i => (
            <Fragment key={i.id}>
              <tr>
                <td className="border p-2">
                  {i.name}
                  {!i.visible && <span className="ml-2 text-xs px-2 py-1 rounded bg-gray-200 text-gray-600">internal</span>}
                  {i.stickied && <span className="ml-2 text-xs px-2 py-1 rounded bg-yellow-100 text-yellow-800">pinned</span>}
                </td>
                <td className="border p-2">{i.status}</td>
                <td className="border p-2">{components.find(c => c.id === i.component_id)?.name || '-'}</td>
                <td className="border p-2">{new Date(i.created_at).toLocaleDateString()}</td>
                <td className="border p-2">
                  <button onClick={() => setOpenTimeline(openTimeline === i.id ? null : i.id)} className="text-gray-700 mr-2">
                    Updates ({i.updates?.length || 0})
                  </button>
                  <button onClick={() => handleEdit(i)} className="text-blue-500 mr-2">Edit</button>
                  <button onClick={() => handleDelete(i.id)} className="text-red-500">Delete</button>
                </td>
              </tr>
              {openTimeline === i.id && (
                <tr>
                  <td colSpan={5} className="border p-0">
                    <IncidentTimeline incident={i} onChange={loadIncidents} />
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SubscribersPage() {
  const [subscribers, setSubscribers] = useState<Subscriber[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadSubscribers(); }, []);

  const loadSubscribers = async () => {
    try {
      const data = await api.getSubscribers();
      setSubscribers(data);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this subscriber?')) return;
    await api.deleteSubscriber(id);
    loadSubscribers();
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h2 className="text-xl font-bold mb-4">Subscribers</h2>
      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Email</th>
            <th className="border p-2 text-left">Confirmed</th>
            <th className="border p-2 text-left">Created</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {subscribers.map(s => (
            <tr key={s.id}>
              <td className="border p-2">{s.email}</td>
              <td className="border p-2">{s.confirmed ? 'Yes' : 'No'}</td>
              <td className="border p-2">{new Date(s.created_at).toLocaleDateString()}</td>
              <td className="border p-2">
                <button onClick={() => handleDelete(s.id)} className="text-red-500">Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TemplatesPage() {
  const [templates, setTemplates] = useState<IncidentTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState({
    name: '',
    default_status: 'investigating',
    name_en: '',
    name_fr: '',
    message_en: '',
    message_fr: '',
    variables: [] as TemplateVariable[],
  });

  useEffect(() => { loadTemplates(); }, []);

  const loadTemplates = async () => {
    try {
      const data = await api.getTemplates();
      setTemplates(data);
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setForm({
      name: '',
      default_status: 'investigating',
      name_en: '',
      name_fr: '',
      message_en: '',
      message_fr: '',
      variables: [],
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editId) {
        await api.updateTemplate(editId, form);
      } else {
        await api.createTemplate(form);
      }
      setShowForm(false);
      setEditId(null);
      resetForm();
      loadTemplates();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error saving template');
    }
  };

  const handleEdit = (t: IncidentTemplate) => {
    setForm({
      name: t.name,
      default_status: t.default_status,
      name_en: t.name_en || '',
      name_fr: t.name_fr || '',
      message_en: t.message_en || '',
      message_fr: t.message_fr || '',
      variables: t.parsed_variables || [],
    });
    setEditId(t.id);
    setShowForm(true);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this template?')) return;
    await api.deleteTemplate(id);
    loadTemplates();
  };

  const addVariable = () => {
    setForm({
      ...form,
      variables: [...form.variables, { name: '', label_en: '', label_fr: '', type: 'text' as const, required: true }],
    });
  };

  const updateVariable = (index: number, field: keyof TemplateVariable, value: string | boolean) => {
    const newVars = [...form.variables];
    newVars[index] = { ...newVars[index], [field]: value };
    setForm({ ...form, variables: newVars });
  };

  const removeVariable = (index: number) => {
    setForm({ ...form, variables: form.variables.filter((_, i) => i !== index) });
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Incident Templates</h2>
        <button onClick={() => { setShowForm(true); setEditId(null); resetForm(); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
          Add Template
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-gray-50 p-4 rounded mb-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Template Name</label>
              <input type="text" placeholder="e.g., Service Outage" value={form.name}
                onChange={e => setForm({...form, name: e.target.value})}
                className="w-full border rounded px-3 py-2" required />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Default Status</label>
              <select value={form.default_status} onChange={e => setForm({...form, default_status: e.target.value})}
                className="w-full border rounded px-3 py-2">
                <option value="investigating">Investigating</option>
                <option value="identified">Identified</option>
                <option value="monitoring">Monitoring</option>
              </select>
            </div>
          </div>

          <div className="border-t pt-4">
            <h4 className="font-medium mb-2">Name Template</h4>
            <p className="text-sm text-gray-500 mb-2">Use {'{{variable_name}}'} for placeholders. System variables: {'{{date}}'}, {'{{time}}'}, {'{{datetime}}'}</p>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-600 mb-1">English</label>
                <input type="text" placeholder="{{service_name}} is unavailable" value={form.name_en}
                  onChange={e => setForm({...form, name_en: e.target.value})}
                  className="w-full border rounded px-3 py-2" />
              </div>
              <div>
                <label className="block text-sm text-gray-600 mb-1">French</label>
                <input type="text" placeholder="{{service_name}} est indisponible" value={form.name_fr}
                  onChange={e => setForm({...form, name_fr: e.target.value})}
                  className="w-full border rounded px-3 py-2" />
              </div>
            </div>
          </div>

          <div className="border-t pt-4">
            <h4 className="font-medium mb-2">Message Template</h4>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-600 mb-1">English</label>
                <textarea placeholder="We are investigating issues with {{service_name}}..." value={form.message_en}
                  onChange={e => setForm({...form, message_en: e.target.value})}
                  className="w-full border rounded px-3 py-2" rows={3} />
              </div>
              <div>
                <label className="block text-sm text-gray-600 mb-1">French</label>
                <textarea placeholder="Nous enquêtons sur des problèmes avec {{service_name}}..." value={form.message_fr}
                  onChange={e => setForm({...form, message_fr: e.target.value})}
                  className="w-full border rounded px-3 py-2" rows={3} />
              </div>
            </div>
          </div>

          <div className="border-t pt-4">
            <div className="flex justify-between items-center mb-2">
              <h4 className="font-medium">Variables</h4>
              <button type="button" onClick={addVariable}
                className="text-sm bg-gray-200 px-3 py-1 rounded hover:bg-gray-300">
                + Add Variable
              </button>
            </div>
            {form.variables.length === 0 && (
              <p className="text-sm text-gray-500">No custom variables defined.</p>
            )}
            {form.variables.map((v, i) => (
              <div key={i} className="grid grid-cols-5 gap-2 mb-2 items-end">
                <div>
                  <label className="block text-xs text-gray-500">Name</label>
                  <input type="text" placeholder="service_name" value={v.name}
                    onChange={e => updateVariable(i, 'name', e.target.value)}
                    className="w-full border rounded px-2 py-1 text-sm" />
                </div>
                <div>
                  <label className="block text-xs text-gray-500">Label (EN)</label>
                  <input type="text" placeholder="Service Name" value={v.label_en}
                    onChange={e => updateVariable(i, 'label_en', e.target.value)}
                    className="w-full border rounded px-2 py-1 text-sm" />
                </div>
                <div>
                  <label className="block text-xs text-gray-500">Label (FR)</label>
                  <input type="text" placeholder="Nom du service" value={v.label_fr}
                    onChange={e => updateVariable(i, 'label_fr', e.target.value)}
                    className="w-full border rounded px-2 py-1 text-sm" />
                </div>
                <div>
                  <label className="block text-xs text-gray-500">Type</label>
                  <select value={v.type} onChange={e => updateVariable(i, 'type', e.target.value)}
                    className="w-full border rounded px-2 py-1 text-sm">
                    <option value="text">Text</option>
                    <option value="number">Number</option>
                    <option value="datetime">DateTime</option>
                  </select>
                </div>
                <button type="button" onClick={() => removeVariable(i)}
                  className="text-red-500 text-sm hover:text-red-700">Remove</button>
              </div>
            ))}
          </div>

          <div className="flex gap-2 pt-4">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => { setShowForm(false); setEditId(null); }}
              className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Default Status</th>
            <th className="border p-2 text-left">Variables</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {templates.map(t => (
            <tr key={t.id}>
              <td className="border p-2">
                <div className="font-medium">{t.name}</div>
                <div className="text-sm text-gray-500">{t.name_en}</div>
              </td>
              <td className="border p-2">{t.default_status}</td>
              <td className="border p-2">
                {t.parsed_variables?.map(v => v.name).join(', ') || '-'}
              </td>
              <td className="border p-2">
                <button onClick={() => handleEdit(t)} className="text-blue-500 mr-2">Edit</button>
                <button onClick={() => handleDelete(t.id)} className="text-red-500">Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const defaultEmailConfig: EmailConfig = {
  provider: 'disabled',
  from_address: '',
  from_name: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_tls: true,
  sendgrid_api_key: '',
  mailgun_api_key: '',
  mailgun_domain: '',
  aws_region: '',
  aws_access_key: '',
  aws_secret_key: '',
};

const defaultBranding: BrandingConfig = {
  title_en: '',
  title_fr: '',
  logo: '',
};

function SettingsPage() {
  const [savedTheme, setSavedTheme] = useState('default');
  const [selectedTheme, setSelectedTheme] = useState('default');
  const [emailConfig, setEmailConfig] = useState<EmailConfig>(defaultEmailConfig);
  const [savedEmailConfig, setSavedEmailConfig] = useState<EmailConfig>(defaultEmailConfig);
  const [branding, setBranding] = useState<BrandingConfig>(defaultBranding);
  const [loading, setLoading] = useState(true);
  const [savingTheme, setSavingTheme] = useState(false);
  const [savingEmail, setSavingEmail] = useState(false);
  const [savingBranding, setSavingBranding] = useState(false);
  const [testingEmail, setTestingEmail] = useState(false);
  const [testEmail, setTestEmail] = useState('');
  const [themeMessage, setThemeMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [emailMessage, setEmailMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [brandingMessage, setBrandingMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Check if email config has unsaved changes
  const hasUnsavedEmailChanges = JSON.stringify(emailConfig) !== JSON.stringify(savedEmailConfig);

  useEffect(() => {
    Promise.all([api.getSettings(), api.getEmailConfig(), api.getBranding()])
      .then(([settings, config, brandingConfig]) => {
        const themeSetting = settings.find(s => s.key === 'theme');
        if (themeSetting) {
          setSavedTheme(themeSetting.value);
          setSelectedTheme(themeSetting.value);
        }
        if (config) {
          const mergedConfig = { ...defaultEmailConfig, ...config };
          setEmailConfig(mergedConfig);
          setSavedEmailConfig(mergedConfig);
        }
        if (brandingConfig) setBranding(brandingConfig);
      })
      .finally(() => setLoading(false));
  }, []);

  const handleThemeSelect = (themeId: string) => {
    setSelectedTheme(themeId);
  };

  const handleThemeSave = async () => {
    setSavingTheme(true);
    setThemeMessage(null);
    try {
      await api.updateSetting('theme', selectedTheme);
      setSavedTheme(selectedTheme);
      setThemeMessage({ type: 'success', text: 'Theme saved successfully! Refresh the status page to see changes.' });
    } catch (err) {
      setThemeMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save theme' });
    } finally {
      setSavingTheme(false);
    }
  };

  const handleEmailConfigSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingEmail(true);
    setEmailMessage(null);
    try {
      await api.updateEmailConfig(emailConfig);
      setSavedEmailConfig(emailConfig);
      setEmailMessage({ type: 'success', text: 'Email configuration saved successfully!' });
    } catch (err) {
      setEmailMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save email config' });
    } finally {
      setSavingEmail(false);
    }
  };

  const handleTestEmail = async () => {
    if (!testEmail) {
      setEmailMessage({ type: 'error', text: 'Please enter a test email address' });
      return;
    }
    setTestingEmail(true);
    setEmailMessage(null);
    try {
      await api.testEmailConfig(testEmail);
      setEmailMessage({ type: 'success', text: 'Test email sent successfully!' });
    } catch (err) {
      setEmailMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to send test email' });
    } finally {
      setTestingEmail(false);
    }
  };

  const handleBrandingSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingBranding(true);
    setBrandingMessage(null);
    try {
      await api.updateBranding(branding);
      setBrandingMessage({ type: 'success', text: 'Branding saved successfully! Refresh the status page to see changes.' });
    } catch (err) {
      setBrandingMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save branding' });
    } finally {
      setSavingBranding(false);
    }
  };

  const handleLogoUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Check file size (max 500KB)
    if (file.size > 500 * 1024) {
      setBrandingMessage({ type: 'error', text: 'Logo file too large. Maximum size is 500KB.' });
      return;
    }

    // Check file type
    if (!file.type.startsWith('image/')) {
      setBrandingMessage({ type: 'error', text: 'Please select an image file.' });
      return;
    }

    const reader = new FileReader();
    reader.onload = (event) => {
      const dataUrl = event.target?.result as string;
      setBranding({ ...branding, logo: dataUrl });
    };
    reader.readAsDataURL(file);
  };

  const handleRemoveLogo = () => {
    setBranding({ ...branding, logo: '' });
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">Settings</h2>

      <div className="mb-8">
        <h3 className="text-lg font-semibold mb-4">Theme Selection</h3>
        <p className="text-gray-600 mb-4">Choose a theme for your public status page.</p>

        {themeMessage && (
          <div className={`p-4 rounded mb-4 ${themeMessage.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
            {themeMessage.text}
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          {themes.map(theme => (
            <div
              key={theme.id}
              onClick={() => !savingTheme && handleThemeSelect(theme.id)}
              className={`cursor-pointer border-2 rounded-lg overflow-hidden transition-all ${
                selectedTheme === theme.id ? 'border-blue-500 ring-2 ring-blue-200' : 'border-gray-200 hover:border-gray-300'
              } ${savingTheme ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              <div className={`h-24 ${theme.preview}`} />
              <div className="p-4 bg-white">
                <div className="flex items-center justify-between mb-1">
                  <span className="font-semibold">{theme.name}</span>
                  <div className="flex gap-1">
                    {savedTheme === theme.id && (
                      <span className="text-xs bg-green-500 text-white px-2 py-1 rounded">Active</span>
                    )}
                    {selectedTheme === theme.id && selectedTheme !== savedTheme && (
                      <span className="text-xs bg-blue-500 text-white px-2 py-1 rounded">Selected</span>
                    )}
                  </div>
                </div>
                <p className="text-sm text-gray-500">{theme.description}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-4">
          <button
            onClick={handleThemeSave}
            disabled={savingTheme || selectedTheme === savedTheme}
            className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {savingTheme ? 'Saving...' : 'Save Theme'}
          </button>
        </div>
      </div>

      <hr className="my-8" />

      <div className="mb-8">
        <h3 className="text-lg font-semibold mb-4">Email Configuration</h3>
        <p className="text-gray-600 mb-4">Configure email notifications for subscribers.</p>

        {emailMessage && (
          <div className={`p-4 rounded mb-4 ${emailMessage.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
            {emailMessage.text}
          </div>
        )}

        <form onSubmit={handleEmailConfigSave} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Email Provider</label>
            <select
              value={emailConfig.provider}
              onChange={e => setEmailConfig({ ...emailConfig, provider: e.target.value })}
              className="w-full border rounded px-3 py-2"
            >
              <option value="disabled">Disabled</option>
              <option value="smtp">SMTP Server</option>
              <option value="sendgrid">SendGrid</option>
              <option value="mailgun">Mailgun</option>
              <option value="ses">Amazon SES</option>
            </select>
          </div>

          {emailConfig.provider !== 'disabled' && (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">From Address</label>
                  <input
                    type="email"
                    value={emailConfig.from_address}
                    onChange={e => setEmailConfig({ ...emailConfig, from_address: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                    placeholder="noreply@example.com"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">From Name</label>
                  <input
                    type="text"
                    value={emailConfig.from_name}
                    onChange={e => setEmailConfig({ ...emailConfig, from_name: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                    placeholder="Status Page"
                  />
                </div>
              </div>

              {emailConfig.provider === 'smtp' && (
                <div className="bg-gray-50 p-4 rounded space-y-4">
                  <h4 className="font-medium">SMTP Settings</h4>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">SMTP Host</label>
                      <input
                        type="text"
                        value={emailConfig.smtp_host}
                        onChange={e => setEmailConfig({ ...emailConfig, smtp_host: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="smtp.example.com"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">SMTP Port</label>
                      <input
                        type="number"
                        value={emailConfig.smtp_port}
                        onChange={e => setEmailConfig({ ...emailConfig, smtp_port: parseInt(e.target.value) || 587 })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="587"
                      />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Username (optional)</label>
                      <input
                        type="text"
                        value={emailConfig.smtp_username}
                        onChange={e => setEmailConfig({ ...emailConfig, smtp_username: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="username"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Password (optional)</label>
                      <input
                        type="password"
                        value={emailConfig.smtp_password}
                        onChange={e => setEmailConfig({ ...emailConfig, smtp_password: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="********"
                      />
                    </div>
                  </div>
                  <div className="flex items-center">
                    <input
                      type="checkbox"
                      id="smtp_tls"
                      checked={emailConfig.smtp_tls}
                      onChange={e => setEmailConfig({
                        ...emailConfig,
                        smtp_tls: e.target.checked,
                        smtp_port: e.target.checked ? 465 : 587
                      })}
                      className="mr-2"
                    />
                    <label htmlFor="smtp_tls" className="text-sm text-gray-700">
                      Use implicit TLS (port 465)
                    </label>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">
                    When unchecked, STARTTLS will be attempted first, with automatic fallback to plaintext if unavailable.
                  </p>
                </div>
              )}

              {emailConfig.provider === 'sendgrid' && (
                <div className="bg-gray-50 p-4 rounded">
                  <h4 className="font-medium mb-4">SendGrid Settings</h4>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                    <input
                      type="password"
                      value={emailConfig.sendgrid_api_key}
                      onChange={e => setEmailConfig({ ...emailConfig, sendgrid_api_key: e.target.value })}
                      className="w-full border rounded px-3 py-2"
                      placeholder="SG.xxxxx..."
                    />
                  </div>
                </div>
              )}

              {emailConfig.provider === 'mailgun' && (
                <div className="bg-gray-50 p-4 rounded space-y-4">
                  <h4 className="font-medium">Mailgun Settings</h4>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                      <input
                        type="password"
                        value={emailConfig.mailgun_api_key}
                        onChange={e => setEmailConfig({ ...emailConfig, mailgun_api_key: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="key-xxxxx..."
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Domain</label>
                      <input
                        type="text"
                        value={emailConfig.mailgun_domain}
                        onChange={e => setEmailConfig({ ...emailConfig, mailgun_domain: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="mg.example.com"
                      />
                    </div>
                  </div>
                </div>
              )}

              {emailConfig.provider === 'ses' && (
                <div className="bg-gray-50 p-4 rounded space-y-4">
                  <h4 className="font-medium">Amazon SES Settings</h4>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">AWS Region</label>
                    <input
                      type="text"
                      value={emailConfig.aws_region}
                      onChange={e => setEmailConfig({ ...emailConfig, aws_region: e.target.value })}
                      className="w-full border rounded px-3 py-2"
                      placeholder="us-east-1"
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Access Key ID</label>
                      <input
                        type="text"
                        value={emailConfig.aws_access_key}
                        onChange={e => setEmailConfig({ ...emailConfig, aws_access_key: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="AKIAXXXXX..."
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Secret Access Key</label>
                      <input
                        type="password"
                        value={emailConfig.aws_secret_key}
                        onChange={e => setEmailConfig({ ...emailConfig, aws_secret_key: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        placeholder="********"
                      />
                    </div>
                  </div>
                </div>
              )}
            </>
          )}

          <div className="flex gap-4 pt-4">
            <button
              type="submit"
              disabled={savingEmail}
              className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50"
            >
              {savingEmail ? 'Saving...' : 'Save Email Configuration'}
            </button>
          </div>
        </form>

        {emailConfig.provider !== 'disabled' && (
          <div className="mt-6 pt-6 border-t">
            <h4 className="font-medium mb-2">Test Email Configuration</h4>
            <p className="text-sm text-gray-600 mb-4">Send a test email to verify your configuration is working.</p>
            {hasUnsavedEmailChanges && (
              <p className="text-sm text-amber-600 mb-4">Save your email configuration before testing.</p>
            )}
            <div className="flex gap-2">
              <input
                type="email"
                value={testEmail}
                onChange={e => setTestEmail(e.target.value)}
                className="flex-1 border rounded px-3 py-2"
                placeholder="test@example.com"
                disabled={hasUnsavedEmailChanges}
              />
              <button
                type="button"
                onClick={handleTestEmail}
                disabled={testingEmail || hasUnsavedEmailChanges}
                className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {testingEmail ? 'Sending...' : 'Send Test Email'}
              </button>
            </div>
          </div>
        )}
      </div>

      <hr className="my-8" />

      <div className="mb-8">
        <h3 className="text-lg font-semibold mb-4">Branding</h3>
        <p className="text-gray-600 mb-4">Customize the title and logo displayed on your status page.</p>

        {brandingMessage && (
          <div className={`p-4 rounded mb-4 ${brandingMessage.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
            {brandingMessage.text}
          </div>
        )}

        <form onSubmit={handleBrandingSave} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Page Title (English)</label>
              <input
                type="text"
                value={branding.title_en}
                onChange={e => setBranding({ ...branding, title_en: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="System Status"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Page Title (French)</label>
              <input
                type="text"
                value={branding.title_fr}
                onChange={e => setBranding({ ...branding, title_fr: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="État du Système"
              />
            </div>
          </div>
          <p className="text-sm text-gray-500">Leave empty to use the default translations.</p>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Logo</label>
            <div className="flex items-start gap-4">
              <div className="flex-1">
                <input
                  type="file"
                  accept="image/*"
                  onChange={handleLogoUpload}
                  className="w-full border rounded px-3 py-2"
                />
                <p className="text-sm text-gray-500 mt-1">Maximum file size: 500KB. Supported formats: PNG, JPG, SVG, GIF.</p>
              </div>
              {branding.logo && (
                <div className="flex flex-col items-center gap-2">
                  <img src={branding.logo} alt="Logo preview" className="h-16 border rounded p-1" />
                  <button
                    type="button"
                    onClick={handleRemoveLogo}
                    className="text-sm text-red-500 hover:text-red-700"
                  >
                    Remove
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="pt-4">
            <button
              type="submit"
              disabled={savingBranding}
              className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50"
            >
              {savingBranding ? 'Saving...' : 'Save Branding'}
            </button>
          </div>
        </form>
      </div>

      <hr className="my-8" />

      <CustomizationSection />

      <hr className="my-8" />

      <PageLanguageSection />
    </div>
  );
}

function NotificationsPage() {
  const [stats, setStats] = useState<NotificationQueueStats | null>(null);
  const [notifications, setNotifications] = useState<QueuedNotification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const loadData = async () => {
    try {
      setError(null);
      const [statsData, notificationsData] = await Promise.all([
        api.getNotificationStats(),
        api.getRecentNotifications(100),
      ]);
      setStats(statsData);
      setNotifications(notificationsData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load notification data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    // Auto-refresh every 10 seconds
    const interval = setInterval(loadData, 10000);
    return () => clearInterval(interval);
  }, []);

  const handleRetryFailed = async () => {
    setRetrying(true);
    setMessage(null);
    try {
      const result = await api.retryFailedNotifications();
      setMessage({ type: 'success', text: `${result.count} failed notifications queued for retry` });
      loadData();
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to retry notifications' });
    } finally {
      setRetrying(false);
    }
  };

  const getStatusBadge = (status: string) => {
    const colors: Record<string, string> = {
      pending: 'bg-yellow-100 text-yellow-800',
      processing: 'bg-blue-100 text-blue-800',
      sent: 'bg-green-100 text-green-800',
      failed: 'bg-red-100 text-red-800',
    };
    return `px-2 py-1 rounded text-xs font-medium ${colors[status] || 'bg-gray-100'}`;
  };

  if (loading) return <div>Loading...</div>;

  if (error) {
    return (
      <div className="text-center py-8">
        <p className="text-red-500 mb-4">{error}</p>
        <p className="text-gray-500 text-sm">
          The notification service may not be configured. Please configure an email provider in Settings.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-bold">Notification Queue</h2>
        <button
          onClick={loadData}
          className="text-blue-500 hover:text-blue-700 text-sm"
        >
          Refresh
        </button>
      </div>

      {message && (
        <div className={`p-4 rounded mb-6 ${message.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
          {message.text}
        </div>
      )}

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-5 gap-4 mb-6">
          <div className="bg-gray-50 p-4 rounded text-center">
            <div className="text-2xl font-bold text-gray-900">{stats.total}</div>
            <div className="text-sm text-gray-500">Total</div>
          </div>
          <div className="bg-yellow-50 p-4 rounded text-center">
            <div className="text-2xl font-bold text-yellow-600">{stats.pending}</div>
            <div className="text-sm text-yellow-700">Pending</div>
          </div>
          <div className="bg-blue-50 p-4 rounded text-center">
            <div className="text-2xl font-bold text-blue-600">{stats.processing}</div>
            <div className="text-sm text-blue-700">Processing</div>
          </div>
          <div className="bg-green-50 p-4 rounded text-center">
            <div className="text-2xl font-bold text-green-600">{stats.sent}</div>
            <div className="text-sm text-green-700">Sent</div>
          </div>
          <div className="bg-red-50 p-4 rounded text-center">
            <div className="text-2xl font-bold text-red-600">{stats.failed}</div>
            <div className="text-sm text-red-700">Failed</div>
          </div>
        </div>
      )}

      {/* Retry Failed Button */}
      {stats && stats.failed > 0 && (
        <div className="mb-6">
          <button
            onClick={handleRetryFailed}
            disabled={retrying}
            className="bg-orange-500 text-white px-4 py-2 rounded hover:bg-orange-600 disabled:opacity-50"
          >
            {retrying ? 'Retrying...' : `Retry ${stats.failed} Failed Notifications`}
          </button>
        </div>
      )}

      {/* Recent Notifications */}
      <h3 className="text-lg font-semibold mb-4">Recent Notifications</h3>
      {notifications.length === 0 ? (
        <p className="text-gray-500">No notifications in queue yet.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="bg-gray-100">
                <th className="border p-2 text-left">ID</th>
                <th className="border p-2 text-left">Email</th>
                <th className="border p-2 text-left">Subject</th>
                <th className="border p-2 text-left">Status</th>
                <th className="border p-2 text-left">Attempts</th>
                <th className="border p-2 text-left">Created</th>
                <th className="border p-2 text-left">Error</th>
              </tr>
            </thead>
            <tbody>
              {notifications.map(n => (
                <tr key={n.id} className={n.status === 'failed' ? 'bg-red-50' : ''}>
                  <td className="border p-2">{n.id}</td>
                  <td className="border p-2">{n.email}</td>
                  <td className="border p-2 max-w-xs truncate" title={n.subject}>{n.subject}</td>
                  <td className="border p-2">
                    <span className={getStatusBadge(n.status)}>{n.status}</span>
                  </td>
                  <td className="border p-2">{n.attempts}/{n.max_attempts}</td>
                  <td className="border p-2">{new Date(n.created_at).toLocaleString()}</td>
                  <td className="border p-2 max-w-xs truncate text-red-600" title={n.last_error || ''}>
                    {n.last_error || '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function MaintenancesPage() {
  const [maintenances, setMaintenances] = useState<Maintenance[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [templates, setTemplates] = useState<MaintenanceTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [useTemplate, setUseTemplate] = useState(false);
  const [selectedTemplateId, setSelectedTemplateId] = useState<number | null>(null);
  const [templateVariables, setTemplateVariables] = useState<TemplateVariable[]>([]);
  const [variableValues, setVariableValues] = useState<Record<string, string>>({});
  const [preview, setPreview] = useState<{ name_en: string; name_fr: string; message_en: string; message_fr: string } | null>(null);

  interface MaintenanceForm {
    name: string;
    status: 'scheduled' | 'in_progress' | 'completed';
    component_id: number;
    extra_component_ids: number[];
    start_at: string;
    end_at: string;
    name_en: string;
    name_fr: string;
    message_en: string;
    message_fr: string;
  }

  const [form, setForm] = useState<MaintenanceForm>({
    name: '',
    status: 'scheduled',
    component_id: 0,
    extra_component_ids: [],
    start_at: '',
    end_at: '',
    name_en: '',
    name_fr: '',
    message_en: '',
    message_fr: '',
  });

  useEffect(() => { loadData(); }, []);

  const loadData = async () => {
    try {
      const [maintenancesData, componentsData, templatesData] = await Promise.all([
        api.getMaintenances(),
        api.getComponents(),
        api.getMaintenanceTemplates(),
      ]);
      setMaintenances(maintenancesData);
      setComponents(componentsData);
      setTemplates(templatesData);
      if (componentsData.length > 0 && form.component_id === 0) {
        setForm(f => ({ ...f, component_id: componentsData[0].id }));
      }
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setForm({
      name: '',
      status: 'scheduled',
      component_id: components.length > 0 ? components[0].id : 0,
      extra_component_ids: [],
      start_at: '',
      end_at: '',
      name_en: '',
      name_fr: '',
      message_en: '',
      message_fr: '',
    });
    setEditId(null);
    setUseTemplate(false);
    setSelectedTemplateId(null);
    setTemplateVariables([]);
    setVariableValues({});
    setPreview(null);
  };

  const handleTemplateSelect = async (templateId: number) => {
    setSelectedTemplateId(templateId);
    try {
      const { template, parsed_variables } = await api.getMaintenanceTemplate(templateId);
      setTemplateVariables(parsed_variables || []);
      const defaults: Record<string, string> = {};
      (parsed_variables || []).forEach(v => { if (v.default) defaults[v.name] = v.default; });
      setVariableValues(defaults);
      if (template.default_component_id) {
        setForm(f => ({ ...f, component_id: template.default_component_id! }));
      }
    } catch (err) {
      console.error('Failed to load template:', err);
    }
  };

  const handlePreview = async () => {
    if (!selectedTemplateId) return;
    try {
      const { preview } = await api.previewMaintenanceTemplate(selectedTemplateId, variableValues);
      setPreview(preview);
    } catch (err) {
      console.error('Failed to preview:', err);
    }
  };

  const buildTranslations = () => {
    const translations: { locale: string; field: string; value: string }[] = [];
    if (useTemplate && preview) {
      if (preview.name_en) translations.push({ locale: 'en', field: 'name', value: preview.name_en });
      if (preview.name_fr) translations.push({ locale: 'fr', field: 'name', value: preview.name_fr });
      if (preview.message_en) translations.push({ locale: 'en', field: 'message', value: preview.message_en });
      if (preview.message_fr) translations.push({ locale: 'fr', field: 'message', value: preview.message_fr });
    } else {
      if (form.name_en) translations.push({ locale: 'en', field: 'name', value: form.name_en });
      if (form.name_fr) translations.push({ locale: 'fr', field: 'name', value: form.name_fr });
      if (form.message_en) translations.push({ locale: 'en', field: 'message', value: form.message_en });
      if (form.message_fr) translations.push({ locale: 'fr', field: 'message', value: form.message_fr });
    }
    return translations;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = useTemplate && preview ? preview.name_en : form.name;
    const data = {
      name,
      status: form.status,
      component_id: form.component_id,
      component_ids: form.extra_component_ids.filter(id => id !== form.component_id),
      start_at: form.start_at,
      end_at: form.end_at,
      translations: buildTranslations(),
    };
    try {
      if (editId) {
        await api.updateMaintenance(editId, data);
      } else {
        await api.createMaintenance(data);
      }
      setShowForm(false);
      resetForm();
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to save maintenance');
    }
  };

  const handleEdit = (m: Maintenance) => {
    const getTranslation = (field: string, locale: string) =>
      m.translations?.find(t => t.field === field && t.locale === locale)?.value || '';

    setForm({
      name: m.name,
      status: m.status,
      component_id: m.component_id ?? 0,
      extra_component_ids: (m.components || []).map(mc => mc.component_id).filter(id => id !== m.component_id),
      start_at: m.start_at.slice(0, 16),
      end_at: m.end_at.slice(0, 16),
      name_en: getTranslation('name', 'en'),
      name_fr: getTranslation('name', 'fr'),
      message_en: getTranslation('message', 'en'),
      message_fr: getTranslation('message', 'fr'),
    });
    setEditId(m.id);
    setUseTemplate(false);
    setShowForm(true);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this maintenance?')) return;
    try {
      await api.deleteMaintenance(id);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete');
    }
  };

  const getStatusBadge = (status: string) => {
    const colors: Record<string, string> = {
      scheduled: 'bg-blue-100 text-blue-800',
      in_progress: 'bg-yellow-100 text-yellow-800',
      completed: 'bg-green-100 text-green-800',
    };
    return `px-2 py-1 rounded text-xs font-medium ${colors[status] || 'bg-gray-100'}`;
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-bold">Maintenances</h2>
        <button
          onClick={() => { resetForm(); setShowForm(true); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600"
        >
          Schedule Maintenance
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="mb-6 p-4 bg-gray-50 rounded space-y-4">
          <div className="flex gap-4 mb-4">
            <button type="button" onClick={() => setUseTemplate(false)}
              className={`px-4 py-2 rounded ${!useTemplate ? 'bg-blue-500 text-white' : 'bg-gray-200'}`}>
              Free Form
            </button>
            <button type="button" onClick={() => setUseTemplate(true)}
              className={`px-4 py-2 rounded ${useTemplate ? 'bg-blue-500 text-white' : 'bg-gray-200'}`}>
              Use Template
            </button>
          </div>

          {useTemplate ? (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Template</label>
                <select
                  value={selectedTemplateId || ''}
                  onChange={e => handleTemplateSelect(parseInt(e.target.value))}
                  className="w-full border rounded px-3 py-2"
                >
                  <option value="">Select a template...</option>
                  {templates.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                </select>
              </div>

              {templateVariables.length > 0 && (
                <div className="grid grid-cols-2 gap-4">
                  {templateVariables.map(v => (
                    <div key={v.name}>
                      <label className="block text-sm font-medium text-gray-700 mb-1">
                        {v.label_en} {v.required && <span className="text-red-500">*</span>}
                      </label>
                      <input
                        type={v.type === 'datetime' ? 'datetime-local' : v.type === 'number' ? 'number' : 'text'}
                        value={variableValues[v.name] || ''}
                        onChange={e => setVariableValues({ ...variableValues, [v.name]: e.target.value })}
                        className="w-full border rounded px-3 py-2"
                        required={v.required}
                      />
                    </div>
                  ))}
                </div>
              )}

              {selectedTemplateId && (
                <button type="button" onClick={handlePreview} className="bg-gray-500 text-white px-4 py-2 rounded">
                  Preview
                </button>
              )}

              {preview && (
                <div className="p-4 bg-white rounded border">
                  <h4 className="font-medium mb-2">Preview:</h4>
                  <p><strong>EN:</strong> {preview.name_en}</p>
                  <p><strong>FR:</strong> {preview.name_fr}</p>
                  <p className="text-sm text-gray-600 mt-2">{preview.message_en}</p>
                </div>
              )}
            </div>
          ) : (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={e => setForm({ ...form, name: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  required
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name (EN)</label>
                  <input
                    type="text"
                    value={form.name_en}
                    onChange={e => setForm({ ...form, name_en: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name (FR)</label>
                  <input
                    type="text"
                    value={form.name_fr}
                    onChange={e => setForm({ ...form, name_fr: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                  />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Message (EN)</label>
                  <textarea
                    value={form.message_en}
                    onChange={e => setForm({ ...form, message_en: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                    rows={3}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Message (FR)</label>
                  <textarea
                    value={form.message_fr}
                    onChange={e => setForm({ ...form, message_fr: e.target.value })}
                    className="w-full border rounded px-3 py-2"
                    rows={3}
                  />
                </div>
              </div>
            </>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Component</label>
              <select
                value={form.component_id}
                onChange={e => setForm({ ...form, ...mainComponentChange(e.target.value) })}
                className="w-full border rounded px-3 py-2"
              >
                <option value={0}>No component (general)</option>
                {components.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
              <div className="mt-2">
                <ExtraComponentsPicker components={components} mainId={form.component_id} value={form.extra_component_ids}
                  onChange={ids => setForm({ ...form, extra_component_ids: ids })} />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
              <select
                value={form.status}
                onChange={e => setForm({ ...form, status: e.target.value as 'scheduled' | 'in_progress' | 'completed' })}
                className="w-full border rounded px-3 py-2"
              >
                <option value="scheduled">Scheduled</option>
                <option value="in_progress">In Progress</option>
                <option value="completed">Completed</option>
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Start Date/Time</label>
              <input
                type="datetime-local"
                value={form.start_at}
                onChange={e => setForm({ ...form, start_at: e.target.value })}
                className="w-full border rounded px-3 py-2"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">End Date/Time</label>
              <input
                type="datetime-local"
                value={form.end_at}
                onChange={e => setForm({ ...form, end_at: e.target.value })}
                className="w-full border rounded px-3 py-2"
                required
              />
            </div>
          </div>

          <div className="flex gap-2">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => { setShowForm(false); resetForm(); }}
              className="bg-gray-300 px-4 py-2 rounded hover:bg-gray-400">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Component</th>
            <th className="border p-2 text-left">Status</th>
            <th className="border p-2 text-left">Start</th>
            <th className="border p-2 text-left">End</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {maintenances.map(m => (
            <tr key={m.id}>
              <td className="border p-2">{m.name}</td>
              <td className="border p-2">{m.component_id == null ? 'General' : m.component?.name || 'Unknown'}</td>
              <td className="border p-2">
                <span className={getStatusBadge(m.status)}>{m.status.replace('_', ' ')}</span>
              </td>
              <td className="border p-2">{new Date(m.start_at).toLocaleString()}</td>
              <td className="border p-2">{new Date(m.end_at).toLocaleString()}</td>
              <td className="border p-2">
                <button onClick={() => handleEdit(m)} className="text-blue-500 hover:text-blue-700 mr-2">Edit</button>
                <button onClick={() => handleDelete(m.id)} className="text-red-500 hover:text-red-700">Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {maintenances.length === 0 && <p className="text-gray-500 text-center py-4">No maintenances scheduled.</p>}
    </div>
  );
}

function MaintenanceTemplatesPage() {
  const [templates, setTemplates] = useState<MaintenanceTemplate[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState({
    name: '',
    default_component_id: undefined as number | undefined,
    name_en: '',
    name_fr: '',
    message_en: '',
    message_fr: '',
    variables: [] as TemplateVariable[],
  });

  useEffect(() => { loadData(); }, []);

  const loadData = async () => {
    try {
      const [templatesData, componentsData] = await Promise.all([
        api.getMaintenanceTemplates(),
        api.getComponents(),
      ]);
      setTemplates(templatesData);
      setComponents(componentsData);
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setForm({
      name: '',
      default_component_id: undefined,
      name_en: '',
      name_fr: '',
      message_en: '',
      message_fr: '',
      variables: [],
    });
    setEditId(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editId) {
        await api.updateMaintenanceTemplate(editId, form);
      } else {
        await api.createMaintenanceTemplate(form);
      }
      setShowForm(false);
      resetForm();
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to save template');
    }
  };

  const handleEdit = async (t: MaintenanceTemplate) => {
    try {
      const { template, parsed_variables } = await api.getMaintenanceTemplate(t.id);
      setForm({
        name: template.name,
        default_component_id: template.default_component_id || undefined,
        name_en: template.name_en,
        name_fr: template.name_fr,
        message_en: template.message_en,
        message_fr: template.message_fr,
        variables: parsed_variables || [],
      });
      setEditId(t.id);
      setShowForm(true);
    } catch (err) {
      console.error('Failed to load template:', err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this template?')) return;
    try {
      await api.deleteMaintenanceTemplate(id);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete');
    }
  };

  const addVariable = () => {
    setForm({
      ...form,
      variables: [...form.variables, { name: '', label_en: '', label_fr: '', type: 'text', required: false }],
    });
  };

  const updateVariable = (index: number, field: string, value: string | boolean) => {
    const newVars = [...form.variables];
    (newVars[index] as unknown as Record<string, string | boolean | string[] | undefined>)[field] = value;
    setForm({ ...form, variables: newVars });
  };

  const removeVariable = (index: number) => {
    setForm({ ...form, variables: form.variables.filter((_, i) => i !== index) });
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-bold">Maintenance Templates</h2>
        <button
          onClick={() => { resetForm(); setShowForm(true); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600"
        >
          Add Template
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="mb-6 p-4 bg-gray-50 rounded space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Template Name</label>
              <input
                type="text"
                value={form.name}
                onChange={e => setForm({ ...form, name: e.target.value })}
                className="w-full border rounded px-3 py-2"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Default Component</label>
              <select
                value={form.default_component_id || ''}
                onChange={e => setForm({ ...form, default_component_id: e.target.value ? parseInt(e.target.value) : undefined })}
                className="w-full border rounded px-3 py-2"
              >
                <option value="">None</option>
                {components.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name Template (EN)</label>
              <input
                type="text"
                value={form.name_en}
                onChange={e => setForm({ ...form, name_en: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="Scheduled Maintenance - {{service_name}}"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name Template (FR)</label>
              <input
                type="text"
                value={form.name_fr}
                onChange={e => setForm({ ...form, name_fr: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="Maintenance planifiée - {{service_name}}"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Message Template (EN)</label>
              <textarea
                value={form.message_en}
                onChange={e => setForm({ ...form, message_en: e.target.value })}
                className="w-full border rounded px-3 py-2"
                rows={3}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Message Template (FR)</label>
              <textarea
                value={form.message_fr}
                onChange={e => setForm({ ...form, message_fr: e.target.value })}
                className="w-full border rounded px-3 py-2"
                rows={3}
              />
            </div>
          </div>

          <div>
            <div className="flex justify-between items-center mb-2">
              <label className="block text-sm font-medium text-gray-700">Variables</label>
              <button type="button" onClick={addVariable} className="text-blue-500 text-sm hover:text-blue-700">
                + Add Variable
              </button>
            </div>
            {form.variables.map((v, i) => (
              <div key={i} className="grid grid-cols-5 gap-2 mb-2 p-2 bg-white rounded border">
                <input
                  placeholder="name"
                  value={v.name}
                  onChange={e => updateVariable(i, 'name', e.target.value)}
                  className="border rounded px-2 py-1 text-sm"
                />
                <input
                  placeholder="Label EN"
                  value={v.label_en}
                  onChange={e => updateVariable(i, 'label_en', e.target.value)}
                  className="border rounded px-2 py-1 text-sm"
                />
                <input
                  placeholder="Label FR"
                  value={v.label_fr}
                  onChange={e => updateVariable(i, 'label_fr', e.target.value)}
                  className="border rounded px-2 py-1 text-sm"
                />
                <select
                  value={v.type}
                  onChange={e => updateVariable(i, 'type', e.target.value)}
                  className="border rounded px-2 py-1 text-sm"
                >
                  <option value="text">Text</option>
                  <option value="number">Number</option>
                  <option value="datetime">DateTime</option>
                </select>
                <div className="flex items-center gap-2">
                  <label className="text-sm">
                    <input
                      type="checkbox"
                      checked={v.required}
                      onChange={e => updateVariable(i, 'required', e.target.checked)}
                      className="mr-1"
                    />
                    Required
                  </label>
                  <button type="button" onClick={() => removeVariable(i)} className="text-red-500 text-sm">X</button>
                </div>
              </div>
            ))}
          </div>

          <div className="flex gap-2">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => { setShowForm(false); resetForm(); }}
              className="bg-gray-300 px-4 py-2 rounded hover:bg-gray-400">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Name (EN)</th>
            <th className="border p-2 text-left">Variables</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {templates.map(t => (
            <tr key={t.id}>
              <td className="border p-2">{t.name}</td>
              <td className="border p-2 text-sm text-gray-600">{t.name_en}</td>
              <td className="border p-2 text-sm">{t.parsed_variables?.length || 0} variables</td>
              <td className="border p-2">
                <button onClick={() => handleEdit(t)} className="text-blue-500 hover:text-blue-700 mr-2">Edit</button>
                <button onClick={() => handleDelete(t.id)} className="text-red-500 hover:text-red-700">Delete</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {templates.length === 0 && <p className="text-gray-500 text-center py-4">No templates yet.</p>}
    </div>
  );
}

function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [showPasswordReset, setShowPasswordReset] = useState<number | null>(null);
  const [form, setForm] = useState({ username: '', email: '', password: '', role: 'viewer' as 'admin' | 'viewer' });
  const [newPassword, setNewPassword] = useState('');
  const { user: currentUser } = useUser();

  useEffect(() => { loadUsers(); }, []);

  const loadUsers = async () => {
    try {
      const data = await api.getUsers();
      setUsers(data);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editId) {
        await api.updateUser(editId, { email: form.email, role: form.role });
      } else {
        await api.createUser(form);
      }
      setShowForm(false);
      setEditId(null);
      setForm({ username: '', email: '', password: '', role: 'viewer' });
      loadUsers();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error saving user');
    }
  };

  const handleEdit = (u: User) => {
    setForm({ username: u.username, email: u.email, password: '', role: u.role });
    setEditId(u.id);
    setShowForm(true);
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this user?')) return;
    try {
      await api.deleteUser(id);
      loadUsers();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete user');
    }
  };

  const handleToggleActive = async (u: User) => {
    try {
      await api.updateUser(u.id, { active: !u.active });
      loadUsers();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to update user');
    }
  };

  const handleResetTwoFactor = async (u: User) => {
    if (!confirm(`Turn two-factor login off for ${u.username}? Do this only if they lost their phone and their recovery codes.`)) return;
    try {
      await api.resetUserTwoFactor(u.id);
      loadUsers();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to reset two-factor login');
    }
  };

  const handleResetPassword = async (id: number) => {
    if (!newPassword || newPassword.length < 8) {
      alert('Password must be at least 8 characters');
      return;
    }
    try {
      await api.resetUserPassword(id, newPassword);
      setShowPasswordReset(null);
      setNewPassword('');
      alert('Password reset successfully');
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to reset password');
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Users</h2>
        <button onClick={() => { setShowForm(true); setEditId(null); setForm({ username: '', email: '', password: '', role: 'viewer' }); }}
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
          Add User
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-gray-50 p-4 rounded mb-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Username</label>
              <input type="text" value={form.username} onChange={e => setForm({...form, username: e.target.value})}
                className="w-full border rounded px-3 py-2" required disabled={!!editId} />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
              <input type="email" value={form.email} onChange={e => setForm({...form, email: e.target.value})}
                className="w-full border rounded px-3 py-2" required />
            </div>
            {!editId && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                <input type="password" value={form.password} onChange={e => setForm({...form, password: e.target.value})}
                  className="w-full border rounded px-3 py-2" required minLength={8} />
              </div>
            )}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Role</label>
              <select value={form.role} onChange={e => setForm({...form, role: e.target.value as 'admin' | 'viewer'})}
                className="w-full border rounded px-3 py-2">
                <option value="admin">Admin</option>
                <option value="viewer">Viewer</option>
              </select>
            </div>
          </div>
          <div className="flex gap-2">
            <button type="submit" className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
              {editId ? 'Update' : 'Create'}
            </button>
            <button type="button" onClick={() => setShowForm(false)} className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">
              Cancel
            </button>
          </div>
        </form>
      )}

      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-gray-100">
            <th className="border p-2 text-left">Username</th>
            <th className="border p-2 text-left">Email</th>
            <th className="border p-2 text-left">Role</th>
            <th className="border p-2 text-left">Status</th>
            <th className="border p-2 text-left">2FA</th>
            <th className="border p-2 text-left">Last Login</th>
            <th className="border p-2 text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {users.map(u => (
            <tr key={u.id} className={!u.active ? 'bg-gray-100 opacity-60' : ''}>
              <td className="border p-2">{u.username}</td>
              <td className="border p-2">{u.email}</td>
              <td className="border p-2">
                <span className={`px-2 py-1 rounded text-xs ${u.role === 'admin' ? 'bg-purple-100 text-purple-800' : 'bg-gray-100 text-gray-800'}`}>
                  {u.role}
                </span>
              </td>
              <td className="border p-2">
                <span className={`px-2 py-1 rounded text-xs ${u.active ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
                  {u.active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td className="border p-2">{u.two_factor_enabled ? 'On' : 'Off'}</td>
              <td className="border p-2">{u.last_login ? new Date(u.last_login).toLocaleString() : 'Never'}</td>
              <td className="border p-2">
                {u.id !== currentUser?.id && (
                  <>
                    <button onClick={() => handleEdit(u)} className="text-blue-500 mr-2">Edit</button>
                    <button onClick={() => handleToggleActive(u)} className="text-yellow-600 mr-2">
                      {u.active ? 'Deactivate' : 'Activate'}
                    </button>
                    <button onClick={() => setShowPasswordReset(u.id)} className="text-orange-500 mr-2">Reset Password</button>
                    {u.two_factor_enabled && (
                      <button onClick={() => handleResetTwoFactor(u)} className="text-orange-500 mr-2">Reset 2FA</button>
                    )}
                    <button onClick={() => handleDelete(u.id)} className="text-red-500">Delete</button>
                  </>
                )}
                {u.id === currentUser?.id && <span className="text-gray-400">Current User</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {showPasswordReset && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
          <div className="bg-white p-6 rounded-lg shadow-lg w-96">
            <h3 className="text-lg font-bold mb-4">Reset Password</h3>
            <input type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)}
              placeholder="New password (min 8 characters)" className="w-full border rounded px-3 py-2 mb-4" />
            <div className="flex gap-2 justify-end">
              <button onClick={() => { setShowPasswordReset(null); setNewPassword(''); }}
                className="bg-gray-500 text-white px-4 py-2 rounded">Cancel</button>
              <button onClick={() => handleResetPassword(showPasswordReset)}
                className="bg-blue-500 text-white px-4 py-2 rounded">Reset</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState('');
  const [userId, setUserId] = useState<number | ''>('');
  const [newKey, setNewKey] = useState<{ name: string; key: string } | null>(null);

  const load = async () => {
    try {
      const [k, u] = await Promise.all([api.getAPIKeys(), api.getUsers()]);
      setKeys(k);
      setUsers(u.filter(x => x.active));
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to load API keys');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const result = await api.createAPIKey({ name, user_id: userId === '' ? undefined : userId });
      setNewKey({ name: result.api_key.name, key: result.key });
      setName('');
      setUserId('');
      load();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to create API key');
    }
  };

  const handleRevoke = async (k: APIKey) => {
    if (!confirm(`Revoke the API key "${k.name}"? Tools using it will stop working.`)) return;
    try {
      await api.revokeAPIKey(k.id);
      load();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to revoke API key');
    }
  };

  const formatDate = (d?: string) => (d ? new Date(d).toLocaleString() : '-');

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h2 className="text-2xl font-bold mb-2">API Keys</h2>
      <p className="text-gray-600 mb-4">
        Permanent keys for monitoring tools and scripts. A key acts with the role of its user. Send it as
        <code className="mx-1 px-1 bg-gray-100 rounded">Authorization: Bearer &lt;key&gt;</code>
        or, for Cachet clients, as
        <code className="mx-1 px-1 bg-gray-100 rounded">X-Cachet-Token: &lt;key&gt;</code>.
      </p>

      {newKey && (
        <div className="bg-green-50 border border-green-300 rounded p-4 mb-4">
          <p className="font-medium text-green-800 mb-2">
            Key "{newKey.name}" created. Copy it now: it will not be shown again.
          </p>
          <div className="flex gap-2">
            <code className="flex-1 bg-white border rounded px-3 py-2 break-all">{newKey.key}</code>
            <button onClick={() => navigator.clipboard?.writeText(newKey.key)} className="bg-green-600 text-white px-3 py-2 rounded">
              Copy
            </button>
            <button onClick={() => setNewKey(null)} className="border px-3 py-2 rounded">Done</button>
          </div>
        </div>
      )}

      <form onSubmit={handleCreate} className="bg-white p-4 rounded shadow mb-4 flex gap-4 items-end">
        <div className="flex-1">
          <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
          <input type="text" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Zabbix"
            className="w-full border rounded px-3 py-2" required maxLength={255} />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Acts as</label>
          <select value={userId} onChange={e => setUserId(e.target.value === '' ? '' : parseInt(e.target.value))}
            className="border rounded px-3 py-2">
            <option value="">Me</option>
            {users.map(u => <option key={u.id} value={u.id}>{u.username} ({u.role})</option>)}
          </select>
        </div>
        <button type="submit" className="bg-blue-600 text-white px-4 py-2 rounded">Create key</button>
      </form>

      <table className="w-full bg-white rounded shadow">
        <thead>
          <tr className="bg-gray-50">
            <th className="border p-2 text-left">Name</th>
            <th className="border p-2 text-left">Key</th>
            <th className="border p-2 text-left">User</th>
            <th className="border p-2 text-left">Created</th>
            <th className="border p-2 text-left">Last used</th>
            <th className="border p-2 text-left">Status</th>
            <th className="border p-2"></th>
          </tr>
        </thead>
        <tbody>
          {keys.map(k => (
            <tr key={k.id} className={k.revoked_at ? 'text-gray-400' : ''}>
              <td className="border p-2">{k.name}</td>
              <td className="border p-2 font-mono text-sm">{k.prefix}…</td>
              <td className="border p-2">{k.user?.username || k.user_id}</td>
              <td className="border p-2 text-sm">{formatDate(k.created_at)}</td>
              <td className="border p-2 text-sm">{formatDate(k.last_used_at)}</td>
              <td className="border p-2 text-sm">{k.revoked_at ? `Revoked ${formatDate(k.revoked_at)}` : 'Active'}</td>
              <td className="border p-2 text-center">
                {!k.revoked_at && (
                  <button onClick={() => handleRevoke(k)} className="text-red-600 hover:underline">Revoke</button>
                )}
              </td>
            </tr>
          ))}
          {keys.length === 0 && (
            <tr><td colSpan={7} className="border p-4 text-center text-gray-500">No API keys yet.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState({ user_id: undefined as number | undefined, action: '', resource: '' });

  useEffect(() => { loadData(); }, [page, filters]);

  const loadData = async () => {
    try {
      setLoading(true);
      const [logsData, usersData] = await Promise.all([
        api.getAuditLogs({ ...filters, page, limit: 50 }),
        api.getUsers(),
      ]);
      setLogs(logsData.logs);
      setTotal(logsData.total);
      setUsers(usersData);
    } catch (err) {
      console.error('Failed to load audit logs:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleExport = async () => {
    try {
      const blob = await api.exportAuditLogs(filters);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'audit_logs.csv';
      a.click();
      window.URL.revokeObjectURL(url);
    } catch (err) {
      alert('Failed to export audit logs');
    }
  };

  const getActionBadge = (action: string) => {
    const colors: Record<string, string> = {
      login_success: 'bg-green-100 text-green-800',
      login_failed: 'bg-red-100 text-red-800',
      logout: 'bg-gray-100 text-gray-800',
      create: 'bg-blue-100 text-blue-800',
      update: 'bg-yellow-100 text-yellow-800',
      delete: 'bg-red-100 text-red-800',
      read: 'bg-gray-100 text-gray-600',
    };
    return `px-2 py-1 rounded text-xs ${colors[action] || 'bg-gray-100'}`;
  };

  const totalPages = Math.ceil(total / 50);

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-bold">Audit Log</h2>
        <button onClick={handleExport} className="bg-green-500 text-white px-4 py-2 rounded hover:bg-green-600">
          Export CSV
        </button>
      </div>

      <div className="bg-gray-50 p-4 rounded mb-4 grid grid-cols-4 gap-4">
        <div>
          <label className="block text-sm text-gray-600 mb-1">User</label>
          <select value={filters.user_id || ''} onChange={e => { setFilters({ ...filters, user_id: e.target.value ? parseInt(e.target.value) : undefined }); setPage(1); }}
            className="w-full border rounded px-3 py-2">
            <option value="">All Users</option>
            {users.map(u => <option key={u.id} value={u.id}>{u.username}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-sm text-gray-600 mb-1">Action</label>
          <select value={filters.action} onChange={e => { setFilters({ ...filters, action: e.target.value }); setPage(1); }}
            className="w-full border rounded px-3 py-2">
            <option value="">All Actions</option>
            <option value="login_success">Login Success</option>
            <option value="login_failed">Login Failed</option>
            <option value="logout">Logout</option>
            <option value="create">Create</option>
            <option value="update">Update</option>
            <option value="delete">Delete</option>
            <option value="read">Read</option>
          </select>
        </div>
        <div>
          <label className="block text-sm text-gray-600 mb-1">Resource</label>
          <select value={filters.resource} onChange={e => { setFilters({ ...filters, resource: e.target.value }); setPage(1); }}
            className="w-full border rounded px-3 py-2">
            <option value="">All Resources</option>
            <option value="users">Users</option>
            <option value="components">Components</option>
            <option value="incidents">Incidents</option>
            <option value="maintenances">Maintenances</option>
            <option value="subscribers">Subscribers</option>
            <option value="settings">Settings</option>
          </select>
        </div>
        <div className="flex items-end">
          <button onClick={() => { setFilters({ user_id: undefined, action: '', resource: '' }); setPage(1); }}
            className="bg-gray-300 px-4 py-2 rounded hover:bg-gray-400">Clear Filters</button>
        </div>
      </div>

      {loading ? <div>Loading...</div> : (
        <>
          <div className="text-sm text-gray-500 mb-2">Total: {total} entries</div>
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="bg-gray-100">
                <th className="border p-2 text-left">Time</th>
                <th className="border p-2 text-left">User</th>
                <th className="border p-2 text-left">Action</th>
                <th className="border p-2 text-left">Resource</th>
                <th className="border p-2 text-left">IP Address</th>
                <th className="border p-2 text-left">Details</th>
              </tr>
            </thead>
            <tbody>
              {logs.map(log => (
                <tr key={log.id} className={!log.success ? 'bg-red-50' : ''}>
                  <td className="border p-2">{new Date(log.created_at).toLocaleString()}</td>
                  <td className="border p-2">{log.username}</td>
                  <td className="border p-2">
                    <span className={getActionBadge(log.action)}>{log.action.replace('_', ' ')}</span>
                  </td>
                  <td className="border p-2">{log.resource || '-'}{log.resource_id ? ` #${log.resource_id}` : ''}</td>
                  <td className="border p-2">{log.ip_address}</td>
                  <td className="border p-2 max-w-xs truncate" title={log.details || ''}>{log.details || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="flex justify-center gap-2 mt-4">
              <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
                className="px-3 py-1 border rounded disabled:opacity-50">Previous</button>
              <span className="px-3 py-1">Page {page} of {totalPages}</span>
              <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages}
                className="px-3 py-1 border rounded disabled:opacity-50">Next</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// Password validation helper
function validatePasswordStrength(password: string) {
  return {
    minLength: password.length >= 12,
    hasUppercase: /[A-Z]/.test(password),
    hasLowercase: /[a-z]/.test(password),
    hasNumber: /[0-9]/.test(password),
    hasSpecial: /[!@#$%^&*()_+\-=\[\]{}|;':",./<>?`~]/.test(password),
  };
}

function isPasswordValid(validation: ReturnType<typeof validatePasswordStrength>): boolean {
  return Object.values(validation).every(v => v);
}

export default function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const [user, setUser] = useState<UserContextType['user']>(null);

  const [showTwoFactor, setShowTwoFactor] = useState(false);

  // Change password modal state
  const [showChangePassword, setShowChangePassword] = useState(false);
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changeError, setChangeError] = useState('');
  const [changeSuccess, setChangeSuccess] = useState('');
  const [changeLoading, setChangeLoading] = useState(false);
  const [passwordValidation, setPasswordValidation] = useState(validatePasswordStrength(''));

  // Load user from localStorage on mount
  useEffect(() => {
    const storedUser = localStorage.getItem('user');
    if (storedUser) {
      try {
        setUser(JSON.parse(storedUser));
      } catch {
        // Invalid user data
      }
    }
  }, []);

  // Update password validation when newPassword changes
  useEffect(() => {
    setPasswordValidation(validatePasswordStrength(newPassword));
  }, [newPassword]);

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setChangeError('');
    setChangeSuccess('');

    if (!isPasswordValid(passwordValidation)) {
      setChangeError('Please ensure your password meets all requirements');
      return;
    }

    if (newPassword !== confirmPassword) {
      setChangeError('Passwords do not match');
      return;
    }

    setChangeLoading(true);
    try {
      await api.changeOwnPassword(oldPassword, newPassword);
      setChangeSuccess('Password changed successfully!');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setTimeout(() => {
        setShowChangePassword(false);
        setChangeSuccess('');
      }, 2000);
    } catch (err) {
      setChangeError(err instanceof Error ? err.message : 'Failed to change password');
    } finally {
      setChangeLoading(false);
    }
  };

  const closeChangePasswordModal = () => {
    setShowChangePassword(false);
    setOldPassword('');
    setNewPassword('');
    setConfirmPassword('');
    setChangeError('');
    setChangeSuccess('');
  };

  const isAdmin = user?.role === 'admin';

  const baseTabs = [
    { path: '/', label: 'Components' },
    { path: '/incidents', label: 'Incidents' },
    { path: '/templates', label: 'Templates' },
    { path: '/maintenances', label: 'Maintenances' },
    { path: '/maint-templates', label: 'Maint. Templates' },
    { path: '/subscribers', label: 'Subscribers' },
    { path: '/notifications', label: 'Notifications' },
    { path: '/settings', label: 'Settings' },
  ];

  const adminTabs = [
    { path: '/users', label: 'Users' },
    { path: '/api-keys', label: 'API Keys' },
    { path: '/audit', label: 'Audit Log' },
  ];

  const tabs = isAdmin ? [...baseTabs, ...adminTabs] : baseTabs;

  return (
    <UserContext.Provider value={{ user, setUser, isAdmin }}>
      <div className="min-h-screen bg-gray-100">
        <header className="bg-white shadow">
          <div className="max-w-7xl mx-auto px-4 py-4 flex justify-between items-center">
            <h1 className="text-xl font-bold">Status Page Admin</h1>
            <div className="flex items-center gap-4">
              {user && (
                <span className="text-sm text-gray-600">
                  {user.username}
                  <span className={`ml-2 px-2 py-1 rounded text-xs ${user.role === 'admin' ? 'bg-purple-100 text-purple-800' : 'bg-gray-100 text-gray-600'}`}>
                    {user.role}
                  </span>
                </span>
              )}
              <button onClick={() => setShowChangePassword(true)} className="text-blue-500 hover:text-blue-700">
                Change Password
              </button>
              <button onClick={() => setShowTwoFactor(true)} className="text-blue-500 hover:text-blue-700">
                Two-Factor Login
              </button>
              <button onClick={handleLogout} className="text-red-500 hover:text-red-700">
                Logout
              </button>
            </div>
          </div>
        </header>

        <div className="max-w-7xl mx-auto px-4 py-6">
          <nav className="flex gap-4 mb-6 flex-wrap">
            {tabs.map(tab => (
              <Link key={tab.path} to={tab.path}
                className={`px-4 py-2 rounded ${location.pathname === tab.path ? 'bg-blue-500 text-white' : 'bg-white hover:bg-gray-50'}`}>
                {tab.label}
              </Link>
            ))}
          </nav>

          <div className="bg-white rounded-lg shadow p-6">
            <Routes>
              <Route path="/" element={<ComponentsPage />} />
              <Route path="/incidents" element={<IncidentsPage />} />
              <Route path="/templates" element={<TemplatesPage />} />
              <Route path="/maintenances" element={<MaintenancesPage />} />
              <Route path="/maint-templates" element={<MaintenanceTemplatesPage />} />
              <Route path="/subscribers" element={<SubscribersPage />} />
              <Route path="/notifications" element={<NotificationsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              {isAdmin && <Route path="/users" element={<UsersPage />} />}
              {isAdmin && <Route path="/api-keys" element={<APIKeysPage />} />}
              {isAdmin && <Route path="/audit" element={<AuditPage />} />}
            </Routes>
          </div>
        </div>

        {showTwoFactor && <TwoFactorModal onClose={() => setShowTwoFactor(false)} />}

        {/* Change Password Modal */}
        {showChangePassword && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
              <div className="flex justify-between items-center mb-4">
                <h2 className="text-xl font-bold">Change Password</h2>
                <button onClick={closeChangePasswordModal} className="text-gray-500 hover:text-gray-700">
                  &times;
                </button>
              </div>

              {changeError && (
                <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
                  {changeError}
                </div>
              )}

              {changeSuccess && (
                <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
                  {changeSuccess}
                </div>
              )}

              <form onSubmit={handleChangePassword}>
                <div className="mb-4">
                  <label className="block text-gray-700 text-sm font-bold mb-2">
                    Current Password
                  </label>
                  <input
                    type="password"
                    value={oldPassword}
                    onChange={(e) => setOldPassword(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500"
                    required
                  />
                </div>

                <div className="mb-4">
                  <label className="block text-gray-700 text-sm font-bold mb-2">
                    New Password
                  </label>
                  <input
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500"
                    required
                  />
                </div>

                <div className="mb-4 p-3 bg-gray-50 rounded">
                  <p className="text-sm font-semibold text-gray-700 mb-2">Password Requirements:</p>
                  <ul className="text-sm space-y-1">
                    <li className={`flex items-center gap-2 ${passwordValidation.minLength ? 'text-green-600' : 'text-gray-500'}`}>
                      <span>{passwordValidation.minLength ? '\u2713' : '\u2717'}</span>
                      <span>At least 12 characters</span>
                    </li>
                    <li className={`flex items-center gap-2 ${passwordValidation.hasUppercase ? 'text-green-600' : 'text-gray-500'}`}>
                      <span>{passwordValidation.hasUppercase ? '\u2713' : '\u2717'}</span>
                      <span>At least one uppercase letter</span>
                    </li>
                    <li className={`flex items-center gap-2 ${passwordValidation.hasLowercase ? 'text-green-600' : 'text-gray-500'}`}>
                      <span>{passwordValidation.hasLowercase ? '\u2713' : '\u2717'}</span>
                      <span>At least one lowercase letter</span>
                    </li>
                    <li className={`flex items-center gap-2 ${passwordValidation.hasNumber ? 'text-green-600' : 'text-gray-500'}`}>
                      <span>{passwordValidation.hasNumber ? '\u2713' : '\u2717'}</span>
                      <span>At least one number</span>
                    </li>
                    <li className={`flex items-center gap-2 ${passwordValidation.hasSpecial ? 'text-green-600' : 'text-gray-500'}`}>
                      <span>{passwordValidation.hasSpecial ? '\u2713' : '\u2717'}</span>
                      <span>At least one special character</span>
                    </li>
                  </ul>
                </div>

                <div className="mb-6">
                  <label className="block text-gray-700 text-sm font-bold mb-2">
                    Confirm New Password
                  </label>
                  <input
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className={`w-full px-3 py-2 border rounded focus:outline-none focus:border-blue-500 ${
                      confirmPassword && newPassword !== confirmPassword ? 'border-red-500' : 'border-gray-300'
                    }`}
                    required
                  />
                  {confirmPassword && newPassword !== confirmPassword && (
                    <p className="text-red-500 text-sm mt-1">Passwords do not match</p>
                  )}
                </div>

                <div className="flex gap-2">
                  <button
                    type="submit"
                    disabled={changeLoading || !isPasswordValid(passwordValidation) || newPassword !== confirmPassword}
                    className="flex-1 bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50"
                  >
                    {changeLoading ? 'Changing...' : 'Change Password'}
                  </button>
                  <button
                    type="button"
                    onClick={closeChangePasswordModal}
                    className="bg-gray-500 text-white py-2 px-4 rounded hover:bg-gray-600"
                  >
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </UserContext.Provider>
  );
}

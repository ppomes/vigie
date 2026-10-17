import { useEffect, useState } from 'react';
import { api, LanguageSetting } from '../lib/api';

// PageLanguageSection sets the public page's default language, and whether
// the page follows the visitor's browser.
export default function PageLanguageSection() {
  const [form, setForm] = useState<LanguageSetting | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    api.getLanguageSetting()
      .then(setForm)
      .catch(err => setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to load the language setting' }));
  }, []);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form) return;
    setSaving(true);
    setMessage(null);
    try {
      setForm(await api.updateLanguageSetting(form));
      setMessage({ type: 'success', text: 'Language setting saved.' });
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save the language setting' });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mb-8">
      <h3 className="text-lg font-semibold mb-4">Public Page Language</h3>
      <p className="text-gray-600 mb-4">
        Visitors can always switch between English and French; their choice is remembered. This sets what they see
        before they choose. New subscribers also get the default language for their emails unless they pick another.
      </p>

      {message && (
        <div className={`p-4 rounded mb-4 ${message.type === 'success' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
          {message.text}
        </div>
      )}

      {form && (
        <form onSubmit={save} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Default language</label>
            <select value={form.default} onChange={e => setForm({ ...form, default: e.target.value })} className="border rounded px-3 py-2">
              <option value="en">English</option>
              <option value="fr">French</option>
            </select>
          </div>
          <div className="space-y-2">
            <label className="flex items-start gap-2">
              <input type="radio" className="mt-1" checked={form.mode === 'fixed'} onChange={() => setForm({ ...form, mode: 'fixed' })} />
              <span>
                <span className="font-medium">Always the default language.</span>{' '}
                <span className="text-gray-600">Best when incidents are written in one language, as after a Cachet import.</span>
              </span>
            </label>
            <label className="flex items-start gap-2">
              <input type="radio" className="mt-1" checked={form.mode === 'browser'} onChange={() => setForm({ ...form, mode: 'browser' })} />
              <span>
                <span className="font-medium">Follow the browser.</span>{' '}
                <span className="text-gray-600">English or French as the visitor's browser asks, the default language otherwise. Best when incidents are written in both languages.</span>
              </span>
            </label>
          </div>
          <button type="submit" disabled={saving} className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50">
            {saving ? 'Saving...' : 'Save Language'}
          </button>
        </form>
      )}
    </div>
  );
}

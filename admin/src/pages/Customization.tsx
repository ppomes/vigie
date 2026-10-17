import { useEffect, useState } from 'react';
import { api, Customization } from '../lib/api';

const empty: Customization = { css: '', header_html: '', footer_html: '' };

// CustomizationSection edits the custom CSS, header and footer of the
// public page.
export default function CustomizationSection() {
  const [form, setForm] = useState<Customization>(empty);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'warning'; text: string } | null>(null);

  useEffect(() => {
    api.getCustomization()
      .then(setForm)
      .catch(err => setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to load the customization' }));
  }, []);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      const saved = await api.updateCustomization(form);
      const changed = saved.header_html !== form.header_html || saved.footer_html !== form.footer_html;
      setForm(saved);
      setMessage(changed
        ? { type: 'warning', text: 'Saved, but some HTML was changed or removed: scripts, event handlers, forms and frames are not allowed. The fields show what was saved.' }
        : { type: 'success', text: 'Customization saved. Refresh the status page to see it.' });
    } catch (err) {
      setMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save the customization' });
    } finally {
      setSaving(false);
    }
  };

  const colors = { success: 'bg-green-100 text-green-700', warning: 'bg-yellow-100 text-yellow-800', error: 'bg-red-100 text-red-700' };
  const area = 'w-full border rounded px-3 py-2 font-mono text-sm';

  return (
    <div className="mb-8">
      <h3 className="text-lg font-semibold mb-4">Customization</h3>
      <p className="text-gray-600 mb-4">
        Custom CSS, header and footer for the status page. The CSS comes after the theme, so its rules win; theme
        colours are CSS variables, such as <code>:root {'{'} --color-operational: #00aa55; {'}'}</code>.
      </p>

      {message && <div className={`p-4 rounded mb-4 ${colors[message.type]}`}>{message.text}</div>}

      <form onSubmit={save} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Custom CSS</label>
          <textarea value={form.css} onChange={e => setForm({ ...form, css: e.target.value })} rows={10} className={area}
            placeholder=".card { border-radius: 0; }" spellCheck={false} />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Header HTML</label>
          <textarea value={form.header_html} onChange={e => setForm({ ...form, header_html: e.target.value })} rows={5} className={area}
            placeholder='<a href="https://example.com">Back to example.com</a>' spellCheck={false} />
          <p className="text-xs text-gray-500 mt-1">Shown above the page.</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Footer HTML</label>
          <textarea value={form.footer_html} onChange={e => setForm({ ...form, footer_html: e.target.value })} rows={5} className={area}
            spellCheck={false} />
          <p className="text-xs text-gray-500 mt-1">
            Shown below the page. Links, images, text formatting and inline styles are allowed; scripts, event
            handlers, forms and frames are removed.
          </p>
        </div>
        <button type="submit" disabled={saving} className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50">
          {saving ? 'Saving...' : 'Save Customization'}
        </button>
      </form>
    </div>
  );
}

import { useEffect, useState } from 'react';
import { api, TwoFactorSetup, User } from '../lib/api';

// TwoFactorModal lets the current user turn two-factor login on or off, and
// make new recovery codes.
export default function TwoFactorModal({ onClose }: { onClose: () => void }) {
  const [me, setMe] = useState<User | null>(null);
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [setup, setSetup] = useState<TwoFactorSetup | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const load = () => api.getCurrentUser().then(setMe).catch(err => setError(err.message));
  useEffect(() => { load(); }, []);

  const run = async (action: () => Promise<void>) => {
    setError('');
    setBusy(true);
    try {
      await action();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed');
    } finally {
      setBusy(false);
    }
  };

  const start = (e: React.FormEvent) => {
    e.preventDefault();
    run(async () => {
      setSetup(await api.setupTwoFactor(password));
      setPassword('');
    });
  };

  const confirmSetup = (e: React.FormEvent) => {
    e.preventDefault();
    run(async () => {
      setRecoveryCodes(await api.enableTwoFactor(code));
      setSetup(null);
      setCode('');
      await load();
    });
  };

  const newCodes = () => run(async () => {
    setRecoveryCodes(await api.regenerateRecoveryCodes(password));
    setPassword('');
    await load();
  });

  const turnOff = () => {
    if (!confirm('Turn two-factor login off? Login will only ask for your password.')) return;
    run(async () => {
      await api.disableTwoFactor(password);
      setPassword('');
      await load();
    });
  };

  const download = () => {
    const blob = new Blob([recoveryCodes!.join('\n') + '\n'], { type: 'text/plain' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'vigie-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const input = 'w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500';
  const passwordField = (
    <div className="mb-4">
      <label className="block text-gray-700 text-sm font-bold mb-2">Current Password</label>
      <input type="password" value={password} onChange={e => setPassword(e.target.value)} className={input} required />
    </div>
  );

  let body;
  if (recoveryCodes) {
    body = (
      <div>
        <p className="text-gray-600 mb-4">
          Save these recovery codes somewhere safe, such as a password manager. Each one lets you log in once
          without your phone. They are shown only now.
        </p>
        <ul className="grid grid-cols-2 gap-2 font-mono text-sm bg-gray-50 p-4 rounded mb-4">
          {recoveryCodes.map(c => <li key={c}>{c}</li>)}
        </ul>
        <div className="flex gap-2 justify-end">
          <button onClick={download} className="bg-gray-500 text-white px-4 py-2 rounded hover:bg-gray-600">Download</button>
          <button onClick={() => setRecoveryCodes(null)} className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600">
            I saved them
          </button>
        </div>
      </div>
    );
  } else if (setup) {
    body = (
      <form onSubmit={confirmSetup}>
        <p className="text-gray-600 mb-4">
          Scan this QR code with your authenticator app (Google Authenticator, 1Password, Aegis...), then enter the
          code it shows.
        </p>
        <img src={setup.qr_code} alt="QR code for your authenticator app" className="mx-auto mb-2 w-48 h-48" />
        <p className="text-xs text-gray-500 text-center mb-4">
          Or type this key: <span className="font-mono select-all break-all">{setup.secret}</span>
        </p>
        <div className="mb-4">
          <label className="block text-gray-700 text-sm font-bold mb-2">Code</label>
          <input type="text" value={code} onChange={e => setCode(e.target.value)} className={`${input} tracking-widest`}
            autoComplete="one-time-code" inputMode="numeric" autoFocus required />
        </div>
        <button type="submit" disabled={busy} className="w-full bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50">
          Turn on
        </button>
      </form>
    );
  } else if (me?.two_factor_enabled) {
    body = (
      <div>
        <p className="text-green-700 mb-2">Two-factor login is on.</p>
        <p className="text-gray-600 mb-4">
          {me.recovery_codes_left ?? 0} recovery codes left.
          {(me.recovery_codes_left ?? 0) < 3 && ' Make new ones before you run out.'}
        </p>
        {passwordField}
        <div className="flex gap-2 justify-end">
          <button onClick={newCodes} disabled={busy || !password} className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50">
            New recovery codes
          </button>
          <button onClick={turnOff} disabled={busy || !password} className="bg-red-500 text-white px-4 py-2 rounded hover:bg-red-600 disabled:opacity-50">
            Turn off
          </button>
        </div>
      </div>
    );
  } else if (me) {
    body = (
      <form onSubmit={start}>
        <p className="text-gray-600 mb-4">
          With two-factor login, logging in also asks for a code from an app on your phone, so a stolen password is
          not enough.
        </p>
        {passwordField}
        <button type="submit" disabled={busy} className="w-full bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50">
          Set up
        </button>
      </form>
    );
  } else {
    body = <p className="text-gray-500">Loading...</p>;
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-bold">Two-Factor Login</h2>
          {!recoveryCodes && (
            <button onClick={onClose} className="text-gray-500 hover:text-gray-700">&times;</button>
          )}
        </div>
        {error && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">{error}</div>
        )}
        {body}
      </div>
    </div>
  );
}

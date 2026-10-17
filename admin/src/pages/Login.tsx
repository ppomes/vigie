import { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api, APIError } from '../lib/api';

interface PasswordValidation {
  minLength: boolean;
  hasUppercase: boolean;
  hasLowercase: boolean;
  hasNumber: boolean;
  hasSpecial: boolean;
}

function validatePassword(password: string): PasswordValidation {
  return {
    minLength: password.length >= 12,
    hasUppercase: /[A-Z]/.test(password),
    hasLowercase: /[a-z]/.test(password),
    hasNumber: /[0-9]/.test(password),
    hasSpecial: /[!@#$%^&*()_+\-=\[\]{}|;':",./<>?`~]/.test(password),
  };
}

function isPasswordValid(validation: PasswordValidation): boolean {
  return Object.values(validation).every(v => v);
}

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  // Second step for users with two-factor login
  const [needCode, setNeedCode] = useState(false);
  const [code, setCode] = useState('');

  // Force password change state
  const [showChangePassword, setShowChangePassword] = useState(false);
  const [tempPassword, setTempPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changeError, setChangeError] = useState('');
  const [changeLoading, setChangeLoading] = useState(false);
  const [validation, setValidation] = useState<PasswordValidation>({
    minLength: false,
    hasUppercase: false,
    hasLowercase: false,
    hasNumber: false,
    hasSpecial: false,
  });

  useEffect(() => {
    setValidation(validatePassword(newPassword));
  }, [newPassword]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const { token, user, force_password_change, recovery_codes_left } =
        await api.login(username, password, needCode ? code : undefined);
      localStorage.setItem('token', token);
      localStorage.setItem('user', JSON.stringify(user));

      if (recovery_codes_left !== undefined) {
        alert(`You logged in with a recovery code. ${recovery_codes_left} left: make new ones from "Two-Factor Login" if you are running out.`);
      }

      if (force_password_change) {
        // Store the password for the change request
        setTempPassword(password);
        setShowChangePassword(true);
      } else {
        navigate('/');
      }
    } catch (err) {
      if (err instanceof APIError && err.data.two_factor_required && !needCode) {
        // Password accepted: ask for the code.
        setNeedCode(true);
      } else {
        setError(err instanceof Error ? err.message : 'Login failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setChangeError('');

    if (!isPasswordValid(validation)) {
      setChangeError('Please ensure your password meets all requirements');
      return;
    }

    if (newPassword !== confirmPassword) {
      setChangeError('Passwords do not match');
      return;
    }

    setChangeLoading(true);

    try {
      await api.changeOwnPassword(tempPassword, newPassword);
      // Update localStorage to reflect force_password_change is now false
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      user.force_password_change = false;
      localStorage.setItem('user', JSON.stringify(user));
      navigate('/');
    } catch (err) {
      setChangeError(err instanceof Error ? err.message : 'Password change failed');
    } finally {
      setChangeLoading(false);
    }
  };

  const ValidationItem = ({ valid, text }: { valid: boolean; text: string }) => (
    <li className={`flex items-center gap-2 ${valid ? 'text-green-600' : 'text-gray-500'}`}>
      <span>{valid ? '\u2713' : '\u2717'}</span>
      <span>{text}</span>
    </li>
  );

  // Force password change modal
  if (showChangePassword) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-100">
        <div className="bg-white p-8 rounded-lg shadow-md w-full max-w-md">
          <h1 className="text-2xl font-bold text-center mb-4">Password Change Required</h1>
          <p className="text-gray-600 mb-4 text-center">
            Your password has been reset by an administrator. You must set a new password before continuing.
          </p>

          {changeError && (
            <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
              {changeError}
            </div>
          )}

          <form onSubmit={handleChangePassword}>
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
                <ValidationItem valid={validation.minLength} text="At least 12 characters" />
                <ValidationItem valid={validation.hasUppercase} text="At least one uppercase letter" />
                <ValidationItem valid={validation.hasLowercase} text="At least one lowercase letter" />
                <ValidationItem valid={validation.hasNumber} text="At least one number" />
                <ValidationItem valid={validation.hasSpecial} text="At least one special character" />
              </ul>
            </div>

            <div className="mb-6">
              <label className="block text-gray-700 text-sm font-bold mb-2">
                Confirm Password
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

            <button
              type="submit"
              disabled={changeLoading || !isPasswordValid(validation) || newPassword !== confirmPassword}
              className="w-full bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50"
            >
              {changeLoading ? 'Changing...' : 'Change Password'}
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-100">
      <div className="bg-white p-8 rounded-lg shadow-md w-full max-w-md">
        <h1 className="text-2xl font-bold text-center mb-6">Admin Login</h1>

        {error && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
            {error}
          </div>
        )}

        {needCode && (
          <form onSubmit={handleSubmit}>
            <p className="text-gray-600 mb-4">
              Enter the code from your authenticator app, or one of your recovery codes.
            </p>
            <div className="mb-6">
              <label className="block text-gray-700 text-sm font-bold mb-2">
                Code
              </label>
              <input
                type="text"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500 tracking-widest"
                autoComplete="one-time-code"
                inputMode="text"
                autoFocus
                required
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="w-full bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50"
            >
              {loading ? 'Checking...' : 'Verify'}
            </button>
            <button
              type="button"
              onClick={() => { setNeedCode(false); setCode(''); setPassword(''); setError(''); }}
              className="w-full mt-2 text-sm text-gray-500 hover:text-gray-700"
            >
              Back
            </button>
          </form>
        )}

        {!needCode && (
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-gray-700 text-sm font-bold mb-2">
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500"
              required
            />
          </div>

          <div className="mb-4">
            <label className="block text-gray-700 text-sm font-bold mb-2">
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:border-blue-500"
              required
            />
          </div>

          <div className="mb-6 text-right">
            <Link to="/forgot-password" className="text-sm text-blue-500 hover:text-blue-700">
              Forgot password?
            </Link>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-blue-500 text-white py-2 px-4 rounded hover:bg-blue-600 disabled:opacity-50"
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>
        )}
      </div>
    </div>
  );
}

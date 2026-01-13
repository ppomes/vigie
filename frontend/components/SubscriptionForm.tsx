'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { subscribe } from '@/lib/api';

export default function SubscriptionForm() {
  const t = useTranslations('subscription');

  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validateEmail = (email: string): boolean => {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Reset states
    setError(null);
    setSuccess(false);

    // Validate email
    if (!email.trim()) {
      setError(t('invalidEmail'));
      return;
    }

    if (!validateEmail(email)) {
      setError(t('invalidEmail'));
      return;
    }

    try {
      setLoading(true);
      await subscribe(email.trim().toLowerCase());
      setSuccess(true);
      setEmail('');
    } catch (err) {
      if (err instanceof Error) {
        // Check for specific error messages
        if (err.message.includes('already subscribed') || err.message.includes('duplicate')) {
          setError(t('alreadySubscribed'));
        } else {
          setError(err.message);
        }
      } else {
        setError(t('errorMessage'));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h2 className="text-2xl font-bold text-gray-900 mb-2">
        {t('title')}
      </h2>
      <p className="text-gray-600 mb-4">
        {t('description')}
      </p>

      {success && (
        <div className="bg-green-50 border border-green-200 rounded-md p-4 mb-4">
          <h3 className="text-green-800 font-semibold mb-1">
            {t('successTitle')}
          </h3>
          <p className="text-green-700 text-sm">
            {t('successMessage')}
          </p>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-4">
          <h3 className="text-red-800 font-semibold mb-1">
            {t('errorTitle')}
          </h3>
          <p className="text-red-700 text-sm">{error}</p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-3">
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder={t('emailPlaceholder')}
          disabled={loading}
          className="flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:bg-gray-100 disabled:cursor-not-allowed"
          aria-label={t('emailPlaceholder')}
        />
        <button
          type="submit"
          disabled={loading || !email.trim()}
          className="px-6 py-2 bg-blue-600 text-white font-semibold rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
        >
          {loading ? t('subscribing') : t('subscribeButton')}
        </button>
      </form>
    </div>
  );
}

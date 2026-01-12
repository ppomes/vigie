'use client';

import { useLocale } from 'next-intl';
import { usePathname, useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';

export default function LanguageSwitcher() {
  const t = useTranslations('language');
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();

  const switchLocale = (newLocale: string) => {
    // Remove the current locale prefix and add the new one
    const path = pathname.replace(`/${locale}`, `/${newLocale}`);
    router.push(path);
  };

  return (
    <div className="flex items-center space-x-2">
      <span className="text-sm text-gray-600">{t('switchTo')}:</span>
      <button
        onClick={() => switchLocale('en')}
        className={`px-3 py-1 rounded-md text-sm font-medium transition-colors ${
          locale === 'en'
            ? 'bg-blue-600 text-white'
            : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
        }`}
        aria-label={t('english')}
        disabled={locale === 'en'}
      >
        EN
      </button>
      <button
        onClick={() => switchLocale('fr')}
        className={`px-3 py-1 rounded-md text-sm font-medium transition-colors ${
          locale === 'fr'
            ? 'bg-blue-600 text-white'
            : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
        }`}
        aria-label={t('french')}
        disabled={locale === 'fr'}
      >
        FR
      </button>
    </div>
  );
}

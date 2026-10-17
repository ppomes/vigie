import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import en from './en.json';
import fr from './fr.json';
import type { LanguageSetting } from '../lib/api';

const CHOSEN_KEY = 'vigie-lang';
export const SUPPORTED = ['en', 'fr'];

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      fr: { translation: fr }
    },
    supportedLngs: ['en', 'fr'],
    fallbackLng: 'en',
    detection: {
      // The visitor's own choice (the EN/FR switcher), then the browser.
      // Detection does not store its result: a stored value always means
      // the visitor chose it.
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: CHOSEN_KEY,
      caches: [],
      // fr-FR -> fr
      convertDetectedLanguage: (lng: string) => lng.split('-')[0],
    },
    interpolation: {
      escapeValue: false
    }
  });

// chooseLanguage switches the page to lng and remembers it as the visitor's
// choice.
export function chooseLanguage(lng: string) {
  try {
    localStorage.setItem(CHOSEN_KEY, lng);
  } catch {
    // Private mode: the choice lasts for this page only.
  }
  i18n.changeLanguage(lng);
}

function chosenLanguage(): string | null {
  try {
    return localStorage.getItem(CHOSEN_KEY);
  } catch {
    return null;
  }
}

// applyPageLanguage applies the admin's language setting to a visitor who
// has not chosen a language: the default in fixed mode, and in browser mode
// when the browser asks for a language the page does not have.
export function applyPageLanguage(setting: LanguageSetting) {
  if (chosenLanguage()) return;
  if (setting.mode === 'fixed' || !SUPPORTED.includes(i18n.language)) {
    i18n.changeLanguage(setting.default);
  }
}

export default i18n;

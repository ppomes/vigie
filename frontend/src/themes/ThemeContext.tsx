import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { Theme, getThemeById, themes } from './index';
import { getTheme as fetchTheme } from '../lib/api';
import { setThemeCSS } from './styles';

interface ThemeContextType {
  theme: Theme;
  themeId: string;
  loading: boolean;
}

const ThemeContext = createContext<ThemeContextType>({
  theme: themes.default,
  themeId: 'default',
  loading: true,
});

export function useTheme() {
  return useContext(ThemeContext);
}

function applyThemeCSS(theme: Theme) {
  const { colors, styles } = theme;
  const vars: Record<string, string> = {
    '--color-background': colors.background,
    '--color-card': colors.card,
    '--color-card-border': colors.cardBorder,
    '--color-text': colors.text,
    '--color-text-muted': colors.textMuted,
    '--color-primary': colors.primary,
    '--color-primary-hover': colors.primaryHover,
    '--color-header': colors.header,
    '--color-header-text': colors.headerText,

    // Status colors
    '--color-operational': colors.operational,
    '--color-degraded': colors.degraded,
    '--color-partial-outage': colors.partialOutage,
    '--color-major-outage': colors.majorOutage,
    '--color-investigating': colors.investigating,
    '--color-identified': colors.identified,
    '--color-monitoring': colors.monitoring,
    '--color-resolved': colors.resolved,

    // Styles
    '--card-radius': styles.cardRadius,
    '--card-shadow': styles.shadow,
  };

  // A style element rather than inline styles on <html>, so that the
  // admin's custom CSS can override these variables.
  const decls = Object.entries(vars).map(([name, value]) => `  ${name}: ${value};`).join('\n');
  setThemeCSS(`:root {\n${decls}\n}\nbody {\n  background-color: var(--color-background);\n  color: var(--color-text);\n}\n`);
}

interface ThemeProviderProps {
  children: ReactNode;
}

export function ThemeProvider({ children }: ThemeProviderProps) {
  const [themeId, setThemeId] = useState('default');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchTheme()
      .then((id) => {
        setThemeId(id);
        applyThemeCSS(getThemeById(id));
      })
      .catch(() => {
        applyThemeCSS(themes.default);
      })
      .finally(() => setLoading(false));
  }, []);

  const theme = getThemeById(themeId);

  return (
    <ThemeContext.Provider value={{ theme, themeId, loading }}>
      {children}
    </ThemeContext.Provider>
  );
}

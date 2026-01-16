import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { Theme, getThemeById, themes } from './index';
import { getTheme as fetchTheme } from '../lib/api';

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
  const root = document.documentElement;
  const { colors, styles } = theme;

  // Apply colors as CSS variables
  root.style.setProperty('--color-background', colors.background);
  root.style.setProperty('--color-card', colors.card);
  root.style.setProperty('--color-card-border', colors.cardBorder);
  root.style.setProperty('--color-text', colors.text);
  root.style.setProperty('--color-text-muted', colors.textMuted);
  root.style.setProperty('--color-primary', colors.primary);
  root.style.setProperty('--color-primary-hover', colors.primaryHover);
  root.style.setProperty('--color-header', colors.header);
  root.style.setProperty('--color-header-text', colors.headerText);

  // Status colors
  root.style.setProperty('--color-operational', colors.operational);
  root.style.setProperty('--color-degraded', colors.degraded);
  root.style.setProperty('--color-partial-outage', colors.partialOutage);
  root.style.setProperty('--color-major-outage', colors.majorOutage);
  root.style.setProperty('--color-investigating', colors.investigating);
  root.style.setProperty('--color-identified', colors.identified);
  root.style.setProperty('--color-monitoring', colors.monitoring);
  root.style.setProperty('--color-resolved', colors.resolved);

  // Styles
  root.style.setProperty('--card-radius', styles.cardRadius);
  root.style.setProperty('--card-shadow', styles.shadow);

  // Apply body background
  document.body.style.backgroundColor = colors.background;
  document.body.style.color = colors.text;
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

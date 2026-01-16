import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { getBranding, BrandingSettings } from '../lib/api';

interface BrandingContextType {
  branding: BrandingSettings;
  loading: boolean;
}

const defaultBranding: BrandingSettings = {
  title_en: '',
  title_fr: '',
  logo: '',
};

const BrandingContext = createContext<BrandingContextType>({
  branding: defaultBranding,
  loading: true,
});

export function useBranding() {
  return useContext(BrandingContext);
}

interface BrandingProviderProps {
  children: ReactNode;
}

export function BrandingProvider({ children }: BrandingProviderProps) {
  const [branding, setBranding] = useState<BrandingSettings>(defaultBranding);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getBranding()
      .then((data) => {
        setBranding(data);
      })
      .catch(() => {
        // Keep default branding on error
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <BrandingContext.Provider value={{ branding, loading }}>
      {children}
    </BrandingContext.Provider>
  );
}

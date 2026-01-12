const createNextIntlPlugin = require('next-intl/plugin');

const withNextIntl = createNextIntlPlugin('./i18n.ts');

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Enable React strict mode for better development experience
  reactStrictMode: true,

  // Configure supported locales
  experimental: {
    // Enable experimental features if needed
  }
};

module.exports = withNextIntl(nextConfig);

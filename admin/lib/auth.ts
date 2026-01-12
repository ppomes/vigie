// Authentication utilities for admin interface
// Manages JWT token storage and retrieval using localStorage

const TOKEN_KEY = 'admin_jwt_token';
const TOKEN_EXPIRY_KEY = 'admin_jwt_expiry';
const USER_KEY = 'admin_user';

export interface UserInfo {
  id: number;
  username: string;
  email: string;
}

export interface AuthData {
  token: string;
  expiresAt: string;
  user: UserInfo;
}

/**
 * Store authentication data in localStorage
 * @param data - Authentication data including token, expiry, and user info
 */
export function setAuthData(data: AuthData): void {
  if (typeof window === 'undefined') return;

  localStorage.setItem(TOKEN_KEY, data.token);
  localStorage.setItem(TOKEN_EXPIRY_KEY, data.expiresAt);
  localStorage.setItem(USER_KEY, JSON.stringify(data.user));
}

/**
 * Get JWT token from localStorage
 * @returns JWT token or null if not found or expired
 */
export function getToken(): string | null {
  if (typeof window === 'undefined') return null;

  const token = localStorage.getItem(TOKEN_KEY);
  const expiry = localStorage.getItem(TOKEN_EXPIRY_KEY);

  if (!token || !expiry) {
    return null;
  }

  // Check if token is expired
  const expiryDate = new Date(expiry);
  if (expiryDate <= new Date()) {
    clearAuthData();
    return null;
  }

  return token;
}

/**
 * Get user information from localStorage
 * @returns User info or null if not found
 */
export function getUserInfo(): UserInfo | null {
  if (typeof window === 'undefined') return null;

  const userJson = localStorage.getItem(USER_KEY);
  if (!userJson) return null;

  try {
    return JSON.parse(userJson) as UserInfo;
  } catch {
    return null;
  }
}

/**
 * Check if user is authenticated
 * @returns true if valid token exists, false otherwise
 */
export function isAuthenticated(): boolean {
  return getToken() !== null;
}

/**
 * Clear all authentication data from localStorage
 */
export function clearAuthData(): void {
  if (typeof window === 'undefined') return;

  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(TOKEN_EXPIRY_KEY);
  localStorage.removeItem(USER_KEY);
}

/**
 * Logout - clears authentication data and optionally redirects to login
 * @param redirect - Whether to redirect to login page (default: true)
 */
export function logout(redirect = true): void {
  clearAuthData();

  if (redirect && typeof window !== 'undefined') {
    window.location.href = '/login';
  }
}

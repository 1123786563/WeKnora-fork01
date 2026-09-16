export async function signOutAndRedirect(logout: () => Promise<void>, replace: (path: '/(auth)/login') => void) {
  await logout();
  replace('/(auth)/login');
}

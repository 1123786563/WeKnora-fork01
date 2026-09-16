// The backend's native marker is weknora://oidc. Keep this route name in
// lockstep with the signed frontend_redirect_uri so Expo Router dispatches
// the server callback to the exchange screen after a cold start.
export { default } from '@/weknora/auth/AuthReturnScreen';

/**
 * Native OIDC return route. `openAuthSessionAsync` delivers the complete URL
 * to the Runtime browser port; this route intentionally contains no protocol
 * parsing or credential handling.
 */
export default function AuthReturn() {
  return null;
}

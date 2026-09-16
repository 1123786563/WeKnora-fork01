export function readInviteToken(search: string): string {
  return new URLSearchParams(search.startsWith('?') ? search.slice(1) : search).get('token')?.trim() ?? '';
}

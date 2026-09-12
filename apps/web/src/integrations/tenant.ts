export function parseIntegrationTenantId(value: string | null | undefined): number | null {
  if (value === null || value === undefined || value.trim() === '') return null;
  const id = Number(value);
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

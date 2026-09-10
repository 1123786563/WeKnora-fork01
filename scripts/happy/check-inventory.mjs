const REQUIRED = ['id', 'route', 'source', 'interaction', 'destination', 'service', 'task'];
const STATUSES = new Set(['pending', 'accepted', 'blocked']);

export function checkInventory(rows, routes) {
  if (!Array.isArray(rows) || !Array.isArray(routes)) throw new TypeError('rows and routes must be arrays');
  const ids = new Set();
  for (const row of rows) {
    if (ids.has(row.id)) throw new Error(`duplicate ${row.id}`);
    ids.add(row.id);
    for (const key of REQUIRED) {
      if (typeof row[key] !== 'string' || !row[key].trim()) throw new Error(`missing ${key}`);
    }
    if (!STATUSES.has(row.status)) throw new Error('invalid status');
    if (typeof row.status !== 'string') throw new Error('invalid status');
  }
  for (const route of routes) {
    if (!rows.some(row => row.route === route)) throw new Error(`unmapped ${route}`);
  }
}

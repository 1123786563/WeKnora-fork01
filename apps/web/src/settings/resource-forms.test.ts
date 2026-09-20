import assert from 'node:assert/strict';
import test from 'node:test';

// Structured-form domain for the three engine drawers (R482 B3-D1, R485 H2):
// payload assembly mirrors the Vue submissions —
//   VectorStoreSettings.vue onDrawerConfirm (L652-691)
//   StorageBackendSettings.vue save (L348-356)
//   WebSearchSettings.vue saveProvider (L569-620)
const {
  parseVectorStoreTypes,
  parseStorageProviders,
  parseWebSearchTypes,
  vectorStoreCreatePayload,
  vectorStoreUpdatePayload,
  vectorStoreCanTest,
  vectorStoreFieldLabel,
  storageBlankConfig,
  storageNeedsEndpoint,
  storageNeedsRegion,
  storageNeedsCredentials,
  storageBackendPayload,
  webSearchParamsOut,
  webSearchCreatePayload,
  webSearchUpdatePayload,
  webSearchCanTest,
} = await import('./resource-forms.ts');

test('parseVectorStoreTypes keeps engine type info with connection/index field schemas', () => {
  const rows = parseVectorStoreTypes([
    {
      type: 'elasticsearch',
      display_name: 'Elasticsearch',
      connection_fields: [
        { name: 'addr', type: 'string', required: true },
        { name: 'username', type: 'string', required: false },
        { name: 'password', type: 'string', required: false, sensitive: true },
        { name: 'insecure_skip_verify', type: 'boolean', required: false },
      ],
      index_fields: [
        { name: 'number_of_shards', type: 'number', required: false, min: 1, max: 64 },
        { name: 'knn_engine', type: 'string', required: false, enum: ['lucene', 'faiss'] },
      ],
    },
    'garbage',
    null,
  ]);
  assert.equal(rows.length, 1);
  const es = rows[0]!;
  assert.equal(es.type, 'elasticsearch');
  assert.equal(es.display_name, 'Elasticsearch');
  assert.equal(es.connection_fields.length, 4);
  assert.equal(es.connection_fields[2]!.sensitive, true);
  assert.equal(es.index_fields[0]!.max, 64);
  assert.deepEqual(es.index_fields[1]!.enum, ['lucene', 'faiss']);
});

test('parseVectorStoreTypes drops malformed field entries instead of crashing', () => {
  const rows = parseVectorStoreTypes([
    { type: 'qdrant', display_name: 'Qdrant', connection_fields: [{ name: 'addr', type: 'string', required: true }, { nope: 1 }, 'bad'], index_fields: [] },
  ]);
  assert.equal(rows[0]!.connection_fields.length, 1);
});

test('parseStorageProviders keeps provider id strings', () => {
  assert.deepEqual(parseStorageProviders(['local', 'minio', 's3', 42, null]), ['local', 'minio', 's3']);
});

test('parseWebSearchTypes keeps provider capability flags', () => {
  const rows = parseWebSearchTypes([
    {
      id: 'brave',
      name: 'Brave Search',
      requires_api_key: true,
      requires_engine_id: false,
      requires_base_url: false,
      supports_proxy: true,
      docs_url: 'https://brave.com/search/api/',
      config_fields: [{ key: 'country', label: 'Country', type: 'select', options: [{ label: 'China', value: 'cn' }] }],
    },
    { id: 'duckduckgo', name: 'DuckDuckGo', requires_api_key: false, requires_engine_id: false, requires_base_url: false, supports_proxy: false },
    'garbage',
  ]);
  assert.equal(rows.length, 2);
  const brave = rows[0]!;
  assert.equal(brave.requires_api_key, true);
  assert.equal(brave.supports_proxy, true);
  assert.equal(brave.docs_url, 'https://brave.com/search/api/');
  assert.equal(brave.config_fields!.length, 1);
  assert.deepEqual(brave.config_fields![0]!.options, [{ label: 'China', value: 'cn' }]);
  assert.equal(rows[1]!.requires_api_key, false);
});

test('vectorStoreCreatePayload mirrors the Vue drawer submission', () => {
  assert.deepEqual(vectorStoreCreatePayload({
    name: '  Primary ES  ',
    engineType: 'elasticsearch',
    connectionConfig: { addr: 'https://es.example.com:9200', username: 'elastic', password: 'secret' },
    indexConfig: { number_of_shards: 2, knn_engine: 'lucene' },
    includeIndex: true,
  }), {
    name: 'Primary ES',
    engine_type: 'elasticsearch',
    connection_config: { addr: 'https://es.example.com:9200', username: 'elastic', password: 'secret' },
    index_config: { number_of_shards: 2, knn_engine: 'lucene' },
  });
  // Advanced section collapsed → Vue submits an empty index_config.
  assert.deepEqual(vectorStoreCreatePayload({
    name: 'Primary ES',
    engineType: 'elasticsearch',
    connectionConfig: {},
    indexConfig: { number_of_shards: 2 },
    includeIndex: false,
  }).index_config, {});
});

test('vectorStoreCreatePayload requires name and engine type', () => {
  assert.throws(() => vectorStoreCreatePayload({ name: '  ', engineType: 'qdrant', connectionConfig: {}, indexConfig: {}, includeIndex: true }), /name/i);
  assert.throws(() => vectorStoreCreatePayload({ name: 'Store', engineType: '', connectionConfig: {}, indexConfig: {}, includeIndex: true }), /engine/i);
});

test('vectorStoreUpdatePayload only carries the editable name (engine is immutable)', () => {
  assert.deepEqual(vectorStoreUpdatePayload('  Renamed  '), { name: 'Renamed' });
  assert.throws(() => vectorStoreUpdatePayload('   '), /name/i);
});

test('vectorStoreCanTest requires every required connection field to be filled', () => {
  const type = { type: 'es', display_name: 'ES', connection_fields: [
    { name: 'addr', type: 'string' as const, required: true },
    { name: 'username', type: 'string' as const, required: false },
  ], index_fields: [] };
  assert.equal(vectorStoreCanTest(type, {}), false);
  assert.equal(vectorStoreCanTest(type, { addr: '  ' }), false);
  assert.equal(vectorStoreCanTest(type, { addr: 'https://es:9200' }), true);
  assert.equal(vectorStoreCanTest(null, { addr: 'x' }), false);
});

test('vectorStoreFieldLabel falls back to the raw field name like Vue fieldLabel', () => {
  const t = (key: string) => (key === 'vectorStoreSettings.fields.addr' ? '服务地址' : key);
  assert.equal(vectorStoreFieldLabel(t, 'addr'), '服务地址');
  assert.equal(vectorStoreFieldLabel(t, 'grpc_address'), 'grpc_address');
});

test('storageBlankConfig seeds the Vue blank form', () => {
  assert.deepEqual(storageBlankConfig(), {
    mode: 'remote', endpoint: '', region: '', access_key_id: '', secret_access_key: '',
    bucket_name: '', path_prefix: '', use_ssl: true,
  });
});

test('storage field visibility mirrors the Vue needs* computeds', () => {
  assert.equal(storageNeedsEndpoint('local', 'remote'), false);
  assert.equal(storageNeedsEndpoint('cos', 'remote'), false);
  assert.equal(storageNeedsEndpoint('minio', 'docker'), false);
  assert.equal(storageNeedsEndpoint('minio', 'remote'), true);
  assert.equal(storageNeedsEndpoint('s3', 'remote'), true);
  assert.equal(storageNeedsRegion('local'), false);
  assert.equal(storageNeedsRegion('minio'), false);
  assert.equal(storageNeedsRegion('s3'), true);
  assert.equal(storageNeedsCredentials('local', 'remote'), false);
  assert.equal(storageNeedsCredentials('minio', 'docker'), false);
  assert.equal(storageNeedsCredentials('minio', 'remote'), true);
});

test('storageBackendPayload mirrors the Vue save payload', () => {
  const config = { ...storageBlankConfig(), region: 'ap-guangzhou', bucket_name: 'weknora' };
  assert.deepEqual(storageBackendPayload('  Prod COS  ', 'cos', config), {
    name: 'Prod COS',
    provider: 'cos',
    config,
  });
  assert.throws(() => storageBackendPayload('', 'cos', storageBlankConfig()), /name/i);
  assert.throws(() => storageBackendPayload('Prod', '', storageBlankConfig()), /provider|type/i);
});

test('webSearchParamsOut keeps Vue field order, filters empty extra_config, gates api_key on create', () => {
  const out = webSearchParamsOut({
    apiKey: 'sk-1',
    engineId: 'bing',
    baseUrl: '',
    proxyUrl: 'http://127.0.0.1:7890',
    extraConfig: { country: 'cn', empty: '' },
  }, { includeApiKey: true });
  assert.deepEqual(out, {
    engine_id: 'bing',
    base_url: '',
    proxy_url: 'http://127.0.0.1:7890',
    extra_config: { country: 'cn' },
    api_key: 'sk-1',
  });
  // update path never carries api_key (credentials go through /credentials)
  const update = webSearchParamsOut({ apiKey: 'sk-1', engineId: '', baseUrl: '', proxyUrl: '', extraConfig: {} }, { includeApiKey: false });
  assert.deepEqual(update, { engine_id: '', base_url: '', proxy_url: '' });
  assert.equal('api_key' in update, false);
});

test('webSearchCreatePayload falls back to the provider display name when name is blank', () => {
  const payload = webSearchCreatePayload({
    name: '', provider: 'duckduckgo', providerDisplayName: 'DuckDuckGo', description: ' notes ',
    apiKey: 'k', engineId: '', baseUrl: '', proxyUrl: '', extraConfig: {}, isDefault: true,
  });
  assert.deepEqual(payload, {
    name: 'DuckDuckGo',
    provider: 'duckduckgo',
    description: ' notes ',
    parameters: { engine_id: '', base_url: '', proxy_url: '', api_key: 'k' },
    is_default: true,
  });
  assert.throws(() => webSearchCreatePayload({ name: '', provider: '', providerDisplayName: '', description: '', apiKey: '', engineId: '', baseUrl: '', proxyUrl: '', extraConfig: {}, isDefault: false }), /provider|type/i);
});

test('webSearchUpdatePayload drops the api_key even when one is typed', () => {
  const payload = webSearchUpdatePayload({
    name: 'Notes engine', provider: 'bing', providerDisplayName: 'Bing', description: '', apiKey: 'fresh-key',
    engineId: 'en-us', baseUrl: '', proxyUrl: '', extraConfig: {}, isDefault: false,
  });
  assert.deepEqual(payload, {
    name: 'Notes engine',
    provider: 'bing',
    description: '',
    parameters: { engine_id: 'en-us', base_url: '', proxy_url: '' },
    is_default: false,
  });
});

test('webSearchCanTest mirrors the Vue create-mode gating', () => {
  const type = {
    id: 'searxng', name: 'SearXNG', requires_api_key: false, requires_engine_id: false,
    requires_base_url: true, supports_proxy: true,
    config_fields: [{ key: 'lang', label: 'Language', type: 'select' as const, required: true, options: [{ label: 'auto', value: 'auto' }] }],
  };
  assert.equal(webSearchCanTest(type, { apiKey: '', engineId: '', baseUrl: '', extraConfig: {} }), false);
  assert.equal(webSearchCanTest(type, { apiKey: '', engineId: '', baseUrl: 'https://searxng.example.com', extraConfig: {} }), false);
  assert.equal(webSearchCanTest(type, { apiKey: '', engineId: '', baseUrl: 'https://searxng.example.com', extraConfig: { lang: 'auto' } }), true);
  const keyType = { ...type, requires_base_url: false, config_fields: [], requires_api_key: true };
  assert.equal(webSearchCanTest(keyType, { apiKey: '', engineId: '', baseUrl: '', extraConfig: {} }), false);
  assert.equal(webSearchCanTest(keyType, { apiKey: 'sk', engineId: '', baseUrl: '', extraConfig: {} }), true);
  assert.equal(webSearchCanTest(null, { apiKey: 'sk', engineId: '', baseUrl: '', extraConfig: {} }), false);
});

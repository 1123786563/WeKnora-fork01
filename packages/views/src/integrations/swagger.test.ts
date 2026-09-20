import assert from 'node:assert/strict';
import test from 'node:test';
import { shouldShowSwaggerDocs, swaggerDocsUrl } from './swagger.ts';

test('swaggerDocsUrl targets the gin-swagger index under any apiBaseUrl spelling', () => {
  assert.equal(swaggerDocsUrl('https://api.example.com'), 'https://api.example.com/swagger/index.html');
  assert.equal(swaggerDocsUrl('https://api.example.com/'), 'https://api.example.com/swagger/index.html');
  assert.equal(swaggerDocsUrl('https://api.example.com//'), 'https://api.example.com/swagger/index.html');
  assert.equal(swaggerDocsUrl('http://localhost:8000/api/v1'), 'http://localhost:8000/api/v1/swagger/index.html');
  // Same-origin deployments resolve resolveApiBaseUrl() to '' — the absolute
  // path still points at the backend-served SPA origin.
  assert.equal(swaggerDocsUrl(''), '/swagger/index.html');
});

test('shouldShowSwaggerDocs only trusts an explicit true flag', () => {
  // The row renders only when the system-info response confirmed the route:
  // undefined (older backend / fetch failure) and false (release build) must
  // both hide the entry rather than ship another dangling link.
  assert.equal(shouldShowSwaggerDocs(true), true);
  assert.equal(shouldShowSwaggerDocs(false), false);
  assert.equal(shouldShowSwaggerDocs(undefined), false);
});

import assert from 'node:assert/strict';
import test from 'node:test';
import { createWebTerminalController, webSocketTarget } from './terminal.ts';

class FakeSocket {
  binaryType = '';
  sent: unknown[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;

  send(value: unknown): void { this.sent.push(value); }
  close(): void { this.closed = true; this.onclose?.(); }
}

test('opens with a short-lived ticket, isolates websocket credentials, and maps PTY frames', async () => {
  const sockets: FakeSocket[] = [];
  const snapshots: Array<{ status: string; output: string; error?: string }> = [];
  const controller = createWebTerminalController({
    sessionId: 'session/1',
    wsOrigin: 'https://weknora.test',
    basePath: '/tenant/proxy',
    issueTicket: async () => ({ ticket: 'short-lived-ticket', expiresIn: 120 }),
    socketFactory: (url) => {
      assert.match(url, /^wss:\/\/weknora\.test\/tenant\/proxy\/api\/v1\/sessions\/session%2F1\/sandbox\/terminal\?/);
      assert.match(url, /ticket=short-lived-ticket/);
      assert.match(url, /provision=1/);
      assert.doesNotMatch(url, /Bearer|access-token|refresh/);
      const socket = new FakeSocket();
      sockets.push(socket);
      return socket;
    },
    onSnapshot: (snapshot) => snapshots.push(snapshot),
  });

  await controller.open({ provision: true });
  assert.equal(controller.snapshot().status, 'connecting');
  const socket = sockets[0]!;
  socket.onopen?.();
  socket.onmessage?.({ data: JSON.stringify({ type: 'ready', pty_id: 7, backend: 'local' }) });
  assert.equal(controller.snapshot().status, 'ready');

  controller.sendInput('ls');
  assert.deepEqual(Array.from(new Uint8Array(socket.sent[0] as Uint8Array)), [108, 115, 13, 10]);
  controller.resize(120, 40);
  assert.deepEqual(socket.sent[1], JSON.stringify({ type: 'resize', cols: 120, rows: 40 }));
  socket.onmessage?.({ data: new TextEncoder().encode('hello') });
  assert.equal(controller.snapshot().output, 'hello');
  socket.onmessage?.({ data: JSON.stringify({ type: 'error', code: 'SANDBOX_PAUSED', message: 'paused' }) });
  assert.equal(controller.snapshot().status, 'paused');
  assert.equal(controller.snapshot().error, 'paused');
  assert.ok(snapshots.length >= 4);
});

test('lookup opens without provisioning and keeps an unbound sandbox actionable', async () => {
  let openedUrl = '';
  const sockets: FakeSocket[] = [];
  const controller = createWebTerminalController({
    sessionId: 'session-1',
    wsOrigin: 'https://weknora.test',
    issueTicket: async () => ({ ticket: 'lookup-ticket', expiresIn: 120 }),
    socketFactory: (url) => {
      openedUrl = url;
      const socket = new FakeSocket();
      sockets.push(socket);
      return socket;
    },
  });

  await controller.open();

  assert.equal(new URL(openedUrl).searchParams.has('provision'), false);
  sockets[0]!.onmessage?.({
    data: JSON.stringify({ type: 'error', code: 'SANDBOX_NOT_BOUND', message: 'not started' }),
  });
  assert.equal(controller.snapshot().status, 'needs_provision');
});

test('ignores stale websocket events after close and never sends input to a closed socket', async () => {
  let socket: FakeSocket | undefined;
  const controller = createWebTerminalController({
    sessionId: 'session-1',
    wsOrigin: 'http://localhost:5173',
    issueTicket: async () => ({ ticket: 'ticket', expiresIn: 120 }),
    socketFactory: () => (socket = new FakeSocket()),
  });

  await controller.open();
  const stale = socket!;
  stale.onopen?.();
  controller.close();
  stale.onmessage?.({ data: new TextEncoder().encode('late') });
  controller.sendInput('ignored');
  assert.equal(controller.snapshot().status, 'idle');
  assert.equal(controller.snapshot().output, '');
  assert.deepEqual(stale.sent, []);
});

test('derives a websocket origin and preserves an API deployment sub-path', () => {
  assert.deepEqual(webSocketTarget('https://weknora.test/prefix/api/v1', 'http://localhost:5173'), {
    origin: 'https://weknora.test',
    basePath: '/prefix',
  });
  assert.deepEqual(webSocketTarget('', 'http://localhost:5173'), {
    origin: 'http://localhost:5173',
    basePath: '',
  });
});

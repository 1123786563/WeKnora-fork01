/**
 * Product assembly for the W25 session upload chain (I-1 fix): the single
 * production point where the upload manager, port, late-result remover and
 * native entry actions become the `attachments` prop the conversation route
 * hands to ConversationScreen.
 *
 * Transport policy: JSON traffic (late-result attachment cleanup) rides the
 * auth session's authenticated transport, so it shares the refresh
 * coordinator. The auth session does not wrap `sendMultipartFile` yet, so
 * multipart uploads ride the platform local file port with the live
 * credential's bearer attached — the same closure pattern as the execution
 * API fallback. A rotated credential re-runs this factory (route memo), so a
 * stale bearer fails the upload visibly and the retained record can retry.
 */
import { createJsonTransport, type BearerCredential, type ProductAuthSession } from '@weknora/api-client';
import { createSessionAttachments, defaultUploadLimits, type SessionAttachmentsPipeline, type UploadScope, type UploadTransport } from './upload';
import { nativeAttachmentSource } from './native-attachment-source';

export interface ProductSessionAttachmentsInput {
  origin: string;
  credential: BearerCredential;
  authSession: ProductAuthSession;
  scope: UploadScope;
  sessionID: string;
}

export function createProductSessionAttachments(input: ProductSessionAttachmentsInput): SessionAttachmentsPipeline {
  const localFilePort = createJsonTransport(fetch);
  const transport: UploadTransport = {
    send: (request) => input.authSession.transport.send(request),
    sendMultipartFile: (request) => localFilePort.sendMultipartFile!({
      ...request,
      headers: { ...request.headers, authorization: `Bearer ${input.credential.accessToken}` },
    }),
  };
  return createSessionAttachments({
    baseURL: input.origin,
    transport,
    scope: input.scope,
    limits: defaultUploadLimits,
    sessionID: input.sessionID,
    source: nativeAttachmentSource,
  });
}

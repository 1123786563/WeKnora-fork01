# T16 live viewer role matrix — 2026-09-11

## Scope

This is an isolated backend check of tenant membership, tenant switching, viewer reads, and viewer write denials. It does not certify the complete organization/system-admin matrix.

## Environment

- Backend: isolated Go server at `http://127.0.0.1:18080`
- Database: `/tmp/weknora-react-live-20260911.db`
- RBAC: `WEKNORA_TENANT_ENABLE_RBAC=true`, cross-tenant access disabled
- Test tenant: tenant `1`, created by the isolated owner account
- Test invitee: a newly registered viewer account created only in this temporary database

## Sequence and results

1. Owner login succeeded; inviting the viewer to tenant `1` returned HTTP `201` with invitation status `pending`.
2. Viewer login succeeded; `GET /api/v1/me/invitations` returned HTTP `200` with the pending viewer invitation.
3. `POST /api/v1/me/invitations/1/accept` returned HTTP `200` and an active `viewer` membership for tenant `1`.
4. `POST /api/v1/auth/switch-tenant` for tenant `1` returned HTTP `200` and a new tenant-scoped token.
5. Viewer `GET /api/v1/tenants/1/members` returned HTTP `200` with two members.
6. Viewer `PUT /api/v1/tenants/1` returned HTTP `403`.
7. Viewer `POST /api/v1/tenants/1/invitations` returned HTTP `403` with `Forbidden: insufficient workspace role`.
8. Viewer `PUT /api/v1/tenants/kv/memory-config` returned HTTP `403` with `Forbidden: insufficient workspace role`.

## Limitations

The check covers one real viewer and one tenant. It does not prove every role permutation, organization permission, system-admin path, Lite capability filter, browser rendering, native runtime, or 409 conflict. The temporary server was stopped after evidence collection; the exact temporary database remains on disk for recoverable inspection.

# V02 shared-isolated candidate: unavailable

Date: 2026-09-20

## Candidate inspected

The local candidate is Docker image `neo4j:2025.10.1`, image ID
`sha256:155c8aad10d5c838bc3bbc476c0418779086547822acb214ec5e3d49ba336907`.

The following read-only checks both reported the Community edition:

```sh
docker image inspect neo4j:2025.10.1 --format '{{range .Config.Env}}{{println .}}{{end}}'
docker exec semantica-v02-neo4j-1 sh -c 'cat /var/lib/neo4j/packaging_info'
```

Output includes `NEO4J_EDITION=community`,
`neo4j-community-2025.10.1-unix.tar.gz`, and `Edition: Community`.

## Isolation decision

The V02 shared-isolated candidate is **unavailable**. The relevant security
boundary for Neo4j is a database plus an account/role that has no access to a
foreign database. The candidate cannot supply that boundary: Neo4j's official
[database administration documentation](https://neo4j.com/docs/operations-manual/current/database-administration/)
states that Community Edition has exactly one standard database, while
Enterprise can have multiple. Its official [user management documentation](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-users/)
states that Community Edition has no roles and users have implied administrator
privileges.

Creating a second Community container, adding labels, or applying a property
prefix would therefore be another dedicated instance or only an application
filter, not a shared-isolated account/role boundary. No Enterprise image was
downloaded, no license agreement was accepted, and no shared result was
synthesized.

## V02 topology selection

The binding ADR allows a dedicated instance when shared infrastructure cannot
reliably isolate the required persistent state. Dedicated local Neo4j is the
selected **V02 capability-validation topology** because it has actual
`real-storage` evidence in [v02-dedicated-reverified.json](v02-dedicated-reverified.json).
This selection is not a production deployment, promotion, or an authorization
claim. A future production shared topology requires an explicitly procured and
authorized Enterprise/Aura boundary with multi-database RBAC evidence.

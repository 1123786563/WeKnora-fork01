# T14 conntrack source audit

Date: 2026-09-29 (Asia/Shanghai)
Scope: read-only audit of the T14 host/runtime conntrack evidence boundary. No
application, policy-helper, Docker configuration, or remote issue was changed.

## Verified facts

1. The current helper deliberately fails closed unless it can read one bounded,
   complete, exact bidirectional TCP `ESTABLISHED` record from
   `/proc/net/nf_conntrack` or `/proc/net/ip_conntrack`. The parser rejects
   missing tables, malformed/truncated/oversized input, wrong family/protocol,
   reverse-tuple mismatch, and duplicate matches. See the T14 helper source in
   the T14 worktree, `deploy/craft/render-boundary/policy-helper/helper.py:139-194`,
   and the fix plan's constraints at
   `docs/plans/2026-09-29-craft-107-t14-control-policy-fix1-plan.md:15-20,25-30,63-68`.

2. The existing live diagnostic exercised the real controller install path with
   a disposable `NetworkMode=none` renderer and recorded
   `kernel conntrack table is unavailable; refusing WebDriver exception`,
   followed by verified cleanup. It did not establish socket reuse or fresh
   same-listener denial. See
   `docs/testing/craft/t14/2026-09-28-established-flow-diagnostic/README.md:1-7`
   and `docs/plans/2026-09-28-craft-107-t14-exact-image-startup-isolation.md:5-17`.

3. Current renderer runtime inspection is consistent with that result. Docker reports
   OrbStack Linux `arm64`, kernel `7.0.14-orbstack-00380-ga7e0a2dc9535`,
   overlayfs. Four pre-existing `t14_*` containers use `NetworkMode=none` and
   `CapDrop=[ALL]`, `Privileged=false`, `no-new-privileges`; their processes
   have empty effective and bounding capability sets. In each renderer container,
   `/proc/net/nf_conntrack`, `/proc/net/ip_conntrack`, and the equivalent
   `/proc/self/net/*` paths do not exist. The helper image has no `conntrack`,
   `nft`, `ss`, or `ip` executable. These are observations only; the containers
   were not stopped, restarted, or modified.

4. The controller's ephemeral helper has a different, explicitly narrow
   privilege profile: `--network container:<renderer>`, `--cap-drop ALL`,
   `--cap-add NET_ADMIN`, `--security-opt no-new-privileges`, `--read-only`,
   `--user 0:0`, 128 MiB, 0.5 CPU, 32 PIDs, and a 16 MiB noexec/nosuid `/tmp`.
   See the T14 controller source at
   `deploy/craft/render-boundary/policy-helper/controller.py:428-440`.
   A disposable one-shot run using the same pinned helper image and the
   requested `--network none` profile verified `CapEff=0x1000` (NET_ADMIN),
   `CapDrop=[ALL]`, uid 0, and no conntrack proc files. It then sent a real
   `IPCTNL_MSG_CT_GET` netlink dump request and received a kernel `NLMSG_DONE`
   response (`type=3`, flags `NLM_F_MULTI`, 20 bytes, no `NLMSG_ERROR`). This
   proves that ctnetlink dump authorization is available to the helper's
   NET_ADMIN capability even though the renderer itself has no capabilities.
   The empty `--network none` namespace had no flow, so this run did not prove
   that a tuple is emitted or that tuple attributes can be parsed. The helper
   command was one-shot, auto-removed, and used no mount or published port.

## Authority analysis

The ctnetlink family is the kernel's authoritative conntrack API and is the
only plausible alternative source to procfs here. The helper capability probe
shows that a narrow implementation could
send a bounded `IPCTNL_MSG_CT_GET` dump from a process in the renderer's network
namespace, parse the kernel attributes (`CTA_TUPLE_ORIG`, `CTA_TUPLE_REPLY`,
`CTA_PROTO_NUM`, and `CTA_STATUS`/TCP state as applicable), and apply the same
exact-one-match rule. The relevant primary references are the Linux kernel
conntrack netlink UAPI (`include/uapi/linux/netfilter/nfnetlink_conntrack.h`),
the kernel conntrack sysctl documentation
<https://www.kernel.org/doc/html/latest/networking/nf_conntrack-sysctl.html>,
and libnetfilter_conntrack's official API documentation
<https://www.netfilter.org/projects/libnetfilter_conntrack/>.

The alternative is therefore viable in the existing helper privilege boundary,
subject to implementation and live tuple proof. The helper must query through
its existing `--network container:<renderer>` namespace, never from the host or
the renderer process. A host-side ctnetlink query would be the wrong namespace
and could observe unrelated flows. `docker exec --user 0` against the renderer
cannot recover capabilities dropped at renderer creation; the controller's
separate helper is the correct authority boundary.

An exact Docker inspect or `/proc/<renderer-pid>/net/...` view is not an
independent alternative: inspect exposes container identity and network
configuration, not live conntrack tuples; proc net files are namespace-scoped,
and the runtime currently exposes neither conntrack proc table. Socket state
(`ss`, `/proc/net/tcp`) can prove an endpoint is open, but cannot prove the
netfilter conntrack record used by `ct state established`, so it has an authority
gap for this policy.

## Recommendation

Keep the current fail-closed behavior until a ctnetlink reader is implemented
and its live tuple evidence passes. Do not replace the policy proof with socket
state, Docker metadata, application logs, nft counters, or a host-global
ctnetlink dump. The procfs source is blocked in this runtime, but the helper's
NET_ADMIN ctnetlink seam is a viable narrow replacement candidate.

## Narrow follow-up if a suitable runtime is supplied

This should be a separate implementation task with explicit security approval:

1. Provide a disposable Linux Docker/VM fixture where conntrack is enabled and
   the barrier process can issue a read-only ctnetlink dump in the renderer
   namespace. The least privilege must be demonstrated (capability and namespace
   receipt); do not use `--privileged`.
2. Add a tiny pinned ctnetlink reader (or a pinned helper binary/package) with
   strict message, attribute, byte, record, and wall-time bounds. Reject
   `NLMSG_ERROR`, truncation, sequence/namespace mismatch, malformed attributes,
   non-TCP/wrong-family records, duplicate exact matches, and missing terminal
   `NLMSG_DONE`.
3. Reuse the existing seven-field flow contract and parser tests, adding raw
   netlink fixtures for one valid record and each failure mode. Keep procfs as an
   optional source only when its namespace and completeness are independently
   established; never fall back to a less authoritative source.
4. Run one disposable namespace test that proves: preexisting WebDriver socket
   request/response survives policy, a new connection to the same listener from
   another source port is denied with the exact drop delta, and cleanup removes
   every helper/container. Only then run the immutable T14 matrix.

Dependencies/resource needs: Linux kernel with `CONFIG_NF_CONNTRACK` and
`CONFIG_NETFILTER_NETLINK`, the existing ephemeral helper's `CAP_NET_ADMIN`
profile, a disposable Docker/VM network namespace, a capability-limited
ctnetlink reader, and a test image containing that reader. The current
OrbStack/macOS runtime satisfies the capability and request-authorization seam;
it still lacks completed live T14 tuple/reuse/denial evidence.

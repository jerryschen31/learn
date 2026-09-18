# 07 — Part G: bounded next action

## 1. Recommended first 90 minutes

| # | Read | Question to answer while reading |
|---|---|---|
| 1 | `agent/agent.go:154-260` (`delegate` interface, `Agent` struct) | Which capabilities does the agent require from *either* role, and which fields are role-specific? |
| 2 | `agent/agent.go:Start` (~605–1000) | In what order are local state, syncer, delegate, persisted services, proxycfg, and listeners created, and why does the order matter? |
| 3 | `agent/local/state.go:1434-1520` (`syncService`) | What exactly is in the `Catalog.Register` request, and which error makes the agent stop retrying? |
| 4 | `agent/consul/catalog_endpoint.go:109-205` (`Register`) | What must be true (forwarding, ACL, validation) before `raftApply` is called? |
| 5 | `agent/consul/state/catalog.go:138-330` (`EnsureRegistration`, `ensureRegistrationTxn`) | Which rows and indexes change for a registration that repeats identical content? |
| 6 | `agent/blockingquery/blockingquery.go:117-215` (`Query`) | Under what three conditions does a blocking query return, and what index does it hand back in each? |

Then run: `go test ./agent/local -run 'TestAgentAntiEntropy_Services$' -v` and match the log lines to files 3–5.

## 2. Five invariants to keep in mind

1. **The owning agent is the source of truth for its services; the catalog is a converged projection.** (`local.State` + `ae.StateSyncer`; full sync repairs drift.)
2. **Every catalog mutation is a Raft entry applied by `fsm.Apply` on the leader before the RPC returns; followers apply later.** (`raftApplyEncoded`, `future.Error()`.)
3. **Reads carry an index; a blocking read returns only when the touched tables' max index exceeds `MinQueryIndex` or a jittered timeout elapses; same index means no change.**
4. **Membership (Serf) and health (checks) are different truths; they meet only through the leader writing `serfHealth`.** (`localMemberEvent` → `reconcileCh` → `HandleFailedMember`.)
5. **Consistency is the caller's choice and the server reports what it did** (`AllowStale`/`RequireConsistent`; `X-Consul-Index`, `X-Consul-LastContact`, `X-Consul-KnownLeader`); default reads are leader-served but unverified.

## 3. Three things not to learn yet

1. **Envoy/xDS resource generation (`agent/xds/*` beyond `delta.go`)** — thousands of lines of Envoy-specific mapping; understanding it requires Envoy's model, and it does not change the discovery invariants above.
2. **Multi-DC federation, WAN mesh gateways, peering (`wanfed`, `federation_state_*`, `agent/rpc/peering`, `peerstream`)** — layered on top of per-DC catalogs; learning it early blurs the "catalog is per-DC, Raft is per-DC" model.
3. **ACL implementation internals and the resource-API remnants (`acl/policy*.go`, auth methods, `internal/resource`, `internal/controller`, `pbresource`)** — you need only the enforcement *boundary* now; the policy engine and the resource remnants are large, partly compat, and do not affect the lifecycle trace.

(Also parked: UI, enterprise-split files, performance tuning, `ServiceAI` metadata.)

## 4. My first checkpoint (answer in your own words before we continue)

Explain why Consul needs **both** Raft-backed catalog state **and** gossip-based membership. Cover: what each is authoritative for, what would go wrong if the catalog were maintained by gossip alone, what would go wrong if membership were maintained through Raft alone, and the single code path where the two meet. I will not give you the answer until you reply.

## 5. Checkout-specific uncertainties (could not be established from this repository alone)

- **Release/version alignment**: `version/VERSION` is `2.1.0-dev` while `CHANGELOG.md` tops out at `2.0.4`; which behaviours here are already released is unknowable from the tree.
- **Enterprise behaviour**: 141 `_ce.go` files hide enterprise halves (partitions, namespaces, segments, sentinel, audit, FIPS). Any code path taking `*acl.EnterpriseMeta` may behave differently in enterprise builds.
- **Deployment topology defaults**: single-DC vs federated, agentful vs agentless (`consul-dataplane`), TLS/ACL on or off, `use_streaming_backend` (default `true` in `builder.go`) and `rpc.enable_streaming` interplay — all change which code path a request takes.
- **Library-level timings**: memberlist probe/suspicion intervals, Raft election timeouts, and `future.Error()` semantics live in `hashicorp/memberlist`, `hashicorp/serf`, `hashicorp/raft` (vendored versions listed in `00-checkout-facts.md`), not in this repo.
- **Raft log store**: fresh data dirs use raft-wal with verification enabled by default; existing `raft.db` keeps BoltDB (`server.go:setupRaft`); operator overrides via `raft_logstore` config.
- **Architecture docs**: none in this checkout; the older in-repo `docs/` architecture material is absent, so any narrative beyond code is inference.
- **CLA / architecture review process**: not documented in the repo; only `CODEOWNERS` and CONTRIBUTING's "discuss in the issue".
- **`ServiceAI` (`agent/structs/service_ai.go`)**: references an external "CAMP design summary"; scope, stability, and which release it ships in are not determinable here.
- **SoTW xDS**: `xds.Server.StreamAggregatedResources` exists alongside the delta protocol; which is used by which Envoy version/bootstrap was not verified.

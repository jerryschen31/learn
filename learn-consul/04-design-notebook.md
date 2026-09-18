# 04 — Part D: design notebook (12 topics, in reading order)

Format per topic: problem → invariant → mechanism & boundary → code & tests → failure scenario → wrong mental model → check question → cautious analogy.

---

## D1. Agent modes: server versus client, and why both exist

- **Problem:** consensus scales to ~5–7 voters, not thousands of machines; but every machine needs a local API, local health checking, and a way to find servers.
- **Invariant:** exactly one process per node owns that node's registrations; only servers hold authoritative state; clients must never block on their own persistence for reads.
- **Mechanism:** one `agent.Agent` type with a `delegate` (`agent/agent.go:154`) that is either `*consul.Server` or `*consul.Client`. Both join LAN Serf; only servers run Raft and RPC endpoints. Clients route RPCs through `agent/router`.
- **Code:** `agent/agent.go:Start` (`ServerMode` branch), `agent/consul/server.go:NewServer` (wires `fsm.NewFromDeps`, `setupRPC`, `setupRaft`, `setupSerf`, `go lanEventHandler`, `go monitorLeadership`, `go listen`, `autopilot.Start`), `agent/consul/client.go:NewClient`, `Client.RPC`. Tests: `agent/consul/server_test.go:TestServer_StartStop`, `TestServer_Expect`; `agent/consul/client_test.go`.
- **Failure scenario:** a server agent is also an agent — if you register services on a server node and then decommission it, anti-entropy on *that* server keeps re-asserting them until it leaves gracefully (`Server.Leave` removes it from Raft and Serf, `server.go:1353`).
- **Wrong model:** "clients are thin stubs." They persist registrations, run checks, cache, resolve ACLs with a down policy, and serve xDS.
- **Check question:** which goroutine on a *client* decides that a server is unhealthy, and what does it do about it? (`client_serf.go:nodeFail` and `router.Manager.NotifyFailedServer`.)
- **Analogy:** Temporal's frontend/history/matching separation is a *service* split; Consul's is a *role* split of the same binary. Your agent harness will likely need the "local sidecar owns the worker's liveness" idea; it will not need Raft on every worker.

## D2. Service registration, catalog entries, and health checks

- **Problem:** a worker must be able to announce itself and its capability without a network round trip to a quorum, and the announcement must outlive the announcer's process.
- **Invariant:** the catalog row for (node, service ID) equals what the owning agent believes, modulo propagation delay; checks attach to a service or a node, never float.
- **Mechanism:** `local.State` is the source of truth per agent; `ae.StateSyncer` converges the catalog to it. Capability = `Tags`/`Meta` on `structs.NodeService` (queryable via `?tag=` and `?filter=`; `Meta` is not indexed).
- **Code:** `agent/agent.go:AddService → addServiceInternal → persistService`, `agent/local/state.go:AddServiceWithChecks`, `syncService`, `structs.RegisterRequest` (`agent/structs/structs.go:489`), `state/catalog.go:ensureRegistrationTxn`, `ensureServiceTxn` (`IsSame` short-circuit). Tests: `agent/local/state_test.go:TestAgentAntiEntropy_Services_WithChecks`, `agent/consul/state/catalog_test.go:TestStateStore_EnsureRegistration`.
- **Failure scenario:** two agents on different nodes register the same service ID — fine, IDs are per node. Two *processes on one node* register the same ID → last one wins and the first's check runner is replaced (`addServiceInternal` removes existing).
- **Wrong model:** "the catalog is the registry and the agent is a client of it." The agent is the *owner*; the catalog is a replicated projection.
- **Check question:** if you `PUT /v1/catalog/deregister` an agent-owned service, what happens within `ae_interval`? (Full sync re-registers it.)
- **Analogy:** Temporal task-queue pollers "register" by polling; there is no durable registration. Consul's registration is closer to a Kubernetes Endpoints object maintained by a kubelet-like agent.

## D3. Gossip membership and failure-detection uncertainty

- **Problem:** with thousands of agents, servers cannot heartbeat every one; and every agent needs the server list without a static config.
- **Invariant:** eventually every live member learns of joins/leaves/failures; a member is only declared failed after direct and indirect probes fail **[library: memberlist]**.
- **Mechanism:** Serf LAN pool per DC (clients + servers), Serf WAN pool (servers only). Tags carry `role=consul`, `dc`, `port`, `grpc_port`, `raft_vsn`, `bootstrap`/`expect`. Servers use events to maintain `serverLookup`/`router`; the leader uses them to reconcile the catalog.
- **Code:** `agent/consul/server_serf.go:setupSerfConfig` (tags), `lanEventHandler`, `localMemberEvent` (sends to `reconcileCh` only if leader), `lanNodeFailed`; `agent/consul/client_serf.go`; `internal/gossip/libserf/serf.go:DefaultConfig`; `agent/metadata/server.go:IsConsulServer`. Tests: `agent/consul/server_test.go:TestServer_JoinLAN`, `TestServer_LANReap`, `agent/router/manager_test.go`.
- **Failure scenario:** a GC pause or saturated NIC makes an agent miss probes → declared failed → leader marks `serfHealth` critical → its services vanish from `?passing` answers → it rejoins seconds later → flaps back. Nothing was wrong with the services.
- **Wrong model:** "`consul members` shows service health" or "gossip carries the catalog." It carries member metadata only.
- **Check question:** why does `localMemberEvent` check `IsLeader()` before sending on `reconcileCh`? (Only the leader may write via Raft; followers would just fail.)
- **Analogy:** memberlist ≈ the heartbeat/liveness layer you would build for a worker fleet; it tells you "the worker host is reachable", never "the tool call will succeed".

## D4. Raft: leader, quorum, log, FSM application, and authoritative state

- **Problem:** the catalog must survive server loss and never fork.
- **Invariant:** a write is acknowledged only after a majority has it in the log; all servers apply the same entries in the same order to a deterministic FSM.
- **Mechanism:** `hashicorp/raft` with `fsm.FSM` as the state machine; log store raft-wal (default for fresh data dirs; BoltDB retained when `raft.db` exists), file snapshot store, network transport multiplexed on the RPC port (`rpc.go:handleRaftRPC`). Autopilot manages server health and dead-server cleanup.
- **Code:** `agent/consul/server.go:setupRaft`, `agent/consul/fsm/fsm.go:Apply/Snapshot/Restore`, `commands_ce.go:init`, `agent/consul/rpc.go:raftApplyEncoded`, `agent/consul/raft_handle.go`, `agent/consul/autopilot.go`. Tests: `fsm/commands_ce_test.go`, `fsm/snapshot_test.go`, `server_test.go:TestServer_LeaveLeader`, `TestServer_AvoidReBootstrap`, `leader_test.go:TestLeader_RollRaftServer`.
- **Failure scenario:** 3 servers, 2 lost → no quorum → all writes and default reads fail; `?stale` reads and DNS keep working; agents keep local state; when 1 server returns, election resumes. A "peers.json" recovery is operator work, not code you read now.
- **Wrong model:** "the FSM is the database." The FSM *applies* to the `state.Store`; reads bypass Raft and go to the store directly (that is why `?consistent` exists).
- **Check question:** what does `future.Error()` returning nil in `raftApplyEncoded` guarantee about followers' FSMs? (Nothing yet — only commit on a quorum and apply on the leader.)
- **Analogy:** Temporal's persistence (Cassandra/SQL) provides durability with conditional updates per shard; Consul's Raft gives one global ordered log per DC. Your agent platform will likely lean on an existing durable store rather than embed Raft.

## D5. RPC forwarding, leader routing, and ownership

- **Problem:** any agent may contact any server; only the leader may write; DCs are separate.
- **Invariant:** writes and non-stale reads execute on the leader of the target DC; retries never cross the caller's `RPCHoldTimeout`; tokens local to a DC are not forwarded elsewhere.
- **Mechanism:** every endpoint starts with `ForwardRPC`. Client-side `router.Manager` rotates servers and reacts to failures.
- **Code:** `agent/consul/rpc.go:ForwardRPC`, `forwardRPC`, `forwardRequestToOtherDatacenter`, `forwardRequestToLeader`, `getLeader`, `canRetry`, `getWaitTime`, `forwardDC`; `agent/consul/client.go:RPC`; `agent/router/manager.go`. Tests: `rpc_test.go:TestRPC_NoLeader_Retry`, `TestCanRetry`, `TestRPC_LocalTokenStrippedOnForward`, `catalog_endpoint_test.go:TestCatalog_Register_ForwardLeader/_ForwardDC`.
- **Failure scenario:** leader steps down while forwarding; follower gets `ErrNotLeader`-class error, `canRetry` says yes for reads and for `ErrNoLeader`; write retries are safe only because the request is idempotent (registration) — non-idempotent writes (KV CAS, sessions) rely on their own semantics.
- **Wrong model:** "the client agent talks to the leader." It talks to *some* server; forwarding is server-side.
- **Check question:** under which three conditions may a follower answer a read locally? (`canServeReadRequest`: read, `AllowStale`, and `raft.LastContact` non-zero.)
- **Analogy:** Temporal's frontend routes to history shards by workflow ID; Consul routes by DC then to the single leader. Ownership is per-DC-leader, not per-key.

## D6. Service-discovery API paths: HTTP, DNS, gRPC

- **Problem:** consumers have different integration abilities (curl, resolver, gRPC client).
- **Invariant:** all paths read the same catalog through the same RPC (`Health.ServiceNodes`/`Catalog.*`) and expose the same health filtering, differing only in defaults.
- **Mechanism:** HTTP handlers parse query params into `structs.QueryOptions`; DNS builds `ServiceSpecificRequest` from `dns_config`; gRPC `pbdns` wraps DNS; all call `rpcClientHealth.ServiceNodes`.
- **Code:** `agent/health_endpoint.go:healthServiceNodes`, `agent/http.go:parse*`/`setMeta`, `agent/dns.go:dispatch → handleServiceQuery → lookupServiceNodes` (defaults `AllowStale`, `HealthFilterExcludeCritical`, `OnlyPassing` if configured), `agent/grpc-external/services/dns/server.go:Query`. Tests: `agent/health_endpoint_test.go`, `agent/dns_service_lookup_test.go:TestDNS_ServiceLookup_FilterCritical/_OnlyPassing/_Randomize/_Truncate`.
- **Failure scenario:** UDP DNS truncation drops instances silently (`trimUDPResponse`); a consumer that only uses DNS A-records sees a *sample*, not the set.
- **Wrong model:** "DNS is consistent with HTTP." DNS is stale-by-default and may be cached; HTTP default reads are leader-served.
- **Check question:** which HTTP header tells you the query was served by the streaming backend, and what request shape triggers it?
- **Analogy:** none needed; but note your coordinator should prefer the HTTP/gRPC path with `?index` (or a streaming client) over DNS for capability routing.

## D7. Blocking queries, indexes, and change notification

- **Problem:** polling a catalog of thousands of services burns CPU and still lags.
- **Invariant:** a blocking read returns only when the max index of the tables it touched exceeds the caller's `MinQueryIndex` or a bounded, jittered timeout elapses; the returned index is the one the caller must send next.
- **Mechanism:** memdb watch channels collected into a `WatchSet` inside the query closure; `blockingquery.Query` loops; `ErrNotFound`/`ErrNotChanged` sentinels; jitter to avoid thundering herds; `store.AbandonCh()` to unblock on snapshot restore.
- **Code:** `agent/blockingquery/blockingquery.go:Query`, `agent/consul/rpc.go:blockingQuery`, `RPCQueryTimeout`, `structs.QueryOptions.BlockingTimeout`, `state/state_store.go:maxIndexWatchTxn`. Tests: `blockingquery_test.go`, `rpc_test.go:TestServer_blockingQuery`, `health_endpoint_test.go:TestHealthServiceNodes_Blocking`.
- **Failure scenario:** a table-level watch fires for an unrelated service; the caller re-fetches and sees identical data with a higher index — callers that compare "index changed" instead of "data changed" do redundant work.
- **Wrong model:** "the index is a version of *my* result." It is a Raft index for the *tables* touched (with row-level watches where the schema supports them).
- **Check question:** why must a query function never return `Index: 0` on an empty result, and how do the sentinels help?
- **Analogy:** Temporal long-poll for tasks (matching) is also a bounded long-poll, but it dequeues work; Consul's blocking query is a *watch*, it dequeues nothing.

## D8. Strong versus stale/consistent reads

- **Problem:** leader-only reads bottleneck; follower reads can be stale; some callers need linearizability.
- **Invariant:** the caller chooses; the server tells the truth about what it did (`X-Consul-Effective-Consistency`, `LastContact`, `KnownLeader`).
- **Mechanism:** `QueryOptions.AllowStale`/`RequireConsistent`; `ConsistentRead` = `raft.VerifyLeader` + `readyForConsistentReads` (set after the leader's barrier); `canServeReadRequest` for followers; agent-side `Cache-Control` adds a third, caller-bounded layer.
- **Code:** `structs.QueryOptions.ConsistencyLevel`, `rpc.go:consistentReadWithContext`, `server.go:setConsistentReadReady/isReadyForConsistentReads`, `http.go:parseConsistency/parseCacheControl`, `agent/cache/cache.go:Get`. Tests: `rpc_test.go:TestRPC_ReadyForConsistentReads`, `catalog_endpoint_test.go:TestCatalog_ListNodes_StaleRead/_ConsistentRead_Fail`, `agent/cache/cache_test.go`.
- **Failure scenario:** immediately after election, `?consistent` reads fail with `ErrNotReadyForConsistentReads` while default reads succeed — a health dashboard using `?consistent` shows errors during a routine leader change.
- **Wrong model:** "default reads are consistent." They are leader-served but unverified.
- **Check question:** explain why `LastContact` is only meaningful for stale reads and is 0 otherwise.
- **Analogy:** Temporal's `DescribeWorkflowExecution` is served from mutable state (strong within a shard); Consul offers a dial instead. For a coordinator, stale reads with an index watch are usually the right choice; use `?consistent` only for fencing-like decisions.

## D9. ACLs, tokens, identities, and the authorization boundary

- **Problem:** anyone reaching an agent could register/deregister anything or read everything.
- **Invariant:** every RPC that mutates the catalog checks `service:write`/`node:write` on the leader; every read filters results the token cannot read (`ResultsFilteredByACLs=true`); agents vet before accepting local registrations.
- **Mechanism:** token → `ACLResolver.ResolveToken` → `acl.Authorizer` (policy/role/chained authorizers); servers resolve locally, clients via RPC with caching and a down policy; `aclfilter.Filter` post-filters results.
- **Code:** `acl/authorizer.go:Authorizer`, `acl/policy_authorizer.go`, `agent/consul/acl.go:ACLResolver`, `filterACL`, `agent/structs/aclfilter/filter.go`, `agent/consul/catalog_endpoint.go:vetRegisterWithACL`, `agent/acl.go:vetServiceRegisterWithAuthorizer`, `agent/token/store.go`, `local/state.go:aclTokenForServiceSync`. Tests: `acl/acl_test.go:TestACL`, `agent/consul/acl_test.go:TestACLResolver_DownPolicy`, `health_endpoint_test.go:TestHealth_ServiceNodes_FilterACL`, `agent_endpoint_test.go:TestAgent_RegisterService_ACLDeny`.
- **Failure scenario:** the agent's default token loses `service:write` for one service; `syncService` gets permission denied, marks `InSync=true`, logs once, and the service silently stops being updated in the catalog.
- **Wrong model:** "ACL enforcement happens in the HTTP layer." The leader RPC is the boundary; HTTP vetting is a courtesy check.
- **Check question:** where does a *client* agent get the policy for a token it has never seen, and what happens if servers are unreachable?
- **Analogy:** Consul ACLs guard the *catalog*, not your tool calls. Application-level authorization (which agent may invoke which tool) is not Consul's job; intentions (D10) only govern mesh connections.

## D10. Service mesh / Connect: control plane vs data plane

- **Problem:** encrypting and authorizing service-to-service traffic without touching application code.
- **Invariant:** the control plane computes desired proxy state from the catalog and config entries; the data plane (Envoy) enforces; the control plane can only observe ACK/NACK.
- **Mechanism:** `proxycfg.Manager` keeps a `state` per registered proxy that subscribes (via `DataSources`, e.g. `Health`) to catalog/config data, coalesces updates into a `ConfigSnapshot`; `xds.Server.DeltaAggregatedResources` streams resources generated by `ResourceGenerator`; `CAManager` signs leaf certs; intentions become Envoy RBAC (`agent/xds/rbac.go`).
- **Code:** `agent/proxycfg/manager.go`, `state.go:run`, `data_sources.go`, `agent/proxycfg-glue/health.go`, `agent/proxycfg-sources/local/sync.go`, `agent/proxycfg-sources/catalog/config_source.go` (agentless), `agent/xds/delta.go:processDelta`, `authenticate/authorize`, `resources.go`, `agent/consul/leader_connect_ca.go`, `command/connect/envoy/envoy.go`. Tests: `agent/xds/delta_test.go`, `proxycfg/manager_test.go:TestManager_BasicLifecycle`, `state_test.go:TestState_WatchesAndUpdates`, `test/integration/connect/envoy`.
- **Failure scenario:** the agent dies; Envoy keeps its last config and continues proxying (fail-static), but stops learning about endpoint changes and cannot rotate certs.
- **Wrong model:** "Consul routes the traffic." It never sees a byte of it.
- **Check question:** why does `processDelta` re-run `authorize` on every new snapshot rather than once at stream start?
- **Analogy:** similar to how a Temporal worker's config (task queue, concurrency) is decided by the worker, not the server; here the *proxy's* config is decided centrally and pushed. For your harness, the useful pattern is "compute a snapshot, coalesce, push deltas, wait for ACK".

## D11. Observability and debugging

- **Problem:** distributed state disagreements are invisible without per-layer signals.
- **Invariant:** each layer exposes its own view: local agent (`/v1/agent/services|checks`, `consul members`), catalog (`/v1/catalog`, `/v1/health`), Raft (`consul operator raft list-peers`, `consul info`), streaming/cache (`X-Consul-Query-Backend`, `X-Cache`), proxy (`consul connect envoy` admin, xDS metrics).
- **Mechanism:** `go-metrics` sinks with Prometheus definitions; hclog named loggers (`logging.Catalog`, `logging.Leader`…); `consul debug` bundles; `consul info` (`command/info`).
- **Code:** `lib/telemetry.go:InitTelemetry`, `agent/setup.go:getPrometheusDefs`, `agent/consul/rpc.go:RPCCounters/RPCGauges/RPCSummaries`, metric names in code paths (`catalog.register`, `fsm.register`, `leader.barrier`, `server.isLeader`), `logging/logger.go:Setup`, `command/debug/debug.go`, `agent/debug/host.go`. Tests: `server_test.go:TestServer_RPC_MetricsIntercept`, `command/debug/debug_test.go`.
- **Failure scenario:** an operator sees a service "healthy in `/v1/agent/checks` on w1 but absent from `/v1/health/service`" — the diagnosis order is: agent sync logs (`agent: Synced service`), then catalog index for the node, then `serfHealth`, then ACL token.
- **Wrong model:** "one dashboard shows the truth." Each layer is truthful only about itself.
- **Check question:** given `X-Consul-KnownLeader: false` and a normal-looking body, what happened?
- **Analogy:** like Temporal's separation of visibility store vs mutable state — query the layer that owns the fact.

## D12. Multi-datacenter / WAN / federation (overview only)

- **Problem:** independent DCs need to discover each other's services without one global Raft.
- **Invariant:** each DC has its own Raft catalog; cross-DC reads are forwarded RPCs, not replicated data; the primary DC is authoritative for ACLs and the CA.
- **Mechanism:** WAN Serf pool among servers (or WAN federation through mesh gateways, `wanfed`), `forwardDC` by `router.FindRoute(dc)`, ACL/config-entry/federation-state replication routines on the leader, and cluster peering as the newer alternative.
- **Code:** `agent/consul/server.go` (serfWAN, `Flood`), `rpc.go:forwardRequestToOtherDatacenter/forwardDC`, `agent/consul/wanfed/`, `federation_state_replication.go`, `acl_replication.go`, `agent/rpc/peering`. Tests: `server_test.go:TestServer_JoinWAN*`, `catalog_endpoint_test.go:TestCatalog_Register_ForwardDC`, `test-integ/peering_commontopo`.
- **Failure scenario:** WAN link down → `ErrNoDCPath` for cross-DC queries; local DC unaffected.
- **Wrong model:** "the catalog is global." It is per-DC.
- **Check question:** which single field in `structs.RegisterRequest` decides whether a write leaves the local DC?
- **Analogy:** Temporal namespaces with multi-cluster replication replicate history; Consul DCs do not replicate the catalog at all. Defer.

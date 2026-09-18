# 05 — Part E: 12-session source-reading apprenticeship

Rules: each session is 60–120 minutes; stop at the stopping criteria; write the "explain it back" paragraph before moving on; leave the Socratic questions for our next interactive session. Instrumentation exercises mean temporary `logger.Info`/`t.Log` lines that you **do not commit** (`git stash` or `git checkout -- <file>` afterwards).

Global prerequisites: `make dev` succeeds; `go test -short ./agent/local` passes on your machine; you have read `00-checkout-facts.md` and `01-first-principles.md`.

---

## Session 1 — Repository map and startup topology
**Question:** what runs when `consul agent -dev` starts, and where does "server vs client" get decided?
**Prereqs:** none.
**Read in order:**
1. `main.go`
2. `command/agent/agent.go` (`(*cmd).run`, from `NewBaseDeps` to `StartSync`)
3. `agent/setup.go` (`BaseDeps`, `NewBaseDeps`)
4. `agent/agent.go:154-260` (`delegate` interface, `Agent` struct fields)
5. `agent/agent.go:Start` (lines ~605–1000), skim `listenAndServeGRPC`, `listenAndServeDNS`, `listenHTTP`
6. `agent/consul/server.go:NewServer` (skim the ordering of `fsm.NewFromDeps`, `setupRPC`, `setupRaft`, `setupSerf`, goroutines)
7. `agent/consul/client.go:NewClient`
**Follow:** `Agent.delegate` assignment; `consulCfg.ServerUp = a.sync.SyncFull.Trigger`; the goroutines started in `NewServer`.
**Trace manually:** `consul agent -dev` → which of `NewServer`/`NewClient` runs, and which listeners bind (HTTP 8500, DNS 8600, gRPC, RPC 8300, Serf 8301).
**Exercise:** run `./bin/consul agent -dev -log-level=debug` for 30s; map the first 40 log lines to the functions above (logger names like `agent.server.serf.lan`, `agent.server.raft` come from `logging/names.go`).
**Explain it back:** in ≤8 sentences, what a client agent has that a server lacks, and vice versa.
**Socratic:** (1) Why is `local.State` created before the delegate? (2) What would break if `StartSync` ran before `Start` returned? (3) Which fields of `Agent` exist only when `ServerMode` is true? (4) Where does `-dev` change Raft's log store?
**Stop when:** you can draw the goroutine/listener diagram of one dev agent. **Defer:** auto-config, TLS, enterprise segments/partitions, UI server.
**Agent-fleet note:** the "one binary, two roles" shape is worth copying for a worker-side sidecar vs a coordinator; the analogy ends at Raft — your workers should not vote.

## Session 2 — Local registration and health checks
**Question:** what exactly does a `PUT /v1/agent/service/register` change, and when does a check flip status?
**Prereqs:** S1.
**Read:** `agent/http_register.go:40-65`; `agent/agent_endpoint.go:AgentRegisterService`; `agent/agent.go:AddService → addServiceLocked → addServiceInternal`, `AddCheck`, `persistService`, `persistCheck`, `updateTTLCheck`; `agent/local/state.go:1-330` (types, `addServiceLocked`, `AddServiceWithChecks`) and `UpdateCheck` (`:670`); `agent/checks/check.go` (`CheckNotifier`, `CheckTTL`, `CheckHTTP.Start/check`, `StatusHandler`).
**Follow:** `ServiceState.InSync`, `CheckState.DeferCheck`, `CheckUpdateInterval`, `TriggerSyncChanges`.
**Trace:** TTL check registered → timer expiry → `UpdateCheck(critical)` → `InSync=false` → trigger.
**Exercise:** run `go test ./agent/checks -run TestCheckTTL -v` and `go test ./agent/local -run TestAgentAntiEntropy_Check_DeferSync -v`; then add a temporary `l.logger.Info("UPDATECHECK", "id", id, "status", status, "deferred", c.DeferCheck != nil)` in `UpdateCheck`, rerun, remove.
**Explain it back:** why output-only changes are deferred but status changes are not.
**Socratic:** (1) What is persisted under `data_dir/checks/state` and why separately from `data_dir/checks`? (2) How does `StatusHandler` implement "3 failures before critical"? (3) What is `isLocal`/`isLocallyDefined` for? (4) Why does `AddService` take a `token`?
**Stop when:** you can list every field of `local.CheckState` and say who writes it. **Defer:** alias checks, docker/OS-service checks, sidecar auto-registration.
**Agent-fleet note:** TTL checks are the closest match to "worker heartbeats"; the deferral logic is a pattern for throttling noisy status output.

## Session 3 — Anti-entropy and the server RPC path
**Question:** how does a local registration become a `Catalog.Register` RPC, and how does that RPC reach the leader?
**Prereqs:** S2.
**Read:** `agent/ae/ae.go` (all, ~350 lines); `agent/local/state.go:1026-1300` (`updateSyncState`, `SyncFull`, `SyncChanges`), `:1434-1572` (`syncService`, `syncCheck`, `syncNodeInfo`); `agent/agent.go:RPC`; `agent/consul/client.go:RPC`; `agent/router/manager.go` (`FindServer`, `NotifyFailedServer`, `RebalanceServers`); `agent/consul/rpc.go:617-910` (`getWaitTime`, `canRetry`, `ForwardRPC`, `forwardRequestToLeader`, `getLeader`, `forwardDC`).
**Follow:** `fsmState` transitions; `structs.RegisterRequest` construction; `SkipNodeUpdate`; the retry/jitter loop in both client and server.
**Trace:** a fresh registration with servers briefly unreachable: `partialSyncState` → RPC error → `InSync` stays false → next `syncChangesNotifEvent` or timer.
**Exercise:** `go test ./agent/ae -run TestAE_FSM -v`; `go test ./agent/local -run TestAgentAntiEntropy_Services$ -v`; `go test ./agent/consul -run 'TestRPC_NoLeader_Retry|TestCanRetry' -v`.
**Explain it back:** the difference between full sync and partial sync, and why both exist.
**Socratic:** (1) Why does permission-denied set `InSync=true`? (2) What is `scaleFactor` protecting? (3) Why does `Client.RPC` apply `RPCHoldTimeout` from the first call, not the first retry? (4) Which errors are retryable for writes vs reads?
**Stop when:** you can write the `RegisterRequest` JSON your agent would send for the running example. **Defer:** RPC rate limiting (`multilimiter`), yamux/TLS negotiation in `handleConn`.
**Agent-fleet note:** "local truth + periodic full reconcile + immediate partial sync" is a reusable design for worker registries; the ACL-denied edge case is a warning about silent failures.

## Session 4 — Catalog endpoint and the state store update
**Question:** what does the leader validate, and what rows change, when `Catalog.Register` is applied?
**Prereqs:** S3.
**Read:** `agent/consul/server_register.go`; `agent/consul/catalog_endpoint.go:109-440` (`Register`, pre-apply functions, `vetRegisterWithACL`, `Deregister`); `agent/consul/state/state_store.go:100-300`; `agent/consul/state/memdb.go` (`changeTrackerDB`, `txn.Commit`); `agent/consul/state/catalog_schema.go` (tables/indexes); `agent/consul/state/catalog.go:138-330` (`EnsureRegistration`, `ensureRegistrationTxn`), `:856-900` (`ensureServiceTxn`), `:2347-2400` (`ensureCheckTxn`).
**Follow:** the `idx` parameter; `IsSame` short-circuit; `indexUpdateMaxTxn`; `RaftIndex{CreateIndex, ModifyIndex}`.
**Trace:** registering the same service twice with identical content: which tables' indexes move? (Expect: none for services if `IsSame`.)
**Exercise:** `go test ./agent/consul/state -run 'TestStateStore_EnsureRegistration$|TestStateStore_EnsureCheck$' -v`; write a tiny throwaway test in a scratch `_test.go` that calls `NewStateStore(nil)`, `EnsureRegistration(1, …)`, then `CheckServiceNodes` and asserts the returned index; delete it after.
**Explain it back:** why `ensureRegistrationTxn` is one transaction and what "preserveIndexes" is for (restore).
**Socratic:** (1) Why does `Register` look up existing `NodeServices` before ACL vetting? (2) What does `SkipNodeUpdate` protect against? (3) How would a check for a service on a *different* node be rejected? (4) Which index does `CheckServiceNodes` return and why might it exceed the service's `ModifyIndex`?
**Stop when:** you can name the three catalog tables, their `id` index keys, and the write order. **Defer:** graveyard/tombstones, virtual IPs, peering/imported services, enterprise tenancy.
**Agent-fleet note:** the "one transaction per registration, index stamped on rows" pattern is what makes watches possible; note that `Meta` is not indexed, so capability lookups by meta are filters, not index scans.

## Session 5 — Raft apply and the FSM
**Question:** what happens between `raftApply` returning and a follower's state store showing the row?
**Prereqs:** S4.
**Read:** `agent/consul/rpc.go:939-1035` (`raftApply*`); `agent/consul/fsm/fsm.go` (whole file); `agent/consul/fsm/commands_ce.go:110-180` (`init`, `applyRegister`, `applyDeregister`); `agent/consul/server.go:setupRaft` (`:940-1192`); `agent/consul/leader.go:70-300` (`monitorLeadership`, `leaderLoop`, `establishLeadership`); `agent/consul/raft_handle.go`.
**Follow:** `structs.MessageType` byte prefix; `IgnoreUnknownTypeFlag`; `raftNotifyCh`; `raft.Barrier` in `leaderLoop`; `setConsistentReadReady`.
**Trace:** leader applies index N; follower receives AppendEntries **[library]**; follower's `fsm.Apply` runs `EnsureRegistration(N, …)`.
**Exercise:** `go test ./agent/consul/fsm -run 'TestFSM_RegisterNode_Service|TestFSM_DeregisterNode' -v`; then `go test ./agent/consul -run TestServer_Expect -v` and read the test to see 3 servers bootstrapping; add a temporary `c.logger.Info("APPLY", "type", msgType, "index", log.Index)` in `fsm.Apply`, run `TestFSM_RegisterNode_Service`, remove.
**Explain it back:** why `fsm.Apply` panics on unknown types unless flagged, and what a snapshot contains.
**Socratic:** (1) What does the barrier in `leaderLoop` guarantee before `reconcile()`? (2) Why is chunking only used for KV? (3) Which store is used for a fresh `data_dir` in this checkout and what enables log verification? (4) What is `raft_vsn` in Serf tags for?
**Stop when:** you can explain `future.Error()` semantics precisely. **Defer:** snapshot/restore internals, autopilot, raft-wal internals, `decode_downgrade.go`.
**Agent-fleet note:** if your harness needs a durable ordered log of fleet decisions, the FSM shape (byte-tagged commands, deterministic apply, snapshot) is the reference; but reach for Temporal/DB first.

## Session 6 — Query path and blocking queries
**Question:** how does `GET /v1/health/service/x?passing&index=N` block, wake, and choose consistency?
**Prereqs:** S4.
**Read:** `agent/http.go:790-1000` (`setMeta`, `parseWait`, `parseCacheControl`, `parseConsistency`), `:1200-1250` (`parse`); `agent/health_endpoint.go:174-260`; `agent/rpcclient/health/health.go`; `agent/consul/health_endpoint.go:206-360`; `agent/blockingquery/blockingquery.go`; `agent/consul/rpc.go:1035-1180` (`blockingQuery`, `SetQueryMeta`, `consistentReadWithContext`, `RPCQueryTimeout`); `agent/structs/structs.go:264-470` (`QueryOptions`, `QueryMeta`), `:2349-2420` (`HealthFilterType`, `Filter`); `agent/consul/state/catalog.go:2879-3000` (`CheckServiceNodes`, `checkServiceNodesTxn`).
**Follow:** `MinQueryIndex`, `WatchSet`, `ErrNotChanged`, `ResultsFilteredByACLs`, `X-Consul-Query-Backend`.
**Trace:** the same request with `?stale`, `?consistent`, and `Cache-Control: max-age=30`.
**Exercise:** `go test ./agent -run 'TestHealthServiceNodes_Blocking$|TestHealthServiceNodes_PassingFilter' -v`; `go test ./agent/blockingquery -v`; against a dev agent, register a service with curl, run `curl -i 'localhost:8500/v1/health/service/x?index=<N>&wait=10s'` in one terminal and change a check in another; observe headers.
**Explain it back:** in one paragraph, what the caller must do with `X-Consul-Index` and what a timeout return means.
**Socratic:** (1) Why is the streaming backend skipped when `MergeCentralConfig` is set? (2) Where would a spurious wake-up come from for a service with no changes? (3) Why is `RequireConsistent` re-checked on each loop iteration? (4) What does `HealthFilterExcludeCritical` include that `IncludeOnlyPassing` excludes?
**Stop when:** you can predict the headers for each consistency mode. **Defer:** streaming internals (S8 covers the shape only), prepared queries, bexpr filters.
**Agent-fleet note:** for a coordinator, "watch by index, diff locally, tolerate spurious wakeups" is the exact contract you will implement against Consul or your own registry.

## Session 7 — Gossip, membership, and leader reconciliation
**Question:** how does an agent's death become a critical `serfHealth` check without the agent doing anything?
**Prereqs:** S5.
**Read:** `agent/consul/server_serf.go:57-140, 270-400, 515-560`; `agent/consul/client_serf.go`; `internal/gossip/libserf/serf.go`; `agent/metadata/server.go:IsConsulServer`; `agent/consul/leader.go:145-280` (the `reconcileCh` handling) and `:940-1090` (`reconcileReaped`, `reconcileMember`); `agent/consul/server_ce.go:reconcile`; `agent/consul/leader_registrator_v1.go` (whole file); `agent/structs/catalog.go` (Serf check constants).
**Follow:** Serf tags; `localMemberEvent` leader gate; `StatusReap`; `HandleFailedMember` idempotence check (`check.Status == critical` short-circuit).
**Trace:** member failed → `reconcileCh` → `reconcileMember` → `HandleFailedMember` → `RaftApplyFunc(RegisterRequestType)` → S4/S5 path.
**Exercise:** `go test ./agent/consul -run 'TestLeader_FailedMember|TestLeader_Reconcile$|TestServer_LANReap' -v` (these boot real in-process servers; expect tens of seconds). Read `TestLeader_FailedMember` line by line.
**Explain it back:** why only the leader reconciles, and why `reconcile()` also runs on a timer.
**Socratic:** (1) What happens if the leader itself is the failed member? (2) What is the difference between `Left` and `Reap` handling? (3) Which Serf tag lets a client find a server's RPC port? (4) Why doesn't a client agent react to another client failing?
**Stop when:** you can state what membership does and does not tell you (write it as two lists). **Defer:** WAN pool, flood joins, network segments, coordinates/RTT.
**Agent-fleet note:** gossip-based liveness for worker hosts is a sound pattern; the leader-side "write a synthetic check" bridge is the key idea, and its false-positive/flap behaviour is the key caveat.

## Session 8 — A focused test/integration scenario
**Question:** can you reproduce the whole lifecycle in one in-process test and read every log line?
**Prereqs:** S1–S7.
**Read:** `agent/testagent.go` (`NewTestAgent`, `StartTestAgent`, `TestConfigHCL`, `randomPortsSource`); `testrpc/wait.go` (`WaitForLeader`, `WaitForTestAgent`, `WaitForAntiEntropySync`); `sdk/testutil/retry/run.go`; `agent/consul/server_test.go:84-330` (helpers); `agent/consul/helper_test.go:joinLAN`; `agent/local/state_test.go:TestAgentAntiEntropy_Services` (whole test).
**Follow:** how tests wait for leadership and anti-entropy rather than sleeping.
**Trace:** `NewTestAgent` → server+bootstrap dev config → `WaitForTestAgent` → `AddServiceWithChecks` → `StartSync`/`SyncFull` → `Catalog.NodeServices` assertion.
**Exercise:** copy `TestAgentAntiEntropy_Services` into a scratch test file, trim it to one service with one TTL check, add a `retry.Run` that waits until `/v1/health/service/<x>?passing` returns the instance, then call `a.State.UpdateCheck(critical)` and wait until it disappears. Run with `-v -log-level=debug` equivalents (`TestAgent` logs to `t`). Delete the file afterwards.
**Explain it back:** which test helper corresponds to each stage of `03-lifecycle-trace.md`.
**Socratic:** (1) Why does `TestAgent` default to server mode? (2) What does `WaitForAntiEntropySync` actually check? (3) What is `freeport` protecting against? (4) When would you use `sdk/testutil.TestServer` (external binary) instead?
**Stop when:** your scratch test passes twice in a row. **Defer:** docker-based suites (`test/integration`, `test-integ`, `testing/deployer`).
**Agent-fleet note:** this is the harness-testing pattern to copy — in-process cluster, wait-for-condition helpers, no sleeps.

## Session 9 — Streaming, agent cache, and materialized views
**Question:** how does the agent avoid re-running blocking queries per consumer?
**Prereqs:** S6.
**Read:** `agent/consul/stream/event_publisher.go`; `agent/consul/state/catalog_events.go:1-300`; `agent/consul/subscribe_backend.go`; `agent/grpc-internal/services/subscribe/subscribe.go`; `agent/submatview/materializer.go`, `rpc_materializer.go`, `store.go`; `agent/rpcclient/health/view.go`; `agent/cache/cache.go:300-600` (`Get`, `fetch`), `agent/cache-types/health_services.go`.
**Follow:** topic/subject, snapshot then events, `resetErr`, view `Update`, `Store.Get` by index.
**Trace:** two consumers blocking on the same service via streaming: one subscription, one view, two waiters.
**Exercise:** `go test ./agent/consul/stream -run TestEventPublisher_SubscribeWithIndex0 -v`; `go test ./agent/submatview -run TestStore_Notify -v`; `go test ./agent/rpcclient/health -run TestClient_ServiceNodes_BackendRouting -v`.
**Explain it back:** the three backends (direct RPC, cache, streaming) and the request shapes that select each.
**Socratic:** (1) Why does an ACL change reset subscriptions? (2) What bounds the event buffer? (3) How is a snapshot for a new subscriber served without blocking publishers? (4) Where is `X-Consul-Query-Backend` set?
**Stop when:** you can explain `useStreaming` predicate from memory. **Defer:** peering streams, connect topic details.
**Agent-fleet note:** a "subscribe → snapshot → events → local view" pipeline is what your coordinator wants instead of N pollers.

## Session 10 — ACL boundary (not the implementation)
**Question:** where exactly is a token turned into a decision on the registration and query paths?
**Prereqs:** S3, S6.
**Read:** `acl/authorizer.go:1-200`; `acl/policy.go` (skim rule syntax); `agent/consul/acl.go:139-340, 1057-1200` (`ACLResolverBackend`, `ACLResolver` struct, `ResolveToken`, `ResolveTokenAndDefaultMeta`, `filterACL`); `agent/consul/acl_client.go`; `agent/structs/aclfilter/filter.go:1-60, 330-360`; `agent/consul/catalog_endpoint.go:vetRegisterWithACL`; `agent/acl.go:35-120`; `agent/token/store.go:200-300`.
**Follow:** `EnforcementDecision`; `ResultsFilteredByACLs`; down policy in resolver config.
**Trace:** registration with a token lacking `node:write`; query with a token that can read the service but not the node.
**Exercise:** `go test ./agent/consul -run 'TestACLResolver_DownPolicy|TestHealth_ServiceNodes_FilterACL' -v`; `go test ./agent -run TestAgent_RegisterService_ACLDeny -v`.
**Explain it back:** the three enforcement points (agent vet, leader vet, read filter) and what each protects.
**Socratic:** (1) Why filter reads instead of denying them? (2) How does a client survive server outage for token resolution? (3) Which token does anti-entropy use for a service registered with its own token? (4) Why did 2.0.4 add `mesh:write` requirements (read the CHANGELOG entry)?
**Stop when:** you can draw the token flow for one registration. **Defer:** auth methods, roles/binding rules, replication, tokens' expiration.
**Agent-fleet note:** Consul ACLs answer "may this identity write this catalog row"; your tool-invocation authorization is a separate layer.

## Session 11 — Connect control plane shape (overview)
**Question:** how does a catalog health change reach an Envoy sidecar?
**Prereqs:** S6, S9.
**Read:** `agent/proxycfg/manager.go`; `agent/proxycfg/state.go:60-140, 184-300, 339-420`; `agent/proxycfg/data_sources.go:40-200`; `agent/proxycfg-glue/health.go`; `agent/proxycfg-sources/local/sync.go`; `agent/xds/server.go:189-350`; `agent/xds/delta.go:63-220`; `agent/xds/resources.go`; skim `agent/xds/endpoints.go`.
**Follow:** `ProxyID`, `ConfigSnapshot`, coalesce timer + rate limiter, `stateCh`/`reqCh`, ACK/NACK nonces, `authorize` per snapshot.
**Trace:** health event → `Health.Notify` callback → `state.run` → snapshot → `Manager.notify` → `processDelta` → EDS update.
**Exercise:** `go test ./agent/proxycfg -run TestManager_BasicLifecycle -v`; `go test ./agent/xds -run TestServer_DeltaAggregatedResources_v3_BasicProtocol_TCP -v` (uses golden files under `agent/xds/testdata`).
**Explain it back:** what the control plane can and cannot guarantee about traffic.
**Socratic:** (1) Why coalesce? (2) What happens on a NACK loop (`TestServer_DeltaAggregatedResources_v3_NackLoop`)? (3) Where do intentions become enforceable? (4) What does agentless (`proxycfg-sources/catalog`) change?
**Stop when:** you can name the four xDS resource types and where each is generated. **Defer:** everything else in `agent/xds`, gateways, extensions, CA providers.
**Agent-fleet note:** "compute snapshot, coalesce, push, wait for ACK" is directly reusable for pushing routing tables to workers; mTLS identity via SPIFFE IDs (`connect/`) is the pattern for worker identity.

## Session 12 — Failure drills and contribution readiness
**Question:** can you predict and then observe behaviour under leader loss and partition?
**Prereqs:** all.
**Read:** `agent/consul/rpc_test.go:49-330` (`TestRPC_NoLeader_*`, `TestRPC_ReadyForConsistentReads`); `agent/consul/server_test.go:TestServer_LeaveLeader`; `agent/consul/leader_test.go:TestLeader_RollRaftServer`; `.github/CONTRIBUTING.md`; `docs/config/checklist-adding-config-fields.md`; `docs/contributing/add-a-changelog-entry.md`; `.github/pull_request_template.md`.
**Trace:** write a prediction table (writes, default reads, stale reads, consistent reads, DNS, anti-entropy) for: leader killed; 2 of 3 servers killed; client isolated.
**Exercise:** start 3 dev servers locally (`consul agent -server -bootstrap-expect=3 …` with distinct ports/data dirs, or reuse `TestServer_LeaveLeader` in a scratch test), register a service, kill the leader, and verify each row of your table with curl headers. No commits.
**Explain it back:** the five invariants in `07-next-action.md`, in your own words, with one code location each.
**Socratic:** (1) Which test would you extend to cover a behaviour you found undocumented? (2) What is the smallest PR-shaped change you could make with confidence? (3) Which two subsystems would you refuse to touch yet, and why? (4) What would a bug report for a flapping `serfHealth` need to include?
**Stop when:** your prediction table matched observation (or you can explain each mismatch). **Defer:** performance tuning, enterprise features, WAN federation.
**Agent-fleet note:** the drills are the same ones you will run against your own harness; Consul gives you a reference for what "graceful degradation" concretely means per layer.

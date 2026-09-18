# 01 — Part A: the first-principles model

Read this before the code. Every term is defined before it is relied on. Claims marked **[code]** were checked in this checkout; **[inference]** is explanation.

Consul is a service networking solution that provides service discovery, secure communication (service mesh), and network automation across multi-cloud and dynamic infrastructure.

## Consul vs Kubernetes

HashiCorp Consul is a dedicated network automation and service mesh tool, whereas Kubernetes is a comprehensive container orchestration platform. While Kubernetes handles the entire lifecycle of containers (including its own basic networking), Consul focuses purely on connecting, securing, and routing traffic across any infrastructure—whether it runs on Kubernetes, virtual machines, or bare metal.

They are not mutually exclusive. In fact, large enterprises frequently use them together.In a hybrid environment, Kubernetes manages the containers inside a cluster, while Consul acts as the bridge that allows those Kubernetes containers to securely communicate with legacy applications running on traditional virtual machines or external clouds.

HashiCorp Nomad is the tool used for container and workload orchestration. While Nomad handles the scheduling and deployment of your containers, it is frequently paired with Consul for networking and Vault for secrets management to create a complete alternative to the Kubernetes ecosystem.

The choice between the HashiCorp Stack (Nomad, Consul, Vault) and Kubernetes comes down to a tradeoff between architectural simplicity and ecosystem dominance. While Kubernetes provides an all-in-one platform tightly coupled to containers, the HashiCorp stack uses a modular approach where each tool excels at a single, specific job.

------------------------------
### Direct Comparison

| Metric | 🛠️ The HashiCorp Stack (Nomad + Consul + Vault) | ☸️Kubernetes (All-in-One) |
|---|---|---|
| Architecture | Modular. Three distinct, lightweight binaries that can be deployed independently. | Monolithic/All-in-One. A complex, tightly coupled control plane. |
| Workload Types | Any workload. Native support for containers, legacy binaries, Java, and micro-VMs. | Container-centric. Optimized almost exclusively for OCI containers. |
| Learning Curve | Low to Moderate. Nomad is straightforward to learn; Consul/Vault add step-by-step complexity. | High. Requires mastering dozens of native API objects, CRDs, and networking layers. |
| Resource Overhead | Very Low. The control plane uses minimal CPU/RAM; great for edge computing. | High. The control plane requires significant system overhead just to idle. |
| Ecosystem & Support | Smaller. Fewer community plug-ins and managed cloud options. | Massive. Industry standard with endless third-party integrations and managed services (EKS, GKE). |

------------------------------
### The HashiCorp Stack
### Pros

* Simplicity and Operational Ease: Nomad is a single binary that is significantly easier to install, operate, and upgrade than a standard Kubernetes cluster.
* Workload Flexibility: Nomad orchestrates non-containerized legacy applications (like standard Java JARs, Windows executables, or raw binaries) right alongside Docker containers.
* Best-in-Class Security and Networking: Vault is the gold standard for secrets management, and Consul offers robust cross-platform networking. Using them means you don't have to rely on Kubernetes' weaker native secrets and basic cluster DNS.
* Resource Efficiency: The stack runs flawlessly on constrained hardware, making it ideal for edge deployments, IoT, and bare-metal environments.

### Cons

* Integration Overhead: You are responsible for wiring Nomad, Consul, and Vault together yourself using configuration files.
* Smaller Community and Talent Pool: It is much harder to find platform engineers who specialize in Nomad compared to the vast pool of Kubernetes experts.
* No "Click-and-Run" Cloud Managed Services: There is no direct equivalent to AWS EKS or Google GKE for Nomad; you generally have to manage the underlying control plane infrastructure yourself.

------------------------------
### Kubernetes
### Pros

* The Industry Standard: Kubernetes has won the orchestration wars. It boasts a massive ecosystem of tools (Helm, ArgoCD, Prometheus) that target it natively.
* Fully Managed Cloud Offerings: Every major cloud provider offers a managed Kubernetes service that abstracts away the pain of managing the control plane.
* All-in-One Out of the Box: Out of the box, Kubernetes handles scheduling, basic service discovery (via CoreDNS), and basic secrets tracking without needing external tools.

### Cons

* Extreme Complexity: Known for its "Kubernetes tax"—the massive amount of engineering hours required just to maintain and configure the platform.
* Strictly Containers: If you have legacy software that cannot be containerized, you cannot run it natively alongside your containerized apps in Kubernetes.
* Weak Native Subsystems: Kubernetes' native secrets are merely Base64 encoded strings (not securely encrypted at rest by default), and its basic ingress/networking often forces teams to install a third-party service mesh anyway.

## Glossary (in dependency order)

| Term | Definition (as used in Consul's code) |
|---|---|
| **Node** | A machine/agent identity in the catalog. Type `structs.Node`. Named by `NodeName`, identified by `NodeID`. |
| **Service instance** | One process on one node offering a named service. Type `structs.NodeService` (`ID`, `Service`, `Tags`, `Meta`, `Address`, `Port`, `Kind`). |
| **Health check** | A per-node or per-service assertion whose status is `passing`/`warning`/`critical`. Type `structs.HealthCheck` (`CheckID`, `Status`, `Output`, `ServiceID`, `Type`). |
| **Service registration** | Telling *some* agent that a service instance exists. In Consul this first lands in the **local agent state** and is later synchronized to the catalog. |
| **Service discovery** | A consumer asking "which instances of service X exist, and which are healthy?" Answered from the **catalog**, not from the local agent that registered them. |
| **Health checking** | The act of executing checks (HTTP/TCP/TTL/gRPC/script) and turning results into check status. Done by the agent that owns the service. |
| **Membership / failure detection** | Which *agents* (processes) are alive, suspect, failed, or left, as learned by gossip (Serf over memberlist). This is about agents, not services. |
| **Service catalog** | The authoritative table of nodes, services, and checks, held by servers in the Raft-replicated in-memory state store. |
| **Control-plane state store** | The server-side in-memory database (`go-memdb`) that Raft's FSM writes into and reads answer from. Type `state.Store`. |
| **Raft index** | A monotonically increasing log position. Every catalog write carries the index at which it was applied; reads return the max index of the tables they touched. |
| **Consistency mode** | Per-request choice: `default` (leader-served), `stale` (any server), `consistent` (leader verified). Fields on `structs.QueryOptions`. |
| **Blocking query** | An HTTP/RPC read that carries `MinQueryIndex` and does not return until the relevant index exceeds it or a timeout fires. Consul's watch primitive. |
| **Streaming / materialized view** | An alternative to polling blocking queries: servers publish change events; the agent maintains a local view. |
| **Service mesh control plane** | The part that decides what proxies should do (listeners, clusters, endpoints, certs, intentions) and pushes that via xDS. |
| **Data plane** | The proxies (Envoy, or consul-dataplane) that actually carry and enforce traffic. Consul does not sit on the data path. |
| **Anti-entropy** | The agent-side loop that reconciles local agent state with the catalog, both incrementally and by periodic full compare. |
| **Reconcile (leader)** | The leader-side loop that reconciles gossip membership with catalog node entries and the `serfHealth` check. |

## A.1 Distinctions that must not blur

1. **Registration vs discovery.** Registration is an input to the *local agent* (`PUT /v1/agent/service/register`, config files). Discovery is a query against the *catalog* (`GET /v1/health/service/<name>`, DNS). Between them sits an asynchronous sync. **[code]** `agent/agent.go:AddService` writes `local.State`; `agent/local/state.go:syncService` later issues the `Catalog.Register` RPC.
2. **Health checking vs failure detection.** Health checks describe a *service instance* and are executed by the owning agent. Failure detection describes an *agent process* and is done by gossip. They meet only in one place: the leader writes a synthetic `serfHealth` check on the node when gossip says the agent failed. **[code]** `agent/consul/leader_registrator_v1.go:HandleFailedMember` writes `structs.SerfCheckID` with `api.HealthCritical`.
3. **Membership vs catalog.** Membership (Serf member list) is eventually consistent and can disagree between agents. The catalog is Raft-consistent. A node can be in the catalog while absent from gossip (e.g. externally registered nodes via `/v1/catalog/register`) and vice versa (agent joined, catalog not yet reconciled).
4. **Catalog vs consistent state store.** The catalog is a *schema* (tables `nodes`, `services`, `checks`) inside the state store, which also holds KV, sessions, ACL, config entries, intentions, CA state, etc. **[code]** `agent/consul/state/catalog_schema.go`.
5. **DNS / HTTP / gRPC lookup.** Three surfaces on the same agent that all end up calling `Health.ServiceNodes` (or the catalog RPCs) with different defaults: DNS defaults to stale reads and filters critical instances; HTTP lets the caller choose; gRPC `pbdns.Query` wraps the DNS handler.
6. **Blocking query vs watch.** A blocking query is a long-poll keyed on an index. A "watch" (`api/watch`, `consul watch`) is a client loop of blocking queries. Streaming (`pbsubscribe`) is push-based and used internally by the agent for the health cache and proxycfg.
7. **Mesh control plane vs data plane.** The control plane (server state + `proxycfg` + `xds`) computes and pushes Envoy configuration; enforcement (mTLS, RBAC, routing) happens inside Envoy. Consul cannot observe or block a connection Envoy allows.

## A.2 Why a central in-memory registry is not enough

An in-memory map on one process gives you registration and lookup, but not:

- **Durability across leader loss.** A single process losing memory loses the fleet. Consul replicates every catalog write through Raft to a quorum before acknowledging. **[code]** `agent/consul/rpc.go:raftApplyEncoded` waits on `future.Error()`.
- **Liveness of the registrar itself.** Who notices that the registry died? Consul's gossip layer makes every agent a failure detector for every other agent.
- **Staleness accounting.** Callers must be able to ask "how old is this answer?" Consul returns `X-Consul-Index`, `X-Consul-LastContact`, `X-Consul-KnownLeader` on every read. **[code]** `agent/http.go:setMeta`.
- **Local survival when the control plane is unreachable.** An agent keeps its own registrations, keeps running checks, and can serve cached/stale answers. **[code]** `agent/local/state.go`, `agent/cache`, `agent/checks`.
- **Ownership.** A registry needs a rule for *who is allowed to change an entry*. Consul makes the owning agent the source of truth for its own services (anti-entropy will re-assert them) and ACLs gate writes.

## A.3 Five categories of state (never conflate them)

| Category | Where it lives | Owner | Consistency | Durable? |
|---|---|---|---|---|
| **Local agent state** | `local.State` in the agent process; persisted per service/check under `data_dir/services`, `data_dir/checks` | The agent | Authoritative *for that agent's own services*; may be ahead of the catalog | Yes (files) **[code]** `agent/agent.go:persistService`, `persistCheck` |
| **Membership / failure-detection state** | Serf/memberlist member list in every agent | No single owner; converges by gossip | Eventually consistent; each agent has its own view | Serf snapshot file for rejoin only |
| **Authoritative catalog/config state** | `state.Store` on every server, written only via Raft FSM | The Raft leader (writes); all servers hold copies | Linearizable at the leader; followers lag by replication | Raft log (WAL or BoltDB) + snapshots |
| **Client-side cached / stale read state** | `agent/cache` and `agent/submatview` views in the agent; any follower's FSM copy for `stale` reads | The reader | Explicitly stale, with `LastContact`/`Index` metadata | No |
| **Proxy / data-plane configuration state** | `proxycfg.ConfigSnapshot` in the agent (or server for dataplanes); Envoy's own applied config | proxycfg manager → xDS → Envoy | Derived; eventually consistent with catalog; Envoy may reject (NACK) | No |

## A.4 What servers and clients respectively do

**Server agent** (`agent/consul/server.go:Server`): participates in Raft (voter or non-voter), holds the full state store, runs RPC endpoints (`agent/consul/server_register.go`), joins LAN gossip and (usually) WAN gossip, runs the leader loop when elected (reconcile membership, ACL/CA init, autopilot), resolves ACL tokens locally, publishes change events for streaming, and serves xDS for dataplanes/proxies that connect to it. The server also embeds a full **agent** (same `agent.Agent` struct; `a.delegate` is the `*consul.Server`).

**Client agent** (`agent/consul/client.go:Client`): joins LAN gossip only, owns local services and checks, runs anti-entropy, forwards *every* RPC to a server chosen by `agent/router` (`FindLANRoute`), caches read results, resolves ACL tokens by asking servers (with a down-policy cache), and serves HTTP/DNS/gRPC/xDS for local consumers and local proxies. Clients never hold catalog state and never vote.

**[code]** The two are selected in `agent/agent.go:Start` (`if c.ServerMode { consul.NewServer(...) } else { consul.NewClient(...) }`) and hidden behind the `delegate` interface (`agent/agent.go:154`).

## A.5 Why both consensus and gossip

**Consensus (Raft)** is needed for state where two servers disagreeing is a correctness bug: the catalog, KV, sessions, ACL tokens/policies, config entries, intentions, CA roots, Raft configuration itself. Property required: a single total order of writes, survivable across a minority of server failures, readable with a known index.

**Gossip (Serf/memberlist)** is used for information that is fine to learn late or slightly wrong, but that must scale to thousands of agents without a central bottleneck: which agents exist, their tags (role, dc, ports, version), whether they appear alive, and cluster-wide events (leave, user events, key rotation). Property required: every agent eventually learns about every other; failure is *suspected* then *confirmed* via indirect probes; no quorum needed.

What users should infer from membership: "this agent process is (probably) reachable on the gossip port". What they should **not** infer: that services on that node are healthy, that the catalog already reflects the membership change, or that a node absent from `consul members` has been deregistered. **[code]** Gossip only touches the catalog through the leader's reconcile path (`agent/consul/leader.go:reconcileMember`), and only affects the `serfHealth` check and node entry, never service checks.

Effect of failures on each category:

| Event | Membership | Catalog | Local agent state | Cache/stale reads | Proxy config |
|---|---|---|---|---|---|
| One client agent dies | Others mark it failed after probes; leader eventually reaps | Leader sets node's `serfHealth` critical; services remain but are filtered as unhealthy | Lost with the process (files remain for restart) | Consumers see change on next blocking-query wake or stream event | Upstream endpoints drop that instance |
| One server dies (quorum intact) | Marked failed; clients drop it from router | Unaffected if it was a follower; new election if leader | Unaffected | Stale reads from that server fail over; `LastContact` may rise | Unaffected |
| Quorum lost | Gossip continues | **Writes fail** (`ErrNoLeader`); `stale` reads still served; `default`/`consistent` reads fail | Agent keeps running checks; anti-entropy retries | Cache keeps serving until TTL; `X-Consul-KnownLeader: false` | Existing Envoy config stays; no updates |
| Client partitioned from servers | Peers on its side still see it; the server side marks it failed | Leader marks `serfHealth` critical → instances filtered out on the server side | Unchanged; local `/v1/agent/*` still works | Local cache/stale answers only | Frozen |

## A.6 The central tradeoffs Consul makes

- **Consistency vs availability for reads**: pushed to the caller via `?stale`/`?consistent`. Default reads go to the leader but do not verify leadership (a partitioned old leader could answer briefly). **[code]** `structs.QueryOptions.ConsistencyLevel`, `Server.consistentReadWithContext` (`raft.VerifyLeader` only when `RequireConsistent`).
- **Liveness under leader loss**: writes and non-stale reads block/retry up to `rpc_hold_timeout` (default `7s`) hoping for a new leader. **[code]** `agent/consul/rpc.go:forwardRequestToLeader`, `canRetry`, `agent/config/default.go` `rpc_hold_timeout = "7s"`.
- **Failure detection uncertainty**: gossip can declare a slow agent failed (false positive) and recovery is automatic on rejoin; the catalog therefore flaps rather than lying permanently.
- **Propagation delay**: registration → catalog is asynchronous (anti-entropy), catalog → consumer is index-gated (blocking query) or event-driven (streaming), catalog → Envoy is coalesced and rate-limited (`proxycfg` coalesce timer). None of these are zero.
- **Stale reads are a feature**: DNS defaults to `allow_stale = true` with `max_stale = 87600h` so that name resolution survives leader loss. **[code]** `agent/config/default.go:95-98`.
- **Quorum requirement**: every write costs a round trip to a majority of servers; Consul accepts this to make the catalog trustworthy.



# Plan: Splunk Dashboard Studio Redesign

## Status: Phase 1-8 COMPLETE — Iterating on query fixes

All 7 dashboards created, imported into Splunk, import script working, README updated. Currently iterating on SPL query accuracy based on live testing.

## TL;DR
Complete redesign of the Cisco MDT telemetry dashboard from Classic XML (30 panels in one file) to a multi-dashboard Splunk Dashboard Framework (JSON) suite covering all active telemetry subscriptions. Organized as a main overview dashboard with navigation links to 6 category-specific dashboards.

## Architecture — 7 Dashboards

| Dashboard | Focus | Key Subscriptions |
|-----------|-------|-------------------|
| **Overview** | System vitals at a glance + navigation | 1001, 1002, 1005, 1006, 1007, 1016 |
| **Infrastructure** | CPU, memory, DRAM, environment, stack, hardware, HA | 1001-1005, 1009, 1016, 1023, 1028 |
| **Network** | Interfaces, VLANs, STP, ARP, MAC, LLDP, CDP, switchport | 1007-1008, 1010-1014, 1017 |
| **Routing** | BGP, OSPF, RIB/FIB, VRF, DHCP, NTP | 1024-1027, 1033, 1042, 1046 |
| **Power & PoE** | Sensors, PoE module/port/budget, PSU, temp, fans | 1005, 1006 |
| **Security** | 802.1X, TrustSec, MACsec, MKA, ACLs, TCAM | 1020-1021, 1030, 1032, 1041, 1141 |
| **Telemetry Health** | MDT subs, data volume, connections, DP resources | 1022, 1043, 1044 |

## Steps

### Phase 1: Overview Dashboard
Create `cisco_mdt_overview.json` with global inputs (switch selector, time range):
- **Row 1**: Device identity table (sw version, boot time, reboot reason, chassis PID, serial) — Sub 1016
- **Row 2**: CPU gauge (single-value, five-seconds) + Memory gauge (% used) + PoE gauge (% budget) — Subs 1001, 1002, 1006
- **Row 3**: Interface throughput sparkline (top 5 in-octets + out-octets) — Sub 1007
- **Row 4**: Environment sensor summary (temp sensors, fan state, PSU state) — Sub 1005
- **Row 5**: Navigation links to 6 category dashboards

### Phase 2: Infrastructure Dashboard
*Depends on Phase 1 for input pattern.*
Create `cisco_mdt_infrastructure.json`:
- **CPU section**: Line chart (5s/1m/5m over time), single-value gauge, top 10 per-process CPU — Sub 1001
- **Memory section**: Pool usage table + % gauge, per-process holding memory bar chart, allocated vs freed — Subs 1002, 1003
- **DRAM section**: Platform software memory (total/used/free GB, % committed) — Sub 1004
- **Environment**: Temp line charts per sensor, fan RPM table, PSU wattage — Sub 1005
- **Stack**: Ring speed/status, member table, keepalive counters — Sub 1009
- **Hardware**: Device inventory table (chassis, CPU, DRAM, PEM, EMMC) — Sub 1016
- **Install**: Software version, installed packages — Sub 1023
- **HA**: HA state, switchover count/reason — Sub 1028

### Phase 3: Network Dashboard
*Parallel with Phase 2.*
Create `cisco_mdt_network.json`:
- **Interfaces**: Throughput area chart (in/out-octets), Rx/Tx kbps line chart, errors/discards table, interface status table — Sub 1007
- **VLANs**: VLAN inventory table (ID, name, status, interfaces) — Sub 1010
- **STP**: Topology changes line chart, root bridge table, port state — Sub 1008
- **ARP**: ARP table (IP, MAC, interface, VRF) — Sub 1012
- **MAC**: MAC address table, total entry count — Sub 1011
- **LLDP**: Enabled interfaces, frame counters — Sub 1013
- **CDP**: Neighbor table (device, local intf, remote port, platform, mgmt IP) — Sub 1014
- **Switchport**: Mode distribution pie chart, interface detail table — Sub 1017

### Phase 4: Routing Dashboard
*Depends on Phase 1.*
Create `cisco_mdt_routing.json`:
- **BGP**: AFI/SAFI enumeration table, route-distinguisher values — Sub 1024
- **OSPF**: Operational mode (limited data — 2 points) — Sub 1025
- **RIB**: IPv4/IPv6 route counts by network instance — Sub 1026
- **FIB/CEF**: Total/forwarding/non-forwarding prefixes, adjacency counts, route tables — Sub 1046
- **VRF**: VRF entries with address-family — Sub 1042
- **DHCP**: DHCPv6 relay binding stats — Sub 1027
- **NTP**: Clock drift, peer associations (stratum, delay, jitter, offset) — Sub 1033

### Phase 5: Power & PoE Dashboard
*Parallel with Phase 4.*
Create `cisco_mdt_power.json`:
- **PoE Budget**: Gauge (% utilization), summary table (consumed, budget, unused) — Sub 1006
- **PoE Modules**: Available power, free/used ports, remaining power — Sub 1006
- **PoE Port Detail**: Per-port power table (intf, oper-power, admin-max, device detected) — Sub 1006
- **PSU**: Power supply readings from environment sensors (watts) — Sub 1005
- **Temperature**: Line chart per probe (Inlet, Outlet, HotSpot) — Sub 1005
- **Fans**: RPM readings, state — Sub 1005

### Phase 6: Security Dashboard
*Parallel with Phase 4.*
Create `cisco_mdt_security.json`:
- **802.1X**: EAPOL counters table (Rx/Tx/Start/Logoff/Response/Invalid) — Sub 1020
- **WebAuth**: HTTP and AAA stats — Sub 1020
- **TrustSec**: Device SGT, environment data status, server count — Sub 1030
- **ACLs**: ACL type table, match counter by rule — Sub 1032
- **MACsec**: Rx/Tx packet counters (bad tag, no SCI, etc.) — Sub 1041
- **MKA**: MKPDU stats (Rx/Tx/DistCAK) — Sub 1141
- **TCAM**: Utilization gauges (TCAM/hash/TLB entries used vs max) — Sub 1021

### Phase 7: Telemetry Health Dashboard
*Parallel with Phase 4.*
Create `cisco_mdt_telemetry.json`:
- **Connection State**: Active connections, peer ID, state (con-state-active) — Sub 1022
- **Subscription Health**: Table of all subs (ID, xpath, state, last-change-time) — Sub 1022
- **Data Volume**: Stacked area chart of data points by subscription over time
- **System Limits**: Max subscriptions gauge (150), valid count (79) — Sub 1022
- **Dataplane Resources**: Resource utilization by feature — Sub 1043
- **Punt/Inject**: CPU queue stats — Sub 1044

### Phase 8: Import Script & Docs
- Create `scripts/import-dashboards.sh` — automate REST API import of all 7 JSON dashboards into Splunk
- Update `start-splunk.sh` — add dashboard auto-import after index creation
- Update `README.md` — update dashboard section to describe the 7 dashboards
- Backup existing `cisco_mdt_overview.xml` → `cisco_mdt_overview.xml.bak`

## Relevant Files
- `splunk-dashboards/cisco_mdt_overview.xml` — current 30-panel Classic XML dashboard (to be replaced)
- `splunk-dashboards/*.json` — 7 new Dashboard Framework JSON files (to create)
- `scripts/import-dashboards.sh` — dashboard import script (to create)
- `start-splunk.sh` — add auto-import step
- `README.md` — update dashboard documentation section
- `telemetry-capture/_all-data-txt/*.txt` — reference data for SPL query design (metric names, entity keys)
- `collector-config.yaml` — Splunk HEC config (index=cisco_mdt, token=cisco-mdt-token)

## Verification
1. Run `./start-splunk.sh` → Splunk starts, index created, dashboards imported
2. Run `./build/cisco-otelcol --config collector-config.yaml` → telemetry flowing
3. Open http://<host>:8000 → Overview dashboard loads with device info, CPU/mem/PoE gauges
4. Click navigation links → each category dashboard loads with correct panels
5. Switch selector and time range inputs filter all panels correctly
6. Spot-check SPL queries against captured data.txt metric names — values match
7. Test with `cisco.node_id="*"` (all switches) and specific switch selection

## Decisions
- **Format**: Splunk Dashboard Framework JSON (not Classic XML)
- **Layout**: 7 dashboards with navigation links from overview (not single scrollable page)
- **Backup**: Keep old XML as `.bak` — don't delete
- **Coverage**: All 35 active subscriptions with data; subs with no data (BFD, HSRP, VRRP, etc.) excluded
- **Scope included**: Dashboard JSON, import script, start-splunk.sh update, README update
- **Scope excluded**: Receiver code changes (key-value correlation is separate engineering task), Prometheus/Grafana dashboards

## Considerations
1. **Cross-dashboard tokens** — Dashboard Framework supports URL param passing for `switch_node` and `time_range` when navigating between dashboards.
2. **Metric naming dependency** — The `cisco.content.` prefix and `_info` suffix depend on the receiver's YANG parser output. If naming convention changes, all SPL queries need updating.
3. **OSPF panel** — Sub 1025 only has 2 data points (op-mode). Routing dashboard OSPF section will be minimal — just a state indicator. Could be omitted if it adds clutter, or kept as a placeholder for environments with active OSPF.

## Completed Work Log

### Phase 1-7: Dashboard JSON Files — DONE
All 7 dashboard JSON files created in `splunk-dashboards/`:
- `cisco_mdt_overview.json` — 8 visualizations, 9 data sources
- `cisco_mdt_infrastructure.json` — 18 visualizations, 18+ data sources
- `cisco_mdt_network.json` — 15 visualizations, 16+ data sources
- `cisco_mdt_routing.json` — 13 visualizations, 14+ data sources
- `cisco_mdt_power.json` — 11 visualizations (updated with real fan RPM, PSU capacity, env overview)
- `cisco_mdt_security.json` — 10 visualizations, 12+ data sources
- `cisco_mdt_telemetry.json` — 7 visualizations (updated with working queries)

### Phase 8: Import Script & Docs — DONE
- `scripts/import-dashboards.sh` — Python-based import (handles CDATA/XML escaping correctly)
- `start-splunk.sh` — auto-imports dashboards after index creation
- `README.md` — updated with 7-dashboard table, new import instructions, project structure
- `cisco_mdt_overview.xml` backed up as `.xml.bak`
- Git committed (not yet pushed to GitHub)

### Post-Phase Bug Fixes Applied
These fixes were applied during live testing with actual switch data:

1. **Telemetry Health — Subscription Health table**: Changed from `count("cisco.content.five-seconds")` (only showed Sub 1001) to `mcatalog values(metric_name) ... BY cisco.subscription_id` — now shows all 79 subscriptions.

2. **Telemetry Health — Data Volume chart**: Changed from `mstats count("cisco.content.*")` (hit Splunk wildcard expansion limit) to `mstats latest(_value) WHERE metric_name="cisco.content.*"` then `stats count BY subscription_id` — now renders correctly.

3. **Telemetry Health — Data Points by Path table**: Same `mcatalog` fix, table row limit increased from 50 to 100.

4. **Power — Fan Speed (RPM)**: Changed from `environment-sensor.current-reading` (returned 0 for fans) to `cisco.content.value.string_info` with `cname="Fan*"` filtering numeric values > 100 — now shows actual RPM (5440/12960).

5. **Power — PSU Type & Capacity**: Changed from `environment-sensor.current-reading` (returned 0 for PSU) to `cisco.content.value.string_info` with `cname="PowerSupply*"` filtering AC/DC type and integer values >= 100 — now shows AC, 350W/1100W.

6. **Power — Fan/PSU Sensor State**: Changed from `cisco.content.state.empty/removable` (no data) to `cisco.content.environment-sensor.state_info` — now shows Norm/NotExist per sensor.

7. **Power — Environment Sensor Overview (new panel)**: Added combined table showing all environment sensors (temp, fan, PSU) with latest reading, units, and state in one view.

8. **Import Script**: Rewrote from bash curl to Python urllib — bash CDATA `<![CDATA[` was conflicting with bash `!` history expansion. Now handles XML escaping, `&` in titles, and create-vs-update logic correctly.

## Known Issues & Future Work
- **Key-value correlation gap**: Fan RPM, PSU watts, and other platform-property values are stored as generic `cisco.content.value.string_info` with `cname` identifying the component but `name` (speed, input-voltage, capacity, etc.) stored as a **separate key metric** (`cisco.keys.name_info`). Without key propagation in the receiver, we can't label which value is RPM vs watts vs voltage in the same query. Current workaround: filter by cname pattern + value range heuristics.
- **Other dashboards not yet tested live**: Infrastructure, Network, Routing, and Security dashboards have not been tested against live Splunk data yet. Queries were adapted from proven XML dashboard queries but may need similar fixes when tested.
- **Dashboard Studio rendering**: Some panels may need layout/size adjustments once viewed in the actual Dashboard Studio UI. Grid positioning was set programmatically.
- **PoE Port Detail query**: Uses multi-append pattern to correlate intf-name with oper-power. May need refinement if there are many ports with power data.
- **Navigation links**: Overview dashboard nav links use `/app/search/cisco_mdt_*` URL pattern — verify these work in the Splunk app context.

## Import Commands
```bash
# Import all dashboards
./scripts/import-dashboards.sh

# Import with custom Splunk URL/credentials
./scripts/import-dashboards.sh https://localhost:8089 admin Cisco123

# View dashboards
# http://<host>:8000/app/search/cisco_mdt_overview
# http://<host>:8000/app/search/cisco_mdt_infrastructure
# http://<host>:8000/app/search/cisco_mdt_network
# http://<host>:8000/app/search/cisco_mdt_routing
# http://<host>:8000/app/search/cisco_mdt_power
# http://<host>:8000/app/search/cisco_mdt_security
# http://<host>:8000/app/search/cisco_mdt_telemetry
```

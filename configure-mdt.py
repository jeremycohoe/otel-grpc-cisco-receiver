#!/usr/bin/env python3
"""
Push MDT gRPC dial-out subscriptions to Cisco IOS XE switches.

Configures 48 YANG operational model subscriptions (IDs 1001-1048 + 1141)
with kvGPB encoding, pointing to the OTEL collector at RECEIVER_IP:RECEIVER_PORT.

Three polling tiers:
  HOT  (30s / 3000 cs)  — CPU, interfaces, process memory, punt/inject
  WARM (60s / 6000 cs)  — environment, platform, routing, STP, stack, PoE, FHRP
  COOL (5m / 30000 cs)  — inventory, slow-changing state, security features

Usage:
    python3 configure-mdt.py
"""

import pexpect
import sys
import time

# ── Switches to configure ──────────────────────────────────────────
SWITCHES = [
    {"host": "10.1.1.5",  "username": "admin", "password": "Cisco123"},
    {"host": "10.1.1.55", "username": "admin", "password": "Cisco123"},
]

# ── Collector target ───────────────────────────────────────────────
RECEIVER_IP = "10.1.1.3"
RECEIVER_PORT = "57500"

# ── Polling intervals (centiseconds) ──────────────────────────────
INTERVAL_HOT = "3000"    # 30 seconds
INTERVAL_WARM = "6000"   # 60 seconds
INTERVAL_COOL = "30000"  # 5 minutes

# ── Subscriptions: (id, xpath, interval) ──────────────────────────
# Organized by polling tier per prd-plan.md §1-§48
SUBSCRIPTIONS = [
    # ── HOT TIER (30s) — CPU, Interfaces, Process Memory, Punt/Inject ──
    (1001, "/process-cpu-ios-xe-oper:cpu-usage/cpu-utilization", INTERVAL_HOT),
    (1003, "/process-memory-ios-xe-oper:memory-usage-processes", INTERVAL_HOT),
    (1007, "/interfaces-ios-xe-oper:interfaces/interface", INTERVAL_HOT),
    (1044, "/switch-dp-punt-inject-oper:switch-dp-punt-inject-oper-data/location/punt-inject-cpuq-brief-stats", INTERVAL_HOT),

    # ── WARM TIER (60s) — Environment, Platform, Routing, STP, Stack, PoE, FHRP ──
    (1002, "/memory-ios-xe-oper:memory-statistics/memory-statistic", INTERVAL_WARM),
    (1004, "/platform-sw-ios-xe-oper:cisco-platform-software/control-processes", INTERVAL_WARM),
    (1005, "/environment-ios-xe-oper:environment-sensors", INTERVAL_WARM),
    (1006, "/poe-ios-xe-oper:poe-oper-data", INTERVAL_WARM),
    (1008, "/stp-ios-xe-oper:stp-details", INTERVAL_WARM),
    (1009, "/stack-ios-xe-oper:stack-oper-data", INTERVAL_WARM),
    (1015, "/platform-ios-xe-oper:components", INTERVAL_WARM),
    (1024, "/bgp-ios-xe-oper:bgp-state-data", INTERVAL_WARM),
    (1025, "/ospf-ios-xe-oper:ospf-oper-data", INTERVAL_WARM),
    (1033, "/ntp-ios-xe-oper:ntp-oper-data/ntp-status-info", INTERVAL_WARM),
    (1034, "/bfd-ios-xe-oper:bfd-state/sessions", INTERVAL_WARM),
    (1035, "/hsrp-ios-xe-oper:hsrp-oper-data/hsrp-group-info", INTERVAL_WARM),
    (1036, "/vrrp-ios-xe-oper:vrrp-oper-data/vrrp-oper-state", INTERVAL_WARM),
    (1037, "/flow-monitor-ios-xe-oper:flow-monitors/flow-monitor", INTERVAL_WARM),
    (1038, "/ip-sla-ios-xe-oper:ip-sla-stats/sla-oper-entry", INTERVAL_WARM),
    (1045, "/poe-ios-xe-oper:poe-oper-data/poe-port", INTERVAL_WARM),
    (1046, "/fib-ios-xe-oper:fib-oper-data", INTERVAL_WARM),

    # ── COOL TIER (300s) — Inventory, slow-changing state, security ──
    (1010, "/vlan-ios-xe-oper:vlans", INTERVAL_COOL),
    (1011, "/matm-ios-xe-oper:matm-oper-data", INTERVAL_COOL),
    (1012, "/arp-ios-xe-oper:arp-data", INTERVAL_COOL),
    (1013, "/lldp-ios-xe-oper:lldp-entries", INTERVAL_COOL),
    (1014, "/cdp-ios-xe-oper:cdp-neighbor-details", INTERVAL_COOL),
    (1016, "/device-hardware-xe-oper:device-hardware-data/device-hardware", INTERVAL_COOL),
    (1017, "/switchport-ios-xe-oper:switchport-oper-data", INTERVAL_COOL),
    (1018, "/xcvr-ios-xe-oper:transceiver-oper-data", INTERVAL_COOL),
    (1019, "/udld-ios-xe-oper:udld-oper-data", INTERVAL_COOL),
    (1020, "/identity-ios-xe-oper:identity-oper-data", INTERVAL_COOL),
    (1021, "/tcam-ios-xe-oper:tcam-details", INTERVAL_COOL),
    (1022, "/mdt-oper-v2:mdt-oper-v2-data", INTERVAL_COOL),
    (1023, "/install-ios-xe-oper:install-oper-data", INTERVAL_COOL),
    (1026, "/rib-ios-xe-oper:rib-oper-data", INTERVAL_COOL),
    (1027, "/dhcp-ios-xe-oper:dhcp-oper-data", INTERVAL_COOL),
    (1028, "/ha-ios-xe-oper:ha-oper-data", INTERVAL_COOL),
    (1029, "/linecard-ios-xe-oper:linecard-oper-data", INTERVAL_COOL),
    (1030, "/trustsec-ios-xe-oper:trustsec-state", INTERVAL_COOL),
    (1031, "/interfaces-ios-xe-oper:interfaces/interface/lag-aggregate-state", INTERVAL_COOL),
    (1032, "/acl-ios-xe-oper:access-lists/access-list", INTERVAL_COOL),
    (1039, "/aaa-ios-xe-oper:aaa-data/aaa-radius-stats", INTERVAL_COOL),
    (1040, "/psecure-ios-xe-oper:psecure-oper-data/psecure-state", INTERVAL_COOL),
    (1041, "/macsec-ios-xe-oper:macsec-oper-data/macsec-statistics", INTERVAL_COOL),
    (1141, "/mka-ios-xe-oper:mka-oper-data/mka-statistics", INTERVAL_COOL),
    (1042, "/vrf-ios-xe-oper:vrf-oper-data/vrf-entry", INTERVAL_COOL),
    (1043, "/dp-resources-oper:switch-dp-resources-oper-data/location/dp-feature-resource", INTERVAL_COOL),
    (1047, "/eigrp-ios-xe-oper:eigrp-oper-data/eigrp-instance", INTERVAL_COOL),
    (1048, "/isis-ios-xe-oper:isis-oper-data/isis-instance", INTERVAL_COOL),
]


def configure_switch(host, username, password):
    print(f"\n{'='*60}")
    print(f"Configuring {host}...")
    print(f"{'='*60}")

    child = pexpect.spawn(
        f"ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {username}@{host}",
        timeout=30, encoding="utf-8",
    )

    index = child.expect(["[Pp]assword:", pexpect.TIMEOUT, pexpect.EOF])
    if index != 0:
        print(f"ERROR: Could not connect to {host}")
        return False

    child.sendline(password)
    index = child.expect(["#", ">", pexpect.TIMEOUT])
    if index == 2:
        print(f"ERROR: Login failed for {host}")
        return False

    # Handle user-exec mode
    if index == 1:
        child.sendline("enable")
        idx = child.expect(["[Pp]assword:", "#"])
        if idx == 0:
            child.sendline(password)
            child.expect("#")

    print(f"Connected to {host}")

    # Disable paging so show commands don't pause
    child.sendline("terminal length 0")
    child.expect("#")

    # Enter config mode
    child.sendline("configure terminal")
    child.expect(r"\(config\)#")

    for sub_id, xpath, interval in SUBSCRIPTIONS:
        print(f"  Sub {sub_id}: {xpath} (interval={interval}cs)")
        child.sendline(f"telemetry ietf subscription {sub_id}")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline("encoding encode-kvgpb")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline(f"filter xpath {xpath}")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline("stream yang-push")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline(f"update-policy periodic {interval}")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline(f"receiver ip address {RECEIVER_IP} {RECEIVER_PORT} protocol grpc-tcp")
        child.expect(r"\(config-mdt-subs\)#")

    # Exit config mode
    child.sendline("end")
    child.expect("#")

    # Save running config to startup
    print("  Saving configuration...")
    child.sendline("write memory")
    child.expect(r"\[OK\]", timeout=60)
    child.expect("#", timeout=10)

    # Show subscription summary
    child.sendline("show telemetry ietf subscription all")
    child.expect("#", timeout=30)
    output = child.before
    active = sum(1 for line in output.split("\n") if line.strip() and line.strip()[0].isdigit())
    print(f"  Total active subscriptions: {active}")

    child.sendline("exit")
    child.close()
    print(f"Done with {host}")
    return True


def main():
    print("MDT Telemetry Subscription Configurator")
    print(f"Receiver target: {RECEIVER_IP}:{RECEIVER_PORT}")
    print(f"Subscriptions: {len(SUBSCRIPTIONS)} total")
    print(f"  HOT  (30s):  {sum(1 for _, _, i in SUBSCRIPTIONS if i == INTERVAL_HOT)}")
    print(f"  WARM (60s):  {sum(1 for _, _, i in SUBSCRIPTIONS if i == INTERVAL_WARM)}")
    print(f"  COOL (300s): {sum(1 for _, _, i in SUBSCRIPTIONS if i == INTERVAL_COOL)}")
    print(f"Switches: {', '.join(s['host'] for s in SWITCHES)}")

    results = {}
    for switch in SWITCHES:
        results[switch["host"]] = configure_switch(
            switch["host"], switch["username"], switch["password"]
        )

    print(f"\n{'='*60}")
    print("Summary:")
    for host, success in results.items():
        status = "SUCCESS" if success else "FAILED"
        print(f"  {host}: {status}")
    print(f"{'='*60}")

    if not all(results.values()):
        sys.exit(1)


if __name__ == "__main__":
    main()

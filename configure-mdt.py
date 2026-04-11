#!/usr/bin/env python3
"""
Push MDT gRPC dial-out subscriptions to Cisco IOS XE switches.

Configures 21 YANG operational model subscriptions (IDs 20001-20021)
with kvGPB encoding, pointing to the OTEL collector at RECEIVER_IP:RECEIVER_PORT.

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

# ── Subscription parameters ───────────────────────────────────────
INTERVAL = "30000"  # centiseconds (30000 = 5 minutes)
START_ID = 20001

SUBSCRIPTIONS = [
    (START_ID,      "/arp-ios-xe-oper:arp-data"),
    (START_ID + 1,  "/cdp-ios-xe-oper:cdp-neighbor-details"),
    (START_ID + 2,  "/environment-ios-xe-oper:environment-sensors"),
    (START_ID + 3,  "/interfaces-ios-xe-oper:interfaces/interface"),
    (START_ID + 4,  "/lldp-ios-xe-oper:lldp-entries"),
    (START_ID + 5,  "/matm-ios-xe-oper:matm-oper-data"),
    (START_ID + 6,  "/mdt-oper:mdt-oper-data/mdt-subscriptions"),
    (START_ID + 7,  "/platform-ios-xe-oper:components"),
    (START_ID + 8,  "/process-cpu-ios-xe-oper:cpu-usage/cpu-utilization"),
    (START_ID + 9,  "/process-memory-ios-xe-oper:memory-usage-processes"),
    (START_ID + 10, "/platform-sw-ios-xe-oper:cisco-platform-software/control-processes"),
    (START_ID + 11, "/poe-ios-xe-oper:poe-oper-data"),
    (START_ID + 12, "/stp-ios-xe-oper:stp-details"),
    (START_ID + 13, "/stack-ios-xe-oper:stack-oper-data"),
    (START_ID + 14, "/identity-ios-xe-oper:identity-oper-data"),
    (START_ID + 15, "/switchport-ios-xe-oper:switchport-oper-data"),
    (START_ID + 16, "/udld-ios-xe-oper:udld-oper-data"),
    (START_ID + 17, "/xcvr-ios-xe-oper:transceiver-oper-data"),
    (START_ID + 18, "/vlan-ios-xe-oper:vlans"),
    (START_ID + 19, "/install-ios-xe-oper:install-oper-data"),
    (START_ID + 20, "/device-hardware-xe-oper:device-hardware-data/device-hardware"),
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

    for sub_id, xpath in SUBSCRIPTIONS:
        print(f"  Sub {sub_id}: {xpath}")
        child.sendline(f"telemetry ietf subscription {sub_id}")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline("encoding encode-kvgpb")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline(f"filter xpath {xpath}")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline("stream yang-push")
        child.expect(r"\(config-mdt-subs\)#")

        child.sendline(f"update-policy periodic {INTERVAL}")
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
    print(f"Subscription IDs: {START_ID}-{START_ID + len(SUBSCRIPTIONS) - 1}")
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

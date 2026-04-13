#!/usr/bin/env python3
"""
Parse telemetry-capture JSONL files into human-readable formats.

Reads  ./telemetry-capture/<sub_id>/metrics.jsonl  (raw OTEL JSON Lines)
Writes per-subscription:
  README.md            Markdown summary with metrics table, entity keys, sample data
  metrics-pretty.json  Indented JSON (one object per line → pretty-printed array)
  flat.jsonl           One line per data point: metric, value, timestamp, attributes
  data.txt             Plain-text table grouped by metric name — easy terminal review

Options:
  --rename     Rename folders from numeric IDs to <SubID>-<xpath-slug>
  --collect    Copy all data.txt files into _all-data-txt/ for consolidated review
  --dir PATH   Capture directory (default: ./telemetry-capture)

Usage:
    python3 parse-capture.py                    # generate all output files
    python3 parse-capture.py --rename           # also rename folders
    python3 parse-capture.py --rename --collect  # rename + collect data.txt
"""

import argparse
import json
import os
import re
import shutil
import sys
from collections import defaultdict
from datetime import datetime, timezone

CAPTURE_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "telemetry-capture")  # default only

# Map subscription IDs to human-friendly names (from prd-plan.md)
SUB_NAMES = {
    "1001": "CPU Utilization",
    "1002": "Memory Statistics",
    "1003": "Process Memory",
    "1004": "Platform Software (DRAM)",
    "1005": "Environment Sensors",
    "1006": "PoE Oper Data",
    "1007": "Interface Statistics",
    "1008": "STP Details",
    "1009": "Stack Oper Data",
    "1010": "VLANs",
    "1011": "MAC Address Table (MATM)",
    "1012": "ARP Table",
    "1013": "LLDP Entries",
    "1014": "CDP Neighbors",
    "1015": "Platform Components",
    "1016": "Device Hardware",
    "1017": "Switchport Oper Data",
    "1018": "Transceiver Oper Data",
    "1019": "UDLD Oper Data",
    "1020": "Identity (802.1X)",
    "1021": "TCAM Details",
    "1022": "MDT Oper v2 (Subscription Health)",
    "1023": "Install Oper Data",
    "1024": "BGP State",
    "1025": "OSPF Oper Data",
    "1026": "RIB / Routing Table",
    "1027": "DHCP Oper Data",
    "1028": "HA Oper Data",
    "1029": "Linecard Oper Data",
    "1030": "TrustSec State",
    "1031": "LAG Aggregate State",
    "1032": "ACL Statistics",
    "1033": "NTP Status",
    "1034": "BFD Sessions",
    "1035": "HSRP Group Info",
    "1036": "VRRP Oper State",
    "1037": "NetFlow / Flow Monitor",
    "1038": "IP SLA Stats",
    "1039": "AAA / RADIUS Stats",
    "1040": "Port Security",
    "1041": "MACsec Statistics",
    "1042": "VRF Entries",
    "1043": "Dataplane Resources",
    "1044": "Punt/Inject CPU Queue Stats",
    "1045": "PoE Port Health",
    "1046": "CEF / FIB State",
    "1047": "EIGRP Instances",
    "1048": "IS-IS Instances",
    "1141": "MKA Statistics",
}

INTERVAL_LABELS = {
    "3000": "30s (HOT)",
    "6000": "60s (WARM)",
    "30000": "5min (COOL)",
}


def get_attr_value(attr):
    """Extract the scalar value from an OTEL attribute value object."""
    for key in ("stringValue", "intValue", "doubleValue", "boolValue"):
        if key in attr.get("value", {}):
            return attr["value"][key]
    return ""


def parse_subscription(sub_dir):
    """Parse a single subscription's JSONL file and return structured data."""
    jsonl_path = os.path.join(sub_dir, "metrics.jsonl")
    if not os.path.isfile(jsonl_path) or os.path.getsize(jsonl_path) == 0:
        return None

    sub_id = extract_sub_id(os.path.basename(sub_dir))
    resource_attrs = {}
    metrics_catalog = {}       # metric_name -> {desc, unit, type, is_monotonic}
    sample_values = {}         # metric_name -> list of (value, timestamp, entity_keys)
    entity_examples = {}       # metric_name -> set of entity key combos seen
    all_attr_keys = set()      # all attribute keys seen across all data points
    record_count = 0
    max_samples_per_metric = 5

    with open(jsonl_path, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            record_count += 1
            try:
                record = json.loads(line)
            except json.JSONDecodeError:
                continue

            for rm in record.get("resourceMetrics", []):
                # Capture resource attributes from first record
                if not resource_attrs:
                    for attr in rm.get("resource", {}).get("attributes", []):
                        resource_attrs[attr["key"]] = get_attr_value(attr)

                for sm in rm.get("scopeMetrics", []):
                    for metric in sm.get("metrics", []):
                        name = metric.get("name", "?")

                        # Determine metric type
                        if "gauge" in metric:
                            metric_type = "Gauge"
                            container = metric["gauge"]
                            is_monotonic = False
                        elif "sum" in metric:
                            is_monotonic = metric["sum"].get("isMonotonic", False)
                            metric_type = "Counter" if is_monotonic else "Sum"
                            container = metric["sum"]
                        else:
                            metric_type = "?"
                            container = {}
                            is_monotonic = False

                        # Register in catalog
                        if name not in metrics_catalog:
                            metrics_catalog[name] = {
                                "description": metric.get("description", ""),
                                "unit": metric.get("unit", ""),
                                "type": metric_type,
                                "is_monotonic": is_monotonic,
                            }

                        # Process data points
                        for dp in container.get("dataPoints", []):
                            value = dp.get("asDouble", dp.get("asInt", ""))

                            # Parse timestamp
                            ts_nano = int(dp.get("timeUnixNano", 0))
                            if ts_nano > 0:
                                ts = datetime.fromtimestamp(
                                    ts_nano / 1e9, tz=timezone.utc
                                ).strftime("%Y-%m-%d %H:%M:%S UTC")
                            else:
                                ts = ""

                            # Separate entity keys from YANG metadata
                            entity_keys = {}
                            yang_attrs = {}
                            for attr in dp.get("attributes", []):
                                k = attr["key"]
                                v = get_attr_value(attr)
                                all_attr_keys.add(k)
                                if k.startswith("yang."):
                                    yang_attrs[k] = v
                                elif k == "encoding_path":
                                    pass  # skip, already in resource
                                else:
                                    entity_keys[k] = v

                            # Collect samples (deduplicated by entity key combo)
                            entity_key_tuple = tuple(sorted(entity_keys.items()))
                            if name not in entity_examples:
                                entity_examples[name] = set()

                            if name not in sample_values:
                                sample_values[name] = []

                            if entity_key_tuple not in entity_examples[name]:
                                entity_examples[name].add(entity_key_tuple)
                                if len(sample_values[name]) < max_samples_per_metric:
                                    sample_values[name].append({
                                        "value": value,
                                        "timestamp": ts,
                                        "entity_keys": entity_keys,
                                        "yang": yang_attrs,
                                    })

    if not metrics_catalog:
        return None

    return {
        "sub_id": sub_id,
        "resource_attrs": resource_attrs,
        "metrics_catalog": metrics_catalog,
        "sample_values": sample_values,
        "all_attr_keys": sorted(all_attr_keys),
        "record_count": record_count,
        "file_size": os.path.getsize(jsonl_path),
    }


def format_value(val):
    """Format a numeric value for display."""
    if isinstance(val, float):
        if val == int(val) and abs(val) < 1e15:
            return f"{int(val):,}"
        return f"{val:,.2f}"
    if isinstance(val, int):
        return f"{val:,}"
    return str(val)


def write_readme(sub_dir, data):
    """Write a human-readable README.md for one subscription."""
    sub_id = data["sub_id"]
    ra = data["resource_attrs"]
    catalog = data["metrics_catalog"]
    samples = data["sample_values"]

    friendly_name = SUB_NAMES.get(sub_id, "Unknown")
    encoding_path = ra.get("cisco.encoding_path", "")
    yang_module = ra.get("cisco.yang_module", "")
    node_id = ra.get("cisco.node_id", "")

    lines = []
    lines.append(f"# Sub {sub_id}: {friendly_name}\n")
    lines.append(f"**Node:** {node_id}  ")
    lines.append(f"**Encoding Path:** `{encoding_path}`  ")
    lines.append(f"**YANG Module:** `{yang_module}`  ")
    lines.append(f"**Records Captured:** {data['record_count']}  ")
    lines.append(f"**File Size:** {data['file_size']:,} bytes  ")
    lines.append("")

    # ── Metrics Summary Table ──
    lines.append("## Metrics Summary\n")
    lines.append("| # | Metric Name | Type | Unit | Description |")
    lines.append("|---|------------|------|------|-------------|")
    for i, (name, info) in enumerate(sorted(catalog.items()), 1):
        short_name = name.replace("cisco.content.", "").replace("cisco.keys.", "🔑 ")
        lines.append(
            f"| {i} | `{short_name}` | {info['type']} | {info['unit']} | {info['description']} |"
        )
    lines.append("")

    # ── Identify entity keys (non-yang, non-encoding_path attrs that vary) ──
    entity_key_names = set()
    for name, sample_list in samples.items():
        for s in sample_list:
            entity_key_names.update(s["entity_keys"].keys())

    if entity_key_names:
        lines.append("## Entity Keys (Group-By Dimensions)\n")
        lines.append("These attributes identify the **entity** each metric belongs to (interface, process, sensor, etc.).\n")
        lines.append("| Key | Example Values |")
        lines.append("|-----|---------------|")
        # Collect example values per key
        key_examples = defaultdict(set)
        for name, sample_list in samples.items():
            for s in sample_list:
                for k, v in s["entity_keys"].items():
                    if len(key_examples[k]) < 5:
                        key_examples[k].add(str(v))
        for k in sorted(entity_key_names):
            examples = ", ".join(f"`{v}`" for v in sorted(key_examples[k])[:5])
            lines.append(f"| `{k}` | {examples} |")
        lines.append("")

    # ── Sample Values ──
    lines.append("## Sample Data\n")
    lines.append("One sample per unique entity, up to 5 per metric.\n")

    # Group metrics by whether they are keys or content
    key_metrics = sorted([n for n in catalog if n.startswith("cisco.keys.")])
    content_metrics = sorted([n for n in catalog if n.startswith("cisco.content.")])
    other_metrics = sorted([n for n in catalog if not n.startswith("cisco.keys.") and not n.startswith("cisco.content.")])

    if content_metrics:
        lines.append("### Content Metrics\n")
        for name in content_metrics:
            short = name.replace("cisco.content.", "")
            info = catalog[name]
            lines.append(f"#### `{short}` ({info['type']}, {info['unit'] or 'unitless'})\n")
            lines.append(f"> {info['description']}\n" if info['description'] else "")

            sample_list = samples.get(name, [])
            if not sample_list:
                lines.append("_No samples captured._\n")
                continue

            # Build sample table
            has_keys = any(s["entity_keys"] for s in sample_list)
            if has_keys:
                # Find all entity key columns
                ek_cols = sorted({k for s in sample_list for k in s["entity_keys"]})
                header = "| " + " | ".join(f"`{c}`" for c in ek_cols) + " | Value | Timestamp |"
                sep = "|" + "|".join("---" for _ in ek_cols) + "|------|-----------|"
                lines.append(header)
                lines.append(sep)
                for s in sample_list:
                    ek_vals = " | ".join(str(s["entity_keys"].get(c, "")) for c in ek_cols)
                    lines.append(f"| {ek_vals} | {format_value(s['value'])} | {s['timestamp']} |")
            else:
                lines.append("| Value | Timestamp |")
                lines.append("|-------|-----------|")
                for s in sample_list:
                    lines.append(f"| {format_value(s['value'])} | {s['timestamp']} |")
            lines.append("")

    if key_metrics:
        lines.append("### Key Metrics (Entity Identifiers)\n")
        lines.append("These are YANG list key fields emitted as separate metrics.\n")
        for name in key_metrics:
            short = name.replace("cisco.keys.", "")
            info = catalog[name]
            sample_list = samples.get(name, [])
            if not sample_list:
                continue
            has_keys = any(s["entity_keys"] for s in sample_list)
            if has_keys:
                ek_cols = sorted({k for s in sample_list for k in s["entity_keys"]})
                lines.append(f"**`{short}`** — {info['description']}\n")
                for s in sample_list[:3]:
                    ek_str = ", ".join(f"{k}=`{v}`" for k, v in sorted(s["entity_keys"].items()))
                    lines.append(f"- {ek_str} → `{s['value']}`")
                lines.append("")

    if other_metrics:
        lines.append("### Other Metrics\n")
        for name in other_metrics:
            info = catalog[name]
            sample_list = samples.get(name, [])
            lines.append(f"#### `{name}` ({info['type']}, {info['unit'] or 'unitless'})\n")
            if sample_list:
                for s in sample_list[:3]:
                    lines.append(f"- Value: `{format_value(s['value'])}`  {s['timestamp']}")
            lines.append("")

    # ── YANG Metadata (show once from first available sample) ──
    first_yang = {}
    for name, sample_list in samples.items():
        for s in sample_list:
            if s.get("yang"):
                first_yang = s["yang"]
                break
        if first_yang:
            break

    if first_yang:
        lines.append("## YANG Metadata\n")
        lines.append("Attributes attached to every data point for schema-aware processing.\n")
        lines.append("| Attribute | Value |")
        lines.append("|-----------|-------|")
        for k, v in sorted(first_yang.items()):
            lines.append(f"| `{k}` | `{v}` |")
        lines.append("")

    # ── All Attribute Keys ──
    lines.append("## All Data Point Attributes\n")
    lines.append("Complete list of attribute keys seen on data points in this subscription.\n")
    for k in data["all_attr_keys"]:
        lines.append(f"- `{k}`")
    lines.append("")

    readme_path = os.path.join(sub_dir, "README.md")
    with open(readme_path, "w") as f:
        f.write("\n".join(lines))

    return readme_path


def main():
    parser = argparse.ArgumentParser(
        description="Parse OTEL telemetry-capture JSONL into human-readable formats."
    )
    parser.add_argument(
        "--dir", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "telemetry-capture"),
        help="Capture directory (default: ./telemetry-capture)"
    )
    parser.add_argument(
        "--rename", action="store_true",
        help="Rename numeric folders to <SubID>-<xpath-slug> descriptive names"
    )
    parser.add_argument(
        "--collect", action="store_true",
        help="Copy all data.txt files into _all-data-txt/ for easy review"
    )
    args = parser.parse_args()
    capture_dir = args.dir

    if not os.path.isdir(capture_dir):
        print(f"ERROR: Capture directory not found: {capture_dir}")
        sys.exit(1)

    # Find subscription directories — match purely numeric names
    # (renamed folders are skipped since they contain hyphens; use numeric originals)
    sub_dirs = sorted(
        [os.path.join(capture_dir, d) for d in os.listdir(capture_dir)
         if os.path.isdir(os.path.join(capture_dir, d)) and re.match(r"^\d+$", d)],
        key=lambda p: int(os.path.basename(p))
    )

    # Also include already-renamed folders (SubID-name format)
    if not sub_dirs:
        sub_dirs = sorted(
            [os.path.join(capture_dir, d) for d in os.listdir(capture_dir)
             if os.path.isdir(os.path.join(capture_dir, d)) and re.match(r"^\d+-.+", d)],
            key=lambda p: int(os.path.basename(p).split("-")[0])
        )

    print(f"Found {len(sub_dirs)} subscription directories in {capture_dir}")
    parsed = 0
    skipped = 0
    results = []  # (sub_dir, data) for post-processing

    for sub_dir in sub_dirs:
        sub_id = extract_sub_id(os.path.basename(sub_dir))
        data = parse_subscription(sub_dir)
        if data is None:
            skipped += 1
            continue

        # Generate all output files
        write_readme(sub_dir, data)
        write_pretty_json(sub_dir)
        write_flat_jsonl(sub_dir, data)
        write_data_txt(sub_dir, data)

        metric_count = len(data["metrics_catalog"])
        friendly = SUB_NAMES.get(sub_id, "")
        print(f"  {sub_id}: {friendly:30s} — {metric_count:3d} metrics, {data['record_count']:3d} records")
        parsed += 1
        results.append((sub_dir, data))

    print(f"\nGenerated: {parsed} subscriptions, {skipped} skipped (empty)")

    # Rename folders to descriptive names if requested
    if args.rename:
        print("\nRenaming folders to descriptive names...")
        renamed_dirs = rename_folders(capture_dir, results)
        # Update results with new paths for --collect
        results = [(new_dir, data) for (_, data), new_dir in zip(results, renamed_dirs)]

    # Collect all data.txt into consolidated folder
    if args.collect:
        collect_data_txt(capture_dir, results)


def extract_sub_id(folder_name):
    """Extract the numeric subscription ID from a folder name like '1001' or '1001-cpu-usage'."""
    match = re.match(r"^(\d+)", folder_name)
    return match.group(1) if match else folder_name


def xpath_to_slug(encoding_path):
    """Convert an encoding path like '/interfaces-ios-xe-oper:interfaces/interface' to a short slug."""
    # Strip the module prefix (everything before and including the colon)
    path = re.sub(r"/[a-zA-Z0-9_-]+:", "/", encoding_path)
    # Remove leading slash
    path = path.lstrip("/")
    # Replace slashes with underscores, collapse special chars
    slug = re.sub(r"[^a-zA-Z0-9_-]", "_", path)
    slug = re.sub(r"_+", "_", slug).strip("_").lower()
    # Truncate to reasonable length
    return slug[:60] if slug else "unknown"


def rename_folders(capture_dir, results):
    """Rename subscription folders from numeric to SubID-XPathSlug format."""
    renamed = []
    for sub_dir, data in results:
        sub_id = extract_sub_id(os.path.basename(sub_dir))
        encoding_path = data["resource_attrs"].get("cisco.encoding_path", "")
        slug = xpath_to_slug(encoding_path)
        new_name = f"{sub_id}-{slug}"
        new_path = os.path.join(capture_dir, new_name)

        if sub_dir != new_path and not os.path.exists(new_path):
            os.rename(sub_dir, new_path)
            print(f"  {os.path.basename(sub_dir)} → {new_name}")
            renamed.append(new_path)
        else:
            renamed.append(sub_dir)
    return renamed


def collect_data_txt(capture_dir, results):
    """Copy all data.txt files into a single _all-data-txt/ directory."""
    out_dir = os.path.join(capture_dir, "_all-data-txt")
    if os.path.exists(out_dir):
        shutil.rmtree(out_dir)
    os.makedirs(out_dir)

    count = 0
    for sub_dir, _ in results:
        data_txt = os.path.join(sub_dir, "data.txt")
        if os.path.isfile(data_txt):
            folder_name = os.path.basename(sub_dir)
            shutil.copy2(data_txt, os.path.join(out_dir, f"{folder_name}.txt"))
            count += 1

    print(f"\nCollected {count} data.txt files into {out_dir}/")


def write_pretty_json(sub_dir):
    """Write an indented, easy-to-read JSON file from the raw JSONL."""
    jsonl_path = os.path.join(sub_dir, "metrics.jsonl")
    out_path = os.path.join(sub_dir, "metrics-pretty.json")

    records = []
    with open(jsonl_path, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                records.append(json.loads(line))
            except json.JSONDecodeError:
                continue

    with open(out_path, "w") as f:
        json.dump(records, f, indent=2)


def write_flat_jsonl(sub_dir, data):
    """Write one line per data point with metric name, value, timestamp, and all attributes."""
    jsonl_path = os.path.join(sub_dir, "metrics.jsonl")
    out_path = os.path.join(sub_dir, "flat.jsonl")

    flat_lines = []
    with open(jsonl_path, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError:
                continue

            for rm in record.get("resourceMetrics", []):
                resource_attrs = {}
                for attr in rm.get("resource", {}).get("attributes", []):
                    resource_attrs[attr["key"]] = get_attr_value(attr)

                for sm in rm.get("scopeMetrics", []):
                    for metric in sm.get("metrics", []):
                        name = metric.get("name", "?")
                        container = metric.get("gauge", metric.get("sum", {}))

                        for dp in container.get("dataPoints", []):
                            value = dp.get("asDouble", dp.get("asInt", ""))
                            ts_nano = int(dp.get("timeUnixNano", 0))
                            timestamp = ""
                            if ts_nano > 0:
                                timestamp = datetime.fromtimestamp(
                                    ts_nano / 1e9, tz=timezone.utc
                                ).strftime("%Y-%m-%d %H:%M:%S UTC")

                            dp_attrs = {}
                            for attr in dp.get("attributes", []):
                                dp_attrs[attr["key"]] = get_attr_value(attr)

                            flat_lines.append(json.dumps({
                                "metric": name,
                                "value": value,
                                "timestamp": timestamp,
                                "attributes": dp_attrs,
                                "node": resource_attrs.get("cisco.node_id", ""),
                            }))

    with open(out_path, "w") as f:
        f.write("\n".join(flat_lines))
        if flat_lines:
            f.write("\n")


def write_data_txt(sub_dir, data):
    """Write a plain-text table grouped by metric — the easiest format for quick terminal review."""
    jsonl_path = os.path.join(sub_dir, "metrics.jsonl")
    out_path = os.path.join(sub_dir, "data.txt")
    sub_id = extract_sub_id(os.path.basename(sub_dir))
    friendly = SUB_NAMES.get(sub_id, "Subscription " + sub_id)

    # Collect all data points grouped by metric name
    metric_data = defaultdict(list)  # name -> list of (value, attr_summary)
    total_points = 0
    message_count = 0

    with open(jsonl_path, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError:
                continue
            message_count += 1

            for rm in record.get("resourceMetrics", []):
                for sm in rm.get("scopeMetrics", []):
                    for metric in sm.get("metrics", []):
                        name = metric.get("name", "?")
                        container = metric.get("gauge", metric.get("sum", {}))

                        for dp in container.get("dataPoints", []):
                            value = dp.get("asDouble", dp.get("asInt", ""))
                            total_points += 1

                            attrs = {}
                            for attr in dp.get("attributes", []):
                                k = attr["key"]
                                if not k.startswith("yang.") and k != "encoding_path":
                                    attrs[k] = get_attr_value(attr)

                            metric_data[name].append((value, attrs))

    # Determine metric type from catalog
    catalog = data["metrics_catalog"]

    lines = []
    separator = "=" * 90
    lines.append(separator)
    lines.append(f"  Sub {sub_id}: {friendly}")
    lines.append(f"  Folder: {os.path.basename(sub_dir)}")
    lines.append(f"  {total_points} data points from {message_count} telemetry messages")
    lines.append(separator)

    for name in sorted(metric_data.keys()):
        points = metric_data[name]
        mtype = catalog.get(name, {}).get("type", "?")
        lines.append(f"\n  {name}  ({mtype.lower()}, {len(points)})")
        lines.append("  " + "-" * 70)

        for value, attrs in points:
            formatted = format_value(value)
            if attrs:
                # Check if value looks like a string-encoded info metric
                attr_parts = []
                for k, v in sorted(attrs.items()):
                    attr_parts.append(f"{k}={v}")
                attr_str = ", ".join(attr_parts)
                lines.append(f"  {formatted:>15}  |  {attr_str}")
            else:
                lines.append(f"  {formatted:>15}  |  (global)")

    lines.append("")

    with open(out_path, "w") as f:
        f.write("\n".join(lines))


if __name__ == "__main__":
    main()

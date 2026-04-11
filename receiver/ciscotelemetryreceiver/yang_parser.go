package ciscotelemetryreceiver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Helper function to create int64 pointers
func int64Ptr(v int64) *int64 {
	return &v
}

// YANGDataType represents the YANG data type information
type YANGDataType struct {
	Type        string           `json:"type"`        // uint8, uint16, uint32, uint64, int8, int16, int32, int64, string, boolean, decimal64, etc.
	Units       string           `json:"units"`       // units like "percent", "seconds", "bytes", "packets"
	Range       *YANGRange       `json:"range"`       // min/max values if applicable
	Description string           `json:"description"` // field description
	Enumeration map[string]int64 `json:"enumeration"` // for enum types: name -> value
}

// YANGRange represents min/max constraints for numeric types
type YANGRange struct {
	Min *int64 `json:"min"`
	Max *int64 `json:"max"`
}

// YANGModule represents a parsed YANG module with its key information
type YANGModule struct {
	Name        string                   `json:"name"`
	Namespace   string                   `json:"namespace"`
	Prefix      string                   `json:"prefix"`
	KeyedLeafs  map[string]string        `json:"keyed_leafs"` // path -> key field name
	ListKeys    map[string][]string      `json:"list_keys"`   // list path -> key fields
	DataTypes   map[string]*YANGDataType `json:"data_types"`  // field path -> data type info
	Description string                   `json:"description"`
}

// YANGParser handles parsing of YANG modules to identify keyed elements
type YANGParser struct {
	modules map[string]*YANGModule
}

// NewYANGParser creates a new YANG parser instance
func NewYANGParser() *YANGParser {
	return &YANGParser{
		modules: make(map[string]*YANGModule),
	}
}

// LoadBuiltinModules loads pre-analyzed YANG modules for all subscribed Cisco IOS XE
// operational models. Key fields, data types, and semantic classifications are aligned
// to the RFC 6020/7950 YANG model definitions published by Cisco.
func (p *YANGParser) LoadBuiltinModules() {

	// ── Cisco-IOS-XE-interfaces-oper ────────────────────────────────────────
	// RFC ref: list /interfaces/interface { key "name"; }
	p.modules["Cisco-IOS-XE-interfaces-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-interfaces-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-interfaces-oper",
		Prefix:    "interfaces-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/interfaces/interface": "name",
		},
		ListKeys: map[string][]string{
			"/interfaces/interface": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/interfaces/interface/name":           {Type: "string", Description: "Interface name key"},
			"/interfaces/interface/interface-type":  {Type: "identityref", Description: "Interface type"},
			"/interfaces/interface/if-index":        {Type: "int32", Description: "Interface ifIndex"},
			"/interfaces/interface/admin-status":    {Type: "enumeration", Description: "Administrative status"},
			"/interfaces/interface/oper-status":     {Type: "enumeration", Description: "Operational status"},
			"/interfaces/interface/speed":           {Type: "uint64", Units: "bits-per-second", Description: "Interface speed"},
			"/interfaces/interface/mtu":             {Type: "uint32", Units: "bytes", Description: "Maximum transmission unit"},
			"/interfaces/interface/phys-address":    {Type: "string", Description: "MAC address"},
			"/interfaces/interface/description":     {Type: "string", Description: "Interface description"},

			// statistics container — counters (RFC 7223 aligned)
			"/interfaces/interface/statistics/in-octets":         {Type: "uint64", Units: "bytes", Description: "Total bytes received"},
			"/interfaces/interface/statistics/in-unicast-pkts":   {Type: "uint64", Units: "packets", Description: "Unicast packets received"},
			"/interfaces/interface/statistics/in-broadcast-pkts": {Type: "uint64", Units: "packets", Description: "Broadcast packets received"},
			"/interfaces/interface/statistics/in-multicast-pkts": {Type: "uint64", Units: "packets", Description: "Multicast packets received"},
			"/interfaces/interface/statistics/in-discards":       {Type: "uint32", Units: "packets", Description: "Inbound packets discarded"},
			"/interfaces/interface/statistics/in-errors":         {Type: "uint32", Units: "packets", Description: "Inbound errors"},
			"/interfaces/interface/statistics/in-unknown-protos": {Type: "uint32", Units: "packets", Description: "Unknown protocol packets"},
			"/interfaces/interface/statistics/in-crc-errors":     {Type: "uint32", Units: "packets", Description: "CRC errors"},
			"/interfaces/interface/statistics/out-octets":         {Type: "uint64", Units: "bytes", Description: "Total bytes transmitted"},
			"/interfaces/interface/statistics/out-unicast-pkts":   {Type: "uint64", Units: "packets", Description: "Unicast packets transmitted"},
			"/interfaces/interface/statistics/out-broadcast-pkts": {Type: "uint64", Units: "packets", Description: "Broadcast packets transmitted"},
			"/interfaces/interface/statistics/out-multicast-pkts": {Type: "uint64", Units: "packets", Description: "Multicast packets transmitted"},
			"/interfaces/interface/statistics/out-discards":       {Type: "uint32", Units: "packets", Description: "Outbound discards"},
			"/interfaces/interface/statistics/out-errors":         {Type: "uint32", Units: "packets", Description: "Outbound errors"},
			"/interfaces/interface/statistics/num-flaps":          {Type: "uint32", Units: "count", Description: "Number of interface flaps"},

			// statistics — rates (gauges)
			"/interfaces/interface/statistics/rx-pps":  {Type: "uint32", Units: "packets-per-second", Description: "Receive packet rate"},
			"/interfaces/interface/statistics/rx-kbps": {Type: "uint32", Units: "kilobits-per-second", Description: "Receive bit rate"},
			"/interfaces/interface/statistics/tx-pps":  {Type: "uint32", Units: "packets-per-second", Description: "Transmit packet rate"},
			"/interfaces/interface/statistics/tx-kbps": {Type: "uint32", Units: "kilobits-per-second", Description: "Transmit bit rate"},
		},
		Description: "Cisco IOS XE interface operational data (RFC 7223 aligned)",
	}

	// ── Cisco-IOS-XE-process-cpu-oper ───────────────────────────────────────
	// list /cpu-usage/cpu-utilization/cpu-usage-processes/cpu-usage-process { key "pid name"; }
	// Container /cpu-usage/cpu-utilization is a singleton (no key).
	p.modules["Cisco-IOS-XE-process-cpu-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-process-cpu-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-process-cpu-oper",
		Prefix:    "process-cpu-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/cpu-usage/cpu-utilization/cpu-usage-processes/cpu-usage-process": "name",
		},
		ListKeys: map[string][]string{
			"/cpu-usage/cpu-utilization/cpu-usage-processes/cpu-usage-process": {"pid", "name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/cpu-usage/cpu-utilization/five-seconds":      {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(100)}, Description: "CPU utilization last 5 seconds"},
			"/cpu-usage/cpu-utilization/five-seconds-intr": {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(100)}, Description: "CPU interrupt pct last 5 seconds"},
			"/cpu-usage/cpu-utilization/one-minute":        {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(100)}, Description: "CPU utilization last 1 minute"},
			"/cpu-usage/cpu-utilization/five-minutes":      {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(100)}, Description: "CPU utilization last 5 minutes"},
		},
		Description: "Process CPU utilization (Cisco-IOS-XE-process-cpu-oper)",
	}

	// ── Cisco-IOS-XE-process-memory-oper ────────────────────────────────────
	// list /memory-usage-processes/memory-usage-process { key "pid name"; }
	p.modules["Cisco-IOS-XE-process-memory-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-process-memory-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-process-memory-oper",
		Prefix:    "process-memory-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/memory-usage-processes/memory-usage-process": "name",
		},
		ListKeys: map[string][]string{
			"/memory-usage-processes/memory-usage-process": {"pid", "name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/memory-usage-processes/memory-usage-process/pid":              {Type: "uint32", Description: "Process ID"},
			"/memory-usage-processes/memory-usage-process/name":             {Type: "string", Description: "Process name"},
			"/memory-usage-processes/memory-usage-process/tty":              {Type: "uint16", Description: "TTY number"},
			"/memory-usage-processes/memory-usage-process/allocated-memory": {Type: "uint64", Units: "bytes", Description: "Allocated memory"},
			"/memory-usage-processes/memory-usage-process/freed-memory":     {Type: "uint64", Units: "bytes", Description: "Freed memory"},
			"/memory-usage-processes/memory-usage-process/holding-memory":   {Type: "uint64", Units: "bytes", Description: "Holding memory"},
			"/memory-usage-processes/memory-usage-process/get-buffers":      {Type: "uint32", Units: "count", Description: "Get-buffer count"},
			"/memory-usage-processes/memory-usage-process/ret-buffers":      {Type: "uint32", Units: "count", Description: "Ret-buffer count"},
		},
		Description: "Process memory usage (Cisco-IOS-XE-process-memory-oper)",
	}

	// ── Cisco-IOS-XE-environment-oper ───────────────────────────────────────
	// list /environment-sensors/environment-sensor { key "name location"; }
	p.modules["Cisco-IOS-XE-environment-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-environment-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-environment-oper",
		Prefix:    "environment-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/environment-sensors/environment-sensor": "name",
		},
		ListKeys: map[string][]string{
			"/environment-sensors/environment-sensor": {"name", "location"},
		},
		DataTypes: map[string]*YANGDataType{
			"/environment-sensors/environment-sensor/name":            {Type: "string", Description: "Sensor name key"},
			"/environment-sensors/environment-sensor/location":        {Type: "string", Description: "Sensor location key"},
			"/environment-sensors/environment-sensor/state":           {Type: "enumeration", Description: "Sensor state (Normal, Warning, Critical, etc.)"},
			"/environment-sensors/environment-sensor/current-reading": {Type: "uint32", Description: "Current sensor reading"},
			"/environment-sensors/environment-sensor/sensor-units":    {Type: "enumeration", Description: "Sensor measurement units"},
			"/environment-sensors/environment-sensor/low-critical-threshold":  {Type: "int32", Description: "Low critical threshold value"},
			"/environment-sensors/environment-sensor/low-normal-threshold":    {Type: "int32", Description: "Low normal threshold value"},
			"/environment-sensors/environment-sensor/high-normal-threshold":   {Type: "int32", Description: "High normal threshold value"},
			"/environment-sensors/environment-sensor/high-critical-threshold": {Type: "int32", Description: "High critical threshold value"},
		},
		Description: "Environment sensor data (temp, fan, power) — Cisco-IOS-XE-environment-oper",
	}

	// ── Cisco-IOS-XE-arp-oper ───────────────────────────────────────────────
	// list /arp-data/arp-vrf/arp-oper { key "address"; }
	// list /arp-data/arp-vrf { key "vrf"; }
	p.modules["Cisco-IOS-XE-arp-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-arp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-arp-oper",
		Prefix:    "arp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/arp-data/arp-vrf":          "vrf",
			"/arp-data/arp-vrf/arp-oper": "address",
		},
		ListKeys: map[string][]string{
			"/arp-data/arp-vrf":          {"vrf"},
			"/arp-data/arp-vrf/arp-oper": {"address"},
		},
		DataTypes: map[string]*YANGDataType{
			"/arp-data/arp-vrf/vrf":                        {Type: "string", Description: "VRF name key"},
			"/arp-data/arp-vrf/arp-oper/address":           {Type: "inet:ipv4-address", Description: "IP address key"},
			"/arp-data/arp-vrf/arp-oper/enctype":           {Type: "enumeration", Description: "Encapsulation type"},
			"/arp-data/arp-vrf/arp-oper/interface":         {Type: "string", Description: "Interface name"},
			"/arp-data/arp-vrf/arp-oper/type":              {Type: "enumeration", Description: "ARP entry type (dynamic/static)"},
			"/arp-data/arp-vrf/arp-oper/mode":              {Type: "enumeration", Description: "ARP mode"},
			"/arp-data/arp-vrf/arp-oper/hwtype":            {Type: "enumeration", Description: "Hardware type"},
			"/arp-data/arp-vrf/arp-oper/hardware":          {Type: "string", Description: "Hardware (MAC) address"},
			"/arp-data/arp-vrf/arp-oper/time":              {Type: "yang:date-and-time", Description: "ARP entry age"},
		},
		Description: "ARP operational data (Cisco-IOS-XE-arp-oper)",
	}

	// ── Cisco-IOS-XE-cdp-oper ───────────────────────────────────────────────
	// list /cdp-neighbor-details/cdp-neighbor-detail { key "device-id"; }
	p.modules["Cisco-IOS-XE-cdp-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-cdp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-cdp-oper",
		Prefix:    "cdp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/cdp-neighbor-details/cdp-neighbor-detail": "device-id",
		},
		ListKeys: map[string][]string{
			"/cdp-neighbor-details/cdp-neighbor-detail": {"device-id"},
		},
		DataTypes: map[string]*YANGDataType{
			"/cdp-neighbor-details/cdp-neighbor-detail/device-id":      {Type: "string", Description: "Neighbor device ID key"},
			"/cdp-neighbor-details/cdp-neighbor-detail/device-name":    {Type: "string", Description: "Neighbor device name"},
			"/cdp-neighbor-details/cdp-neighbor-detail/platform-name":  {Type: "string", Description: "Neighbor platform"},
			"/cdp-neighbor-details/cdp-neighbor-detail/port-id":        {Type: "string", Description: "Remote port ID"},
			"/cdp-neighbor-details/cdp-neighbor-detail/local-intf-name": {Type: "string", Description: "Local interface name"},
			"/cdp-neighbor-details/cdp-neighbor-detail/capability":     {Type: "string", Description: "Neighbor capabilities"},
			"/cdp-neighbor-details/cdp-neighbor-detail/ip-address":     {Type: "inet:ipv4-address", Description: "Neighbor IP address"},
			"/cdp-neighbor-details/cdp-neighbor-detail/version":        {Type: "string", Description: "Software version"},
			"/cdp-neighbor-details/cdp-neighbor-detail/duplex":         {Type: "enumeration", Description: "Duplex setting"},
			"/cdp-neighbor-details/cdp-neighbor-detail/native-vlan":    {Type: "uint16", Description: "Native VLAN"},
			"/cdp-neighbor-details/cdp-neighbor-detail/hello-message":  {Type: "string", Description: "Hello message"},
			"/cdp-neighbor-details/cdp-neighbor-detail/ttl":            {Type: "uint16", Units: "seconds", Description: "Time-to-live"},
		},
		Description: "CDP neighbor detail (Cisco-IOS-XE-cdp-oper)",
	}

	// ── Cisco-IOS-XE-lldp-oper ──────────────────────────────────────────────
	// list /lldp-entries/lldp-entry { key "device-id local-interface connecting-interface"; }
	p.modules["Cisco-IOS-XE-lldp-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-lldp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-lldp-oper",
		Prefix:    "lldp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/lldp-entries/lldp-entry": "device-id",
		},
		ListKeys: map[string][]string{
			"/lldp-entries/lldp-entry": {"device-id", "local-interface", "connecting-interface"},
		},
		DataTypes: map[string]*YANGDataType{
			"/lldp-entries/lldp-entry/device-id":             {Type: "string", Description: "Neighbor device ID key"},
			"/lldp-entries/lldp-entry/local-interface":        {Type: "string", Description: "Local interface key"},
			"/lldp-entries/lldp-entry/connecting-interface":   {Type: "string", Description: "Remote interface key"},
			"/lldp-entries/lldp-entry/ttl":                    {Type: "uint32", Units: "seconds", Description: "Time-to-live"},
			"/lldp-entries/lldp-entry/capabilities":           {Type: "string", Description: "System capabilities"},
			"/lldp-entries/lldp-entry/port-id":                {Type: "string", Description: "Remote port ID"},
			"/lldp-entries/lldp-entry/system-name":            {Type: "string", Description: "System name"},
			"/lldp-entries/lldp-entry/system-desc":            {Type: "string", Description: "System description"},
			"/lldp-entries/lldp-entry/mgmt-addrs":             {Type: "string", Description: "Management addresses"},
		},
		Description: "LLDP neighbor entries (Cisco-IOS-XE-lldp-oper)",
	}

	// ── Cisco-IOS-XE-matm-oper ──────────────────────────────────────────────
	// list /matm-oper-data/matm-table { key "table-type"; }
	// list /matm-oper-data/matm-table/matm-mac-entry { key "mac-addr"; }  (compound with vlan)
	p.modules["Cisco-IOS-XE-matm-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-matm-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-matm-oper",
		Prefix:    "matm-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/matm-oper-data/matm-table":                "table-type",
			"/matm-oper-data/matm-table/matm-mac-entry": "mac-addr",
		},
		ListKeys: map[string][]string{
			"/matm-oper-data/matm-table":                {"table-type"},
			"/matm-oper-data/matm-table/matm-mac-entry": {"mac-addr"},
		},
		DataTypes: map[string]*YANGDataType{
			"/matm-oper-data/matm-table/table-type":                       {Type: "enumeration", Description: "MAC table type key"},
			"/matm-oper-data/matm-table/matm-mac-entry/mac-addr":          {Type: "string", Description: "MAC address key"},
			"/matm-oper-data/matm-table/matm-mac-entry/vlan-id":           {Type: "uint16", Description: "VLAN ID"},
			"/matm-oper-data/matm-table/matm-mac-entry/interface":         {Type: "string", Description: "Port/interface"},
			"/matm-oper-data/matm-table/matm-mac-entry/type":              {Type: "enumeration", Description: "Entry type (static/dynamic)"},
		},
		Description: "MAC address table (Cisco-IOS-XE-matm-oper)",
	}

	// ── Cisco-IOS-XE-mdt-oper ───────────────────────────────────────────────
	// list /mdt-oper-data/mdt-subscriptions { key "subscription-id"; }
	p.modules["Cisco-IOS-XE-mdt-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-mdt-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-mdt-oper",
		Prefix:    "mdt-oper",
		KeyedLeafs: map[string]string{
			"/mdt-oper-data/mdt-subscriptions": "subscription-id",
		},
		ListKeys: map[string][]string{
			"/mdt-oper-data/mdt-subscriptions": {"subscription-id"},
		},
		DataTypes: map[string]*YANGDataType{
			"/mdt-oper-data/mdt-subscriptions/subscription-id": {Type: "uint32", Description: "Subscription ID key"},
			"/mdt-oper-data/mdt-subscriptions/type":            {Type: "enumeration", Description: "Subscription type"},
			"/mdt-oper-data/mdt-subscriptions/state":           {Type: "enumeration", Description: "Subscription state (Valid/Invalid)"},
			"/mdt-oper-data/mdt-subscriptions/comments":        {Type: "string", Description: "Subscription status comments"},
			"/mdt-oper-data/mdt-subscriptions/updates-in":      {Type: "uint64", Units: "count", Description: "Updates received"},
			"/mdt-oper-data/mdt-subscriptions/updates-dampened": {Type: "uint64", Units: "count", Description: "Updates dampened"},
			"/mdt-oper-data/mdt-subscriptions/updates-dropped": {Type: "uint64", Units: "count", Description: "Updates dropped"},
		},
		Description: "MDT subscription operational data (Cisco-IOS-XE-mdt-oper)",
	}

	// ── Cisco-IOS-XE-platform-oper ──────────────────────────────────────────
	// list /components/component { key "cname"; }
	p.modules["Cisco-IOS-XE-platform-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-platform-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-platform-oper",
		Prefix:    "platform-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/components/component": "cname",
		},
		ListKeys: map[string][]string{
			"/components/component": {"cname"},
		},
		DataTypes: map[string]*YANGDataType{
			"/components/component/cname":       {Type: "string", Description: "Component name key"},
			"/components/component/description": {Type: "string", Description: "Component description"},
			"/components/component/type":        {Type: "identityref", Description: "Component type"},
			"/components/component/state":       {Type: "enumeration", Description: "Component state"},
			"/components/component/part-no":     {Type: "string", Description: "Part number"},
			"/components/component/serial-no":   {Type: "string", Description: "Serial number"},
			"/components/component/temperature": {Type: "decimal64", Units: "celsius", Description: "Component temperature"},
			"/components/component/temp-alarm":  {Type: "boolean", Description: "Temperature alarm active"},
		},
		Description: "Platform component inventory (Cisco-IOS-XE-platform-oper)",
	}

	// ── Cisco-IOS-XE-bgp-oper (retained from original) ─────────────────────
	p.modules["Cisco-IOS-XE-bgp-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-bgp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-bgp-oper",
		Prefix:    "bgp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/bgp-state-data/neighbors/neighbor":              "neighbor-id",
			"/bgp-state-data/address-families/address-family": "afi-safi",
		},
		ListKeys: map[string][]string{
			"/bgp-state-data/neighbors/neighbor":              {"neighbor-id"},
			"/bgp-state-data/address-families/address-family": {"afi-safi"},
		},
		DataTypes: map[string]*YANGDataType{
			"/bgp-state-data/neighbors/neighbor/neighbor-id": {Type: "inet:ipv4-address", Description: "BGP neighbor address key"},
		},
		Description: "BGP operational data (Cisco-IOS-XE-bgp-oper)",
	}

	// ── Cisco-IOS-XE-ospf-oper (retained from original) ────────────────────
	p.modules["Cisco-IOS-XE-ospf-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-ospf-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-ospf-oper",
		Prefix:    "ospf-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/ospf-oper-data/ospf-state/ospf-instance":           "router-id",
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area": "area-id",
		},
		ListKeys: map[string][]string{
			"/ospf-oper-data/ospf-state/ospf-instance":           {"router-id"},
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area": {"area-id"},
		},
		DataTypes: map[string]*YANGDataType{
			"/ospf-oper-data/ospf-state/ospf-instance/router-id":         {Type: "inet:ipv4-address", Description: "OSPF router ID key"},
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area/area-id": {Type: "uint32", Description: "OSPF area ID key"},
		},
		Description: "OSPF operational data (Cisco-IOS-XE-ospf-oper)",
	}

	// ── Cisco-IOS-XE-poe-oper (bonus: present in live data) ────────────────
	// list /poe-oper-data/poe-port { key "intf-name"; }
	// list /poe-oper-data/poe-module { key "module-num"; }
	p.modules["Cisco-IOS-XE-poe-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-poe-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-poe-oper",
		Prefix:    "poe-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/poe-oper-data/poe-port":   "intf-name",
			"/poe-oper-data/poe-module": "module-num",
		},
		ListKeys: map[string][]string{
			"/poe-oper-data/poe-port":   {"intf-name"},
			"/poe-oper-data/poe-module": {"module-num"},
		},
		DataTypes: map[string]*YANGDataType{
			"/poe-oper-data/poe-port/intf-name":         {Type: "string", Description: "Interface name key"},
			"/poe-oper-data/poe-port/poe-intf-enabled":  {Type: "boolean", Description: "PoE enabled on interface"},
			"/poe-oper-data/poe-port/power-used":        {Type: "decimal64", Units: "watts", Description: "Power consumed"},
			"/poe-oper-data/poe-port/pd-class":          {Type: "enumeration", Description: "PD class"},
			"/poe-oper-data/poe-module/module-num":      {Type: "uint32", Description: "Module number key"},
			"/poe-oper-data/poe-module/available-power": {Type: "decimal64", Units: "watts", Description: "Available power"},
			"/poe-oper-data/poe-module/used-power":      {Type: "decimal64", Units: "watts", Description: "Used power"},
			"/poe-oper-data/poe-module/remaining-power": {Type: "decimal64", Units: "watts", Description: "Remaining power"},
		},
		Description: "PoE operational data (Cisco-IOS-XE-poe-oper)",
	}

	// --- Platform Software (system memory/DRAM) ---
	// list /cisco-platform-software/control-processes/control-process { key "fru slot bay chassis"; }
	p.modules["Cisco-IOS-XE-platform-software-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-platform-software-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-platform-software-oper",
		Prefix:    "platform-sw-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/cisco-platform-software/control-processes/control-process": "fru",
		},
		ListKeys: map[string][]string{
			"/cisco-platform-software/control-processes/control-process": {"fru", "slot", "bay", "chassis"},
		},
		DataTypes: map[string]*YANGDataType{
			"/cisco-platform-software/control-processes/control-process/memory-stats/total":             {Type: "uint64", Units: "kilobytes", Description: "Total memory"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/used-number":       {Type: "uint64", Units: "kilobytes", Description: "Used memory"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/used-percent":      {Type: "uint8", Units: "percent", Description: "Memory usage percent"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/free-number":       {Type: "uint64", Units: "kilobytes", Description: "Free memory"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/free-percent":      {Type: "uint8", Units: "percent", Description: "Free memory percent"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/available-number":  {Type: "uint64", Units: "kilobytes", Description: "Available memory"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/available-percent": {Type: "uint8", Units: "percent", Description: "Available memory percent"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/committed-number":  {Type: "uint64", Units: "kilobytes", Description: "Committed memory"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/committed-percent": {Type: "uint8", Units: "percent", Description: "Committed memory percent"},
			"/cisco-platform-software/control-processes/control-process/memory-stats/memory-status":     {Type: "string", Description: "Memory status (Healthy/Warning/Critical)"},
		},
		Description: "Platform software operational data (Cisco-IOS-XE-platform-software-oper)",
	}

	// --- Spanning Tree ---
	// list /stp-details/stp-detail { key "instance"; }
	// list /stp-details/stp-detail/interfaces/interface { key "name"; }
	p.modules["Cisco-IOS-XE-spanning-tree-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-spanning-tree-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-spanning-tree-oper",
		Prefix:    "stp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/stp-details/stp-detail":                      "instance",
			"/stp-details/stp-detail/interfaces/interface": "name",
		},
		ListKeys: map[string][]string{
			"/stp-details/stp-detail":                      {"instance"},
			"/stp-details/stp-detail/interfaces/interface": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/stp-details/stp-detail/instance":                                  {Type: "string", Description: "STP instance ID"},
			"/stp-details/stp-detail/designated-root-priority":                  {Type: "uint32", Description: "Root bridge priority"},
			"/stp-details/stp-detail/designated-root-address":                   {Type: "string", Description: "Root bridge MAC address"},
			"/stp-details/stp-detail/root-cost":                                 {Type: "uint32", Description: "Root path cost"},
			"/stp-details/stp-detail/root-port":                                 {Type: "uint16", Description: "Root port number"},
			"/stp-details/stp-detail/interfaces/interface/name":                 {Type: "string", Description: "Interface name"},
			"/stp-details/stp-detail/interfaces/interface/cost":                 {Type: "uint64", Description: "Port cost"},
			"/stp-details/stp-detail/interfaces/interface/port-priority":        {Type: "uint16", Description: "Port priority"},
			"/stp-details/stp-detail/interfaces/interface/role":                 {Type: "enumeration", Description: "STP port role"},
			"/stp-details/stp-detail/interfaces/interface/state":                {Type: "enumeration", Description: "STP port state"},
			"/stp-details/stp-detail/interfaces/interface/bpdu-sent":            {Type: "uint64", Description: "BPDUs sent"},
			"/stp-details/stp-detail/interfaces/interface/bpdu-received":        {Type: "uint64", Description: "BPDUs received"},
			"/stp-details/stp-detail/interfaces/interface/forward-transitions":  {Type: "uint64", Description: "Forward transitions"},
			"/stp-details/stp-detail/interfaces/interface/bpdu-guard":           {Type: "boolean", Description: "BPDU guard enabled"},
			"/stp-details/stp-detail/interfaces/interface/bpdu-filter":          {Type: "boolean", Description: "BPDU filter enabled"},
			"/stp-details/stp-detail/interfaces/interface/guard":                {Type: "enumeration", Description: "Guard type"},
			"/stp-details/stp-detail/interfaces/interface/link-type":            {Type: "enumeration", Description: "Link type"},
		},
		Description: "Spanning tree operational data (Cisco-IOS-XE-spanning-tree-oper)",
	}

	// --- Stack ---
	// list /stack-oper-data/stack-node { key "chassis-number"; }
	// list /stack-oper-data/stack-node/stack-ports { key "port-num"; }
	p.modules["Cisco-IOS-XE-stack-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-stack-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-stack-oper",
		Prefix:    "stack-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/stack-oper-data/stack-node":              "chassis-number",
			"/stack-oper-data/stack-node/stack-ports":  "port-num",
		},
		ListKeys: map[string][]string{
			"/stack-oper-data/stack-node":              {"chassis-number"},
			"/stack-oper-data/stack-node/stack-ports":  {"port-num"},
		},
		DataTypes: map[string]*YANGDataType{
			"/stack-oper-data/stack-node/chassis-number":                  {Type: "uint8", Description: "Chassis number key"},
			"/stack-oper-data/stack-node/role":                            {Type: "enumeration", Description: "Stack member role"},
			"/stack-oper-data/stack-node/node-state":                      {Type: "enumeration", Description: "Stack node state"},
			"/stack-oper-data/stack-node/priority":                        {Type: "uint8", Description: "Stack priority"},
			"/stack-oper-data/stack-node/mac-address":                     {Type: "string", Description: "MAC address"},
			"/stack-oper-data/stack-node/serial-number":                   {Type: "string", Description: "Serial number"},
			"/stack-oper-data/stack-node/latency":                         {Type: "uint32", Description: "Stack latency"},
			"/stack-oper-data/stack-node/sso-ready-flag":                  {Type: "boolean", Description: "SSO ready"},
			"/stack-oper-data/stack-node/stack-ports/port-num":            {Type: "uint8", Description: "Stack port number"},
			"/stack-oper-data/stack-node/stack-ports/port-state":          {Type: "enumeration", Description: "Stack port state"},
			"/stack-oper-data/stack-node/keepalive-counters/sent":         {Type: "uint64", Description: "Keepalive sent"},
			"/stack-oper-data/stack-node/keepalive-counters/received":     {Type: "uint64", Description: "Keepalive received"},
		},
		Description: "Stack operational data (Cisco-IOS-XE-stack-oper)",
	}

	// --- Identity / 802.1X ---
	// list /identity-oper-data/session-context-data { key "mac intf-name"; }
	p.modules["Cisco-IOS-XE-identity-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-identity-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-identity-oper",
		Prefix:    "identity-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/identity-oper-data/session-context-data": "mac",
		},
		ListKeys: map[string][]string{
			"/identity-oper-data/session-context-data": {"mac", "intf-name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/identity-oper-data/session-context-data/mac":             {Type: "string", Description: "Client MAC address"},
			"/identity-oper-data/session-context-data/intf-name":       {Type: "string", Description: "Interface name"},
			"/identity-oper-data/session-context-data/ipv4":            {Type: "string", Description: "IPv4 address"},
			"/identity-oper-data/session-context-data/vlan-id":         {Type: "uint16", Description: "VLAN ID"},
			"/identity-oper-data/session-context-data/authorized":      {Type: "boolean", Description: "Authorization status"},
			"/identity-oper-data/session-context-data/state":           {Type: "enumeration", Description: "Session state"},
			"/identity-oper-data/session-context-data/domain":          {Type: "enumeration", Description: "Auth domain"},
			"/identity-oper-data/session-context-data/device-type":     {Type: "string", Description: "Device type"},
			"/identity-oper-data/session-context-data/device-name":     {Type: "string", Description: "Device name"},
			"/identity-oper-data/session-context-data/policy-name":     {Type: "string", Description: "Applied policy"},
		},
		Description: "Identity/802.1X session data (Cisco-IOS-XE-identity-oper)",
	}

	// --- Switchport ---
	// list /switchport-oper-data/switchport { key "name"; }
	p.modules["Cisco-IOS-XE-switchport-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-switchport-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-switchport-oper",
		Prefix:    "switchport-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/switchport-oper-data/switchport": "name",
		},
		ListKeys: map[string][]string{
			"/switchport-oper-data/switchport": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/switchport-oper-data/switchport/name":                {Type: "string", Description: "Interface name"},
			"/switchport-oper-data/switchport/switchport-mode":     {Type: "enumeration", Description: "Switchport mode (access/trunk)"},
			"/switchport-oper-data/switchport/access-vlan":         {Type: "uint16", Description: "Access VLAN"},
			"/switchport-oper-data/switchport/trunk-native-vlan":   {Type: "uint16", Description: "Trunk native VLAN"},
			"/switchport-oper-data/switchport/operational-mode":    {Type: "enumeration", Description: "Operational mode"},
		},
		Description: "Switchport operational data (Cisco-IOS-XE-switchport-oper)",
	}

	// --- UDLD ---
	// list /udld-oper-data/udld-neighbor-nbr { key "device-id port-id local-port"; }
	p.modules["Cisco-IOS-XE-udld-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-udld-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-udld-oper",
		Prefix:    "udld-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/udld-oper-data/udld-neighbor-nbr": "device-id",
		},
		ListKeys: map[string][]string{
			"/udld-oper-data/udld-neighbor-nbr": {"device-id", "port-id", "local-port"},
		},
		DataTypes: map[string]*YANGDataType{
			"/udld-oper-data/udld-neighbor-nbr/device-id":   {Type: "string", Description: "Neighbor device ID"},
			"/udld-oper-data/udld-neighbor-nbr/port-id":     {Type: "string", Description: "Remote port ID"},
			"/udld-oper-data/udld-neighbor-nbr/local-port":  {Type: "string", Description: "Local port"},
		},
		Description: "UDLD neighbor operational data (Cisco-IOS-XE-udld-oper)",
	}

	// --- Transceiver ---
	// list /transceiver-oper-data/transceiver { key "name"; }
	p.modules["Cisco-IOS-XE-transceiver-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-transceiver-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-transceiver-oper",
		Prefix:    "xcvr-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/transceiver-oper-data/transceiver": "name",
		},
		ListKeys: map[string][]string{
			"/transceiver-oper-data/transceiver": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/transceiver-oper-data/transceiver/name":             {Type: "string", Description: "Transceiver name"},
			"/transceiver-oper-data/transceiver/enabled":          {Type: "boolean", Description: "Transceiver enabled"},
			"/transceiver-oper-data/transceiver/present":          {Type: "enumeration", Description: "Transceiver present"},
			"/transceiver-oper-data/transceiver/connector-type":   {Type: "string", Description: "Connector type"},
			"/transceiver-oper-data/transceiver/ethernet-pmd":     {Type: "string", Description: "Ethernet PMD"},
			"/transceiver-oper-data/transceiver/vendor":           {Type: "string", Description: "Vendor name"},
			"/transceiver-oper-data/transceiver/vendor-part":      {Type: "string", Description: "Vendor part number"},
			"/transceiver-oper-data/transceiver/serial-no":        {Type: "string", Description: "Serial number"},
			"/transceiver-oper-data/transceiver/fault-condition":  {Type: "boolean", Description: "Fault condition"},
		},
		Description: "Transceiver/optics operational data (Cisco-IOS-XE-transceiver-oper)",
	}

	// --- TCAM ---
	// list /tcam-details/tcam-detail { key "fru slot bay chassis"; }
	p.modules["Cisco-IOS-XE-tcam-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-tcam-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-tcam-oper",
		Prefix:    "tcam-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/tcam-details/tcam-detail": "fru",
		},
		ListKeys: map[string][]string{
			"/tcam-details/tcam-detail": {"fru", "slot", "bay", "chassis"},
		},
		DataTypes: map[string]*YANGDataType{
			"/tcam-details/tcam-detail/fru":               {Type: "enumeration", Description: "FRU type key"},
			"/tcam-details/tcam-detail/slot":              {Type: "int32", Description: "Slot number key"},
			"/tcam-details/tcam-detail/asic-num":          {Type: "int32", Description: "ASIC number"},
			"/tcam-details/tcam-detail/used-entries":      {Type: "uint32", Units: "count", Description: "Used TCAM entries"},
			"/tcam-details/tcam-detail/free-entries":      {Type: "uint32", Units: "count", Description: "Free TCAM entries"},
			"/tcam-details/tcam-detail/max-entries":       {Type: "uint32", Units: "count", Description: "Max TCAM entries"},
			"/tcam-details/tcam-detail/used-percent":      {Type: "uint8", Units: "percent", Description: "TCAM usage percent"},
			"/tcam-details/tcam-detail/protocol":          {Type: "string", Description: "Protocol / feature name"},
		},
		Description: "TCAM utilization details (Cisco-IOS-XE-tcam-oper)",
	}

	// --- DHCP ---
	// list /dhcp-oper-data/dhcpv4-server-oper/dhcpv4-server-binding { key "client-id"; }
	// list /dhcp-oper-data/dhcpv4-server-oper/pool { key "pool-name"; }
	p.modules["Cisco-IOS-XE-dhcp-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-dhcp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-dhcp-oper",
		Prefix:    "dhcp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/dhcp-oper-data/dhcpv4-server-oper/pool": "pool-name",
		},
		ListKeys: map[string][]string{
			"/dhcp-oper-data/dhcpv4-server-oper/pool": {"pool-name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/dhcp-oper-data/dhcpv4-server-oper/pool/pool-name":        {Type: "string", Description: "DHCP pool name key"},
			"/dhcp-oper-data/dhcpv4-server-oper/pool/total-addresses":  {Type: "uint32", Units: "count", Description: "Total pool addresses"},
			"/dhcp-oper-data/dhcpv4-server-oper/pool/used-addresses":   {Type: "uint32", Units: "count", Description: "Used pool addresses"},
			"/dhcp-oper-data/dhcpv4-server-oper/pool/avail-addresses":  {Type: "uint32", Units: "count", Description: "Available pool addresses"},
		},
		Description: "DHCP operational data (Cisco-IOS-XE-dhcp-oper)",
	}

	// --- High Availability ---
	// container /ha-oper-data is a singleton
	p.modules["Cisco-IOS-XE-ha-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-ha-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-ha-oper",
		Prefix:    "ha-ios-xe-oper",
		KeyedLeafs: map[string]string{},
		ListKeys:   map[string][]string{},
		DataTypes: map[string]*YANGDataType{
			"/ha-oper-data/ha-state":          {Type: "enumeration", Description: "HA state (active/standby)"},
			"/ha-oper-data/switchover-reason":  {Type: "string", Description: "Last switchover reason"},
			"/ha-oper-data/switchover-count":   {Type: "uint32", Units: "count", Description: "Number of switchovers"},
			"/ha-oper-data/sso-ready":          {Type: "boolean", Description: "SSO ready status"},
		},
		Description: "High availability operational data (Cisco-IOS-XE-ha-oper)",
	}

	// --- Linecard ---
	// list /linecard-oper-data/linecard { key "name"; }
	p.modules["Cisco-IOS-XE-linecard-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-linecard-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-linecard-oper",
		Prefix:    "linecard-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/linecard-oper-data/linecard": "name",
		},
		ListKeys: map[string][]string{
			"/linecard-oper-data/linecard": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			"/linecard-oper-data/linecard/name":         {Type: "string", Description: "Linecard name key"},
			"/linecard-oper-data/linecard/power-admin":  {Type: "enumeration", Description: "Power admin state"},
			"/linecard-oper-data/linecard/status":       {Type: "enumeration", Description: "Linecard status"},
			"/linecard-oper-data/linecard/part-number":  {Type: "string", Description: "Part number"},
			"/linecard-oper-data/linecard/serial-number": {Type: "string", Description: "Serial number"},
		},
		Description: "Linecard operational data (Cisco-IOS-XE-linecard-oper)",
	}

	// --- TrustSec ---
	// list /trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap { key "ip sgt-source"; }
	// list /trustsec-state/cts-sxp-connections/cts-sxp-connection { key "peer-ip"; }
	p.modules["Cisco-IOS-XE-trustsec-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-trustsec-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-trustsec-oper",
		Prefix:    "trustsec-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap": "ip",
			"/trustsec-state/cts-sxp-connections/cts-sxp-connection":     "peer-ip",
		},
		ListKeys: map[string][]string{
			"/trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap": {"ip", "sgt-source"},
			"/trustsec-state/cts-sxp-connections/cts-sxp-connection":     {"peer-ip"},
		},
		DataTypes: map[string]*YANGDataType{
			"/trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap/ip":         {Type: "string", Description: "IP address key"},
			"/trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap/sgt-source": {Type: "string", Description: "SGT source"},
			"/trustsec-state/cts-rolebased-sgtmaps/cts-rolebased-sgtmap/sgt":        {Type: "uint16", Description: "Security Group Tag"},
			"/trustsec-state/cts-sxp-connections/cts-sxp-connection/peer-ip":         {Type: "string", Description: "SXP peer IP key"},
			"/trustsec-state/cts-sxp-connections/cts-sxp-connection/status":          {Type: "enumeration", Description: "SXP connection status"},
			"/trustsec-state/cts-sxp-connections/cts-sxp-connection/mode":            {Type: "enumeration", Description: "SXP connection mode"},
		},
		Description: "TrustSec/CTS operational data (Cisco-IOS-XE-trustsec-oper)",
	}

	// --- VLAN ---
	// list /vlans/vlan { key "id"; }
	p.modules["Cisco-IOS-XE-vlan-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-vlan-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-vlan-oper",
		Prefix:    "vlan-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/vlans/vlan": "id",
		},
		ListKeys: map[string][]string{
			"/vlans/vlan": {"id"},
		},
		DataTypes: map[string]*YANGDataType{
			"/vlans/vlan/id":              {Type: "uint16", Description: "VLAN ID"},
			"/vlans/vlan/name":            {Type: "string", Description: "VLAN name"},
			"/vlans/vlan/status":          {Type: "enumeration", Description: "VLAN status (active/suspend)"},
			"/vlans/vlan/vlan-interfaces": {Type: "string", Description: "Member interfaces"},
		},
		Description: "VLAN operational data (Cisco-IOS-XE-vlan-oper)",
	}

	// --- Install ---
	// list /install-oper-data/install-location-information { key "fru slot bay chassis"; }
	p.modules["Cisco-IOS-XE-install-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-install-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-install-oper",
		Prefix:    "install-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/install-oper-data/install-location-information": "fru",
		},
		ListKeys: map[string][]string{
			"/install-oper-data/install-location-information": {"fru", "slot", "bay", "chassis"},
		},
		DataTypes: map[string]*YANGDataType{
			"/install-oper-data/install-location-information/fru":                          {Type: "enumeration", Description: "FRU type key"},
			"/install-oper-data/install-location-information/slot":                         {Type: "int32", Description: "Slot number key"},
			"/install-oper-data/install-location-information/install-packages/pkg-name":    {Type: "string", Description: "Package name"},
			"/install-oper-data/install-location-information/install-packages/pkg-state":   {Type: "enumeration", Description: "Package state (active/inactive)"},
			"/install-oper-data/install-location-information/install-packages/pkg-version": {Type: "string", Description: "Package version"},
			"/install-oper-data/install-location-information/install-packages/pkg-type":    {Type: "enumeration", Description: "Package type"},
		},
		Description: "Software install operational data (Cisco-IOS-XE-install-oper)",
	}

	// --- Device Hardware ---
	// container /device-hardware-data/device-hardware is a singleton
	// list /device-hardware-data/device-hardware/device-inventory { key "hw-type hw-dev-index"; }
	p.modules["Cisco-IOS-XE-device-hardware-oper"] = &YANGModule{
		Name:      "Cisco-IOS-XE-device-hardware-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-device-hardware-oper",
		Prefix:    "device-hardware-xe-oper",
		KeyedLeafs: map[string]string{
			"/device-hardware-data/device-hardware/device-inventory": "hw-type",
		},
		ListKeys: map[string][]string{
			"/device-hardware-data/device-hardware/device-inventory": {"hw-type", "hw-dev-index"},
		},
		DataTypes: map[string]*YANGDataType{
			// device-system-data singleton (uptime, version, etc.)
			"/device-hardware-data/device-hardware/device-system-data/boot-time":         {Type: "yang:date-and-time", Description: "System boot time"},
			"/device-hardware-data/device-hardware/device-system-data/software-version":   {Type: "string", Description: "Software version"},
			"/device-hardware-data/device-hardware/device-system-data/rommon-version":     {Type: "string", Description: "ROMMON version"},
			"/device-hardware-data/device-hardware/device-system-data/last-reboot-reason": {Type: "string", Description: "Last reboot reason"},
			"/device-hardware-data/device-hardware/device-system-data/current-time":       {Type: "yang:date-and-time", Description: "Current system time"},
			// device-inventory list
			"/device-hardware-data/device-hardware/device-inventory/hw-type":        {Type: "enumeration", Description: "Hardware type key"},
			"/device-hardware-data/device-hardware/device-inventory/hw-dev-index":   {Type: "uint32", Description: "Hardware device index key"},
			"/device-hardware-data/device-hardware/device-inventory/version":        {Type: "string", Description: "Hardware version"},
			"/device-hardware-data/device-hardware/device-inventory/part-number":    {Type: "string", Description: "Part number"},
			"/device-hardware-data/device-hardware/device-inventory/serial-number":  {Type: "string", Description: "Serial number"},
			"/device-hardware-data/device-hardware/device-inventory/hw-description": {Type: "string", Description: "Hardware description"},
		},
		Description: "Device hardware operational data — uptime, version, inventory (Cisco-IOS-XE-device-hardware-oper)",
	}
}

// GetKeyForPath returns the key field name for a given YANG path
func (p *YANGParser) GetKeyForPath(moduleName, path string) string {
	module, exists := p.modules[moduleName]
	if !exists {
		return ""
	}

	// Try exact match first
	if key, found := module.KeyedLeafs[path]; found {
		return key
	}

	// Try pattern matching for flexible path matching
	for yangPath, key := range module.KeyedLeafs {
		if p.matchPath(yangPath, path) {
			return key
		}
	}

	return ""
}

// GetKeysForList returns all key fields for a YANG list
func (p *YANGParser) GetKeysForList(moduleName, listPath string) []string {
	module, exists := p.modules[moduleName]
	if !exists {
		return nil
	}

	// Try exact match first
	if keys, found := module.ListKeys[listPath]; found {
		return keys
	}

	// Try pattern matching
	for yangPath, keys := range module.ListKeys {
		if p.matchPath(yangPath, listPath) {
			return keys
		}
	}

	return nil
}

// matchPath performs flexible path matching
func (p *YANGParser) matchPath(yangPath, telemetryPath string) bool {
	// Remove prefixes for comparison
	cleanYang := p.removePrefixes(yangPath)
	cleanTelemetry := p.removePrefixes(telemetryPath)

	// Direct match
	if cleanYang == cleanTelemetry {
		return true
	}

	// Pattern match - check if telemetry path ends with yang path
	if strings.HasSuffix(cleanTelemetry, cleanYang) {
		return true
	}

	// Pattern match - check if yang path pattern matches telemetry
	return p.isPathPattern(cleanYang, cleanTelemetry)
}

// removePrefixes removes YANG prefixes from paths
func (p *YANGParser) removePrefixes(path string) string {
	// Remove common prefixes like "interfaces-ios-xe-oper:"
	re := regexp.MustCompile(`[a-zA-Z0-9-]+:`)
	return re.ReplaceAllString(path, "")
}

// isPathPattern checks if a YANG path pattern matches a telemetry path
func (p *YANGParser) isPathPattern(yangPattern, telemetryPath string) bool {
	// Simple pattern matching - can be enhanced
	yangParts := strings.Split(strings.Trim(yangPattern, "/"), "/")
	telemetryParts := strings.Split(strings.Trim(telemetryPath, "/"), "/")

	if len(yangParts) > len(telemetryParts) {
		return false
	}

	// Check if yang pattern matches end of telemetry path
	offset := len(telemetryParts) - len(yangParts)
	for i, yangPart := range yangParts {
		if telemetryParts[offset+i] != yangPart {
			return false
		}
	}

	return true
}

// AnalyzeEncodingPath analyzes a telemetry encoding path to identify keys
func (p *YANGParser) AnalyzeEncodingPath(encodingPath string) *PathAnalysis {
	analysis := &PathAnalysis{
		EncodingPath: encodingPath,
		ModuleName:   "",
		Keys:         make(map[string]string),
		ListPath:     "",
	}

	// Extract module name from encoding path
	// Format: Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics
	parts := strings.Split(encodingPath, ":")
	if len(parts) >= 2 {
		analysis.ModuleName = parts[0]
		pathPart := parts[1]

		pathSegments := strings.Split(pathPart, "/")

		// Walk from deepest to shallowest to find the first matching list path.
		// This handles cases like "interfaces/interface/statistics" where the
		// list is "interfaces/interface" but the data contains a trailing leaf
		// container.
		matched := false
		for depth := len(pathSegments); depth >= 1 && !matched; depth-- {
			candidate := "/" + strings.Join(pathSegments[:depth], "/")
			if keys := p.GetKeysForList(analysis.ModuleName, candidate); len(keys) > 0 {
				analysis.ListPath = candidate
				for _, k := range keys {
					analysis.Keys[candidate] = k // primary key
				}
				matched = true
			}
		}

		// Fallback: set ListPath with or without trailing leaf
		if !matched {
			analysis.ListPath = "/" + strings.Join(pathSegments, "/")
		}
	}

	return analysis
}

// PathAnalysis contains the results of analyzing a telemetry path
type PathAnalysis struct {
	EncodingPath string            `json:"encoding_path"`
	ModuleName   string            `json:"module_name"`
	Keys         map[string]string `json:"keys"`      // path -> key field name
	ListPath     string            `json:"list_path"` // the list container path
}

// GetDataTypeForField returns the YANG data type information for a specific field
func (p *YANGParser) GetDataTypeForField(moduleName, fieldPath string) *YANGDataType {
	module, exists := p.modules[moduleName]
	if !exists {
		return nil
	}

	// Try exact match first
	if dataType, found := module.DataTypes[fieldPath]; found {
		return dataType
	}

	// Try pattern matching for flexible path matching
	for yangPath, dataType := range module.DataTypes {
		if p.matchPath(yangPath, fieldPath) {
			return dataType
		}
	}

	return nil
}

// GetDataTypeForEncodingPath analyzes an encoding path and field name to get data type
func (p *YANGParser) GetDataTypeForEncodingPath(encodingPath, fieldName string) *YANGDataType {
	analysis := p.AnalyzeEncodingPath(encodingPath)
	if analysis == nil {
		return nil
	}

	// Construct possible field paths
	possiblePaths := []string{
		analysis.ListPath + "/" + fieldName,
		analysis.ListPath + "/statistics/" + fieldName,
		fieldName, // Direct field name
	}

	for _, path := range possiblePaths {
		if dataType := p.GetDataTypeForField(analysis.ModuleName, path); dataType != nil {
			return dataType
		}
	}

	return nil
}

// IsNumericType checks if a YANG data type is numeric
func (dt *YANGDataType) IsNumericType() bool {
	if dt == nil {
		return false
	}

	numericTypes := []string{
		"uint8", "uint16", "uint32", "uint64",
		"int8", "int16", "int32", "int64",
		"decimal64",
	}

	for _, numType := range numericTypes {
		if dt.Type == numType {
			return true
		}
	}

	return false
}

// IsCounterType checks if this is a counter-type metric (monotonically increasing).
// Counters only increase and reset on restart (e.g. total bytes received).
// Memory amounts, current readings, and rates are NOT counters — they are gauges.
func (dt *YANGDataType) IsCounterType() bool {
	if dt == nil {
		return false
	}

	// Rates are always gauges, never counters.
	if dt.IsGaugeType() {
		return false
	}

	// Only unsigned integers can be counters.
	if !strings.HasPrefix(dt.Type, "uint") {
		return false
	}

	// Units that indicate monotonically increasing counters.
	counterUnits := []string{"bytes", "packets", "count", "errors", "discards"}
	for _, unit := range counterUnits {
		if dt.Units == unit {
			// Guard: "bytes" is a counter for traffic stats, but NOT for
			// memory quantities.  If the description mentions memory,
			// treat as gauge.
			if unit == "bytes" && strings.Contains(strings.ToLower(dt.Description), "memory") {
				return false
			}
			return true
		}
	}

	return false
}

// IsGaugeType checks if this is a gauge-type metric (can increase or decrease)
func (dt *YANGDataType) IsGaugeType() bool {
	if dt == nil {
		return false
	}

	// Gauge types include rates, percentages, current values
	gaugeUnits := []string{
		"percent", "per-second", "pps", "bps", "kbps", "mbps", "gbps",
		"utilization", "rate", "current", "level",
	}

	for _, unit := range gaugeUnits {
		if strings.Contains(dt.Units, unit) {
			return true
		}
	}

	return false
}

// SaveModulesToFile saves the loaded modules to a JSON file for inspection
func (p *YANGParser) SaveModulesToFile(filename string) error {
	data, err := json.MarshalIndent(p.modules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modules: %v", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadModulesFromFile loads modules from a JSON file
func (p *YANGParser) LoadModulesFromFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	return json.Unmarshal(data, &p.modules)
}

// GetAvailableModules returns a list of loaded module names
func (p *YANGParser) GetAvailableModules() []string {
	var modules []string
	for name := range p.modules {
		modules = append(modules, name)
	}
	return modules
}

// ExtractYANGFromFiles attempts to extract YANG module information from .yang files
func (p *YANGParser) ExtractYANGFromFiles(yangDir string) error {
	return filepath.Walk(yangDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yang") {
			return nil
		}

		// Basic YANG file parsing - can be enhanced
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		module := p.parseYANGContent(string(content), filepath.Base(path))
		if module != nil {
			p.modules[module.Name] = module
		}

		return nil
	})
}

// parseYANGContent performs basic parsing of YANG file content
func (p *YANGParser) parseYANGContent(content, filename string) *YANGModule {
	lines := strings.Split(content, "\n")
	module := &YANGModule{
		KeyedLeafs: make(map[string]string),
		ListKeys:   make(map[string][]string),
	}

	var currentPath []string
	var inList bool
	var listPath string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract module name
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module.Name = strings.TrimSuffix(parts[1], " {")
			}
		}

		// Extract namespace
		if strings.HasPrefix(line, "namespace ") {
			re := regexp.MustCompile(`namespace\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				module.Namespace = matches[1]
			}
		}

		// Extract prefix
		if strings.HasPrefix(line, "prefix ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module.Prefix = strings.Trim(parts[1], "\";")
			}
		}

		// Detect list definitions with keys
		if strings.Contains(line, "list ") && strings.Contains(line, "{") {
			inList = true
			re := regexp.MustCompile(`list\s+([^\s{]+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				currentPath = append(currentPath, matches[1])
				listPath = "/" + strings.Join(currentPath, "/")
			}
		}

		// Extract key information
		if inList && strings.HasPrefix(line, "key ") {
			re := regexp.MustCompile(`key\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				keys := strings.Fields(matches[1])
				module.ListKeys[listPath] = keys
				if len(keys) > 0 {
					module.KeyedLeafs[listPath] = keys[0] // Primary key
				}
			}
		}

		// Handle nesting and closing braces
		if strings.Contains(line, "}") {
			if inList {
				inList = false
				if len(currentPath) > 0 {
					currentPath = currentPath[:len(currentPath)-1]
				}
			}
		}
	}

	if module.Name == "" {
		return nil
	}

	return module
}

package models

import "time"

type PortStatus string
type PortSpeed string
type DuplexMode string
type PoEPriority string
type VLANMode string
type STPState string

const (
	PortStatusUp       PortStatus = "up"
	PortStatusDown     PortStatus = "down"
	PortStatusDisabled PortStatus = "disabled"

	PortSpeed10M  PortSpeed = "10M"
	PortSpeed100M PortSpeed = "100M"
	PortSpeed1G   PortSpeed = "1G"
	PortSpeed2_5G PortSpeed = "2.5G"
	PortSpeed10G  PortSpeed = "10G"

	DuplexFull DuplexMode = "full"
	DuplexHalf DuplexMode = "half"

	PoEPriorityLow      PoEPriority = "low"
	PoEPriorityNormal   PoEPriority = "normal"
	PoEPriorityHigh     PoEPriority = "high"
	PoEPriorityCritical PoEPriority = "critical"

	VLANModeTagged   VLANMode = "tagged"
	VLANModeUntagged VLANMode = "untagged"
	VLANModeExcluded VLANMode = "excluded"

	STPStateForwarding STPState = "forwarding"
	STPStateBlocking   STPState = "blocking"
	STPStateListening  STPState = "listening"
	STPStateLearning   STPState = "learning"
	STPStateDisabled   STPState = "disabled"
)

// Switch represents a managed switch in the inventory.
type Switch struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	IP       string        `json:"ip"`
	Model    string        `json:"model"`
	Firmware string        `json:"firmware"`
	Hardware string        `json:"hardware"`
	Serial   string        `json:"serial"`
	MAC      string        `json:"mac"`
	Uptime   time.Duration `json:"uptime"`
	Location string        `json:"location"`
}

// Port represents a single physical switch port.
type Port struct {
	Number      int        `json:"number"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      PortStatus `json:"status"`
	Speed       PortSpeed  `json:"speed"`        // actual negotiated link speed (display)
	SpeedConfig PortSpeed  `json:"speed_config"` // operator-configured speed (used for writes)
	Duplex      DuplexMode `json:"duplex"`
	FlowControl bool       `json:"flow_control"`
	Enabled     bool       `json:"enabled"`
	LastUpdated time.Time  `json:"last_updated,omitempty"`
}

// PortStats holds TX/RX counters for one port.
type PortStats struct {
	PortNumber  int       `json:"port_number"`
	TXBytes     uint64    `json:"tx_bytes"`
	RXBytes     uint64    `json:"rx_bytes"`
	TXPackets   uint64    `json:"tx_packets"`
	RXPackets   uint64    `json:"rx_packets"`
	TXErrors    uint64    `json:"tx_errors"`
	RXErrors    uint64    `json:"rx_errors"`
	TXDropped   uint64    `json:"tx_dropped"`
	RXDropped   uint64    `json:"rx_dropped"`
	LastUpdated time.Time `json:"last_updated,omitempty"`
}

// VLAN represents an 802.1Q VLAN definition.
type VLAN struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	PortMembers map[int]VLANMode `json:"port_members"`
}

// PoEPort holds PoE state for a single port.
type PoEPort struct {
	PortNumber  int         `json:"port_number"`
	Enabled     bool        `json:"enabled"`
	Priority    PoEPriority `json:"priority"`
	PowerLimitW float64     `json:"power_limit_w"`
	PowerWatts  float64     `json:"power_watts"`
	Class       int         `json:"class"`
	LastUpdated time.Time   `json:"last_updated,omitempty"`
}

// PoEStatus holds overall PoE budget information.
type PoEStatus struct {
	TotalBudgetW   float64   `json:"total_budget_w"`
	ConsumedWatts  float64   `json:"consumed_watts"`
	RemainingWatts float64   `json:"remaining_watts"`
	Ports          []PoEPort `json:"ports"`
}

// LAGGroup represents a Link Aggregation Group.
type LAGGroup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Ports       []int  `json:"ports"`
	LACPEnabled bool   `json:"lacp_enabled"`
	Active      bool   `json:"active"`
}

// STPPort holds STP state for a single port.
type STPPort struct {
	PortNumber int      `json:"port_number"`
	State      STPState `json:"state"`
	Role       string   `json:"role"`
	Cost       int      `json:"cost"`
	Priority   int      `json:"priority"`
}

// STPStatus holds global + per-port STP information.
type STPStatus struct {
	Mode         string    `json:"mode"`
	Enabled      bool      `json:"enabled"`
	RootBridgeID string    `json:"root_bridge_id"`
	Ports        []STPPort `json:"ports"`
}

// MACEntry is one row in the MAC address table.
type MACEntry struct {
	MAC       string `json:"mac"`
	VLAN      int    `json:"vlan"`
	Port      int    `json:"port"`
	EntryType string `json:"entry_type"`
}

// LLDPNeighbor is a peer discovered via LLDP.
type LLDPNeighbor struct {
	LocalPort   int    `json:"local_port"`
	ChassisID   string `json:"chassis_id"`
	PortID      string `json:"port_id"`
	SystemName  string `json:"system_name"`
	Description string `json:"description"`
}

// MirrorMode specifies which traffic direction is mirrored.
type MirrorMode string

const (
	MirrorBoth    MirrorMode = "both"
	MirrorIngress MirrorMode = "ingress"
	MirrorEgress  MirrorMode = "egress"
)

// PortMirror holds port mirroring configuration.
type PortMirror struct {
	Enabled     bool       `json:"enabled"`
	DestPort    int        `json:"dest_port"`
	Mode        MirrorMode `json:"mode"`
	SourcePorts []int      `json:"source_ports"`
}

// IGMPGroup is one multicast group entry from IGMP snooping.
type IGMPGroup struct {
	IP    string `json:"ip"`
	VLAN  int    `json:"vlan"`
	Ports []int  `json:"ports"`
}

// IGMPStatus holds global IGMP snooping state.
type IGMPStatus struct {
	Enabled     bool        `json:"enabled"`
	Suppression bool        `json:"suppression"`
	Groups      []IGMPGroup `json:"groups"`
}

// QoSMode selects the QoS priority mode.
type QoSMode string

const (
	QoSModePort  QoSMode = "port"
	QoSMode8021p QoSMode = "802.1p"
	QoSModeDSCP  QoSMode = "dscp"
)

// PortQoS is the per-port priority assignment.
type PortQoS struct {
	PortNumber int `json:"port_number"`
	Priority   int `json:"priority"` // 1=Lowest 2=Normal 3=Medium 4=Highest
}

// QoSStatus holds global QoS mode and per-port priorities.
type QoSStatus struct {
	Mode  QoSMode   `json:"mode"`
	Ports []PortQoS `json:"ports"`
}

// BandwidthControl is per-port ingress/egress rate limiting.
type BandwidthControl struct {
	PortNumber      int  `json:"port_number"`
	IngressEnabled  bool `json:"ingress_enabled"`
	IngressRateKbps int  `json:"ingress_rate_kbps"` // 0 = unlimited
	EgressRateKbps  int  `json:"egress_rate_kbps"`  // 0 = unlimited
}

// StormControl is per-port storm protection thresholds.
type StormControl struct {
	PortNumber         int `json:"port_number"`
	BroadcastKbps      int `json:"broadcast_kbps"`       // 0 = disabled
	MulticastKbps      int `json:"multicast_kbps"`
	UnknownUnicastKbps int `json:"unknown_unicast_kbps"`
}

// PortConfig carries the writable fields of a port for SetPort actions.
type PortConfig struct {
	Enabled     bool      `json:"enabled"`
	Speed       PortSpeed `json:"speed"`
	FlowControl bool      `json:"flow_control"`
}

// SwitchStatus is the runtime reachability state of a switch.
type SwitchStatus string

const (
	SwitchStatusOnline  SwitchStatus = "online"
	SwitchStatusOffline SwitchStatus = "offline"
	SwitchStatusUnknown SwitchStatus = "unknown"
)

// SwitchConfig is the persistent inventory record stored in SQLite.
type SwitchConfig struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IP             string `json:"ip"`
	Username       string `json:"username"`
	Password       string `json:"-"` // plaintext in memory only — never serialized
	InsecureTLS    bool   `json:"insecure_tls"`
	Enabled        bool   `json:"enabled"`
	PollStatsSecs  int    `json:"poll_stats_secs"`
	PollConfigSecs int    `json:"poll_config_secs"`
}

// SwitchSnapshot is the full collected state of one switch at a point in time.
type SwitchSnapshot struct {
	Switch         Switch             `json:"switch"`
	Ports          []Port             `json:"ports"`
	PortStats      []PortStats        `json:"port_stats"`
	VLANs          []VLAN             `json:"vlans"`
	PoE            *PoEStatus         `json:"poe,omitempty"`
	LAGs           []LAGGroup         `json:"lags"`
	STP            *STPStatus         `json:"stp,omitempty"`
	MACTable       []MACEntry         `json:"mac_table"`
	LLDPNeighbors  []LLDPNeighbor     `json:"lldp_neighbors"`
	Mirror         *PortMirror        `json:"mirror,omitempty"`
	IGMP           *IGMPStatus        `json:"igmp,omitempty"`
	QoS            *QoSStatus         `json:"qos,omitempty"`
	Bandwidth      []BandwidthControl `json:"bandwidth"`
	StormControl   []StormControl     `json:"storm_control"`
	LoopPrevention bool               `json:"loop_prevention"`
	CollectedAt    time.Time          `json:"collected_at"`
}

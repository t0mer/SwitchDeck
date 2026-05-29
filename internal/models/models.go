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
	ID       string
	Name     string
	IP       string
	Model    string
	Firmware string
	Hardware string
	Serial   string
	MAC      string
	Uptime   time.Duration
	Location string
}

// Port represents a single physical switch port.
type Port struct {
	Number      int
	Name        string
	Description string
	Status      PortStatus
	Speed       PortSpeed
	Duplex      DuplexMode
	FlowControl bool
	Enabled     bool
	LastUpdated time.Time
}

// PortStats holds TX/RX counters for one port.
type PortStats struct {
	PortNumber  int
	TXBytes     uint64
	RXBytes     uint64
	TXPackets   uint64
	RXPackets   uint64
	TXErrors    uint64
	RXErrors    uint64
	TXDropped   uint64
	RXDropped   uint64
	LastUpdated time.Time
}

// VLAN represents an 802.1Q VLAN definition.
type VLAN struct {
	ID          int
	Name        string
	PortMembers map[int]VLANMode
}

// PoEPort holds PoE state for a single port.
type PoEPort struct {
	PortNumber  int
	Enabled     bool
	Priority    PoEPriority
	PowerLimitW float64
	PowerWatts  float64
	Class       int
	LastUpdated time.Time
}

// PoEStatus holds overall PoE budget information.
type PoEStatus struct {
	TotalBudgetW   float64
	ConsumedWatts  float64
	RemainingWatts float64
	Ports          []PoEPort
}

// LAGGroup represents a Link Aggregation Group.
type LAGGroup struct {
	ID          int
	Name        string
	Ports       []int
	LACPEnabled bool
	Active      bool
}

// STPPort holds STP state for a single port.
type STPPort struct {
	PortNumber int
	State      STPState
	Role       string
	Cost       int
	Priority   int
}

// STPStatus holds global + per-port STP information.
type STPStatus struct {
	Mode         string
	Enabled      bool
	RootBridgeID string
	Ports        []STPPort
}

// MACEntry is one row in the MAC address table.
type MACEntry struct {
	MAC       string
	VLAN      int
	Port      int
	EntryType string
}

// LLDPNeighbor is a peer discovered via LLDP.
type LLDPNeighbor struct {
	LocalPort   int
	ChassisID   string
	PortID      string
	SystemName  string
	Description string
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
	Enabled     bool
	DestPort    int
	Mode        MirrorMode
	SourcePorts []int
}

// IGMPGroup is one multicast group entry from IGMP snooping.
type IGMPGroup struct {
	IP    string
	VLAN  int
	Ports []int
}

// IGMPStatus holds global IGMP snooping state.
type IGMPStatus struct {
	Enabled     bool
	Suppression bool
	Groups      []IGMPGroup
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
	PortNumber int
	Priority   int // 1=Lowest 2=Normal 3=Medium 4=Highest
}

// QoSStatus holds global QoS mode and per-port priorities.
type QoSStatus struct {
	Mode  QoSMode
	Ports []PortQoS
}

// BandwidthControl is per-port ingress/egress rate limiting.
type BandwidthControl struct {
	PortNumber      int
	IngressEnabled  bool
	IngressRateKbps int // 0 = unlimited
	EgressRateKbps  int // 0 = unlimited
}

// StormControl is per-port storm protection thresholds.
type StormControl struct {
	PortNumber         int
	BroadcastKbps      int // 0 = disabled
	MulticastKbps      int
	UnknownUnicastKbps int
}

// PortConfig carries the writable fields of a port for SetPort actions.
type PortConfig struct {
	Enabled     bool
	Speed       PortSpeed
	FlowControl bool
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
	ID             string
	Name           string
	IP             string
	Username       string
	Password       string // plaintext in memory only — encrypted at rest
	InsecureTLS    bool
	Enabled        bool
	PollStatsSecs  int
	PollConfigSecs int
}

// SwitchSnapshot is the full collected state of one switch at a point in time.
type SwitchSnapshot struct {
	Switch         Switch
	Ports          []Port
	PortStats      []PortStats
	VLANs          []VLAN
	PoE            *PoEStatus
	LAGs           []LAGGroup
	STP            *STPStatus
	MACTable       []MACEntry
	LLDPNeighbors  []LLDPNeighbor
	Mirror         *PortMirror
	IGMP           *IGMPStatus
	QoS            *QoSStatus
	Bandwidth      []BandwidthControl
	StormControl   []StormControl
	LoopPrevention bool
	CollectedAt    time.Time
}

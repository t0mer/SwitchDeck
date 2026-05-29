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

// SwitchSnapshot is the full collected state of one switch at a point in time.
type SwitchSnapshot struct {
	Switch        Switch
	Ports         []Port
	PortStats     []PortStats
	VLANs         []VLAN
	PoE           *PoEStatus
	LAGs          []LAGGroup
	STP           *STPStatus
	MACTable      []MACEntry
	LLDPNeighbors []LLDPNeighbor
	CollectedAt   time.Time
}

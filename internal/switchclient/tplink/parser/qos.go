package parser

import (
	"encoding/json"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
)

var qosModeMap = map[int]models.QoSMode{
	1: models.QoSModePort,
	2: models.QoSMode8021p,
	3: models.QoSModeDSCP,
}

// ParseQoSBasic extracts QoS mode and per-port priority from /QosBasicRpm.htm.
func ParseQoSBasic(js string) (*models.QoSStatus, error) {
	mode := extractIntVar(js, "qosMode", 2)
	portNum := extractIntVar(js, "portNumber", 8)

	priRaw := ExtractVar(js, "pPri")
	if priRaw == "" {
		return nil, fmt.Errorf("pPri not found")
	}
	var pris []int
	if err := json.Unmarshal([]byte(JSToJSON(priRaw)), &pris); err != nil {
		return nil, fmt.Errorf("unmarshal pPri: %w", err)
	}

	qos := &models.QoSStatus{
		Mode:  qosModeMap[mode],
		Ports: make([]models.PortQoS, portNum),
	}
	for i := 0; i < portNum; i++ {
		pri := 1
		if i < len(pris) {
			pri = pris[i]
		}
		qos.Ports[i] = models.PortQoS{PortNumber: i + 1, Priority: pri}
	}
	return qos, nil
}

// ParseBandwidth extracts per-port bandwidth control from /QosBandWidthControlRpm.htm.
// bcInfo layout: [ingressEnabled, ingressKbps, egressKbps] per port.
func ParseBandwidth(js string) ([]models.BandwidthControl, error) {
	portNum := extractIntVar(js, "portNumber", 8)
	raw := ExtractVar(js, "bcInfo")
	if raw == "" {
		return nil, fmt.Errorf("bcInfo not found")
	}
	var vals []int
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &vals); err != nil {
		return nil, fmt.Errorf("unmarshal bcInfo: %w", err)
	}

	result := make([]models.BandwidthControl, portNum)
	for i := 0; i < portNum; i++ {
		bc := models.BandwidthControl{PortNumber: i + 1}
		base := i * 3
		if base+2 < len(vals) {
			bc.IngressEnabled = vals[base] != 0
			bc.IngressRateKbps = vals[base+1]
			bc.EgressRateKbps = vals[base+2]
		}
		result[i] = bc
	}
	return result, nil
}

// ParseStormControl extracts per-port storm control from /QosStormControlRpm.htm.
// scInfo layout: [broadcastKbps, multicastKbps, unknownUnicastKbps] per port.
func ParseStormControl(js string) ([]models.StormControl, error) {
	portNum := extractIntVar(js, "portNumber", 8)
	raw := ExtractVar(js, "scInfo")
	if raw == "" {
		return nil, fmt.Errorf("scInfo not found")
	}
	var vals []int
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &vals); err != nil {
		return nil, fmt.Errorf("unmarshal scInfo: %w", err)
	}

	result := make([]models.StormControl, portNum)
	for i := 0; i < portNum; i++ {
		sc := models.StormControl{PortNumber: i + 1}
		base := i * 3
		if base+2 < len(vals) {
			sc.BroadcastKbps = vals[base]
			sc.MulticastKbps = vals[base+1]
			sc.UnknownUnicastKbps = vals[base+2]
		}
		result[i] = sc
	}
	return result, nil
}

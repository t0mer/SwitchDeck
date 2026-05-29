package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// actualSpeedCode maps spd_act to link status and negotiated duplex.
// 0=no link, 1 unused in spd_act, 2=10MHalf, 3=10MFull, 4=100MHalf, 5=100MFull, 6=1000MFull
func actualSpeedCode(code int) (models.PortSpeed, models.DuplexMode, models.PortStatus) {
	switch code {
	case 2:
		return models.PortSpeed10M, models.DuplexHalf, models.PortStatusUp
	case 3:
		return models.PortSpeed10M, models.DuplexFull, models.PortStatusUp
	case 4:
		return models.PortSpeed100M, models.DuplexHalf, models.PortStatusUp
	case 5:
		return models.PortSpeed100M, models.DuplexFull, models.PortStatusUp
	case 6:
		return models.PortSpeed1G, models.DuplexFull, models.PortStatusUp
	default:
		return "", "", models.PortStatusDown
	}
}

// configuredSpeed maps spd_cfg to the PortSpeed the operator chose.
// 1=Auto (no fixed speed), 2/3=10M, 4/5=100M, 6=1000M.
func configuredSpeed(code int) models.PortSpeed {
	switch code {
	case 2, 3:
		return models.PortSpeed10M
	case 4, 5:
		return models.PortSpeed100M
	case 6:
		return models.PortSpeed1G
	default:
		return "" // Auto or unknown
	}
}

// CfgSpeedCode converts a PortSpeed back to the switch's spd_cfg numeric code.
func CfgSpeedCode(s models.PortSpeed) string {
	switch s {
	case models.PortSpeed10M:
		return "3" // 10MFull
	case models.PortSpeed100M:
		return "5" // 100MFull
	case models.PortSpeed1G:
		return "6" // 1000MFull
	default:
		return "1" // Auto
	}
}

type portSettingsJS struct {
	State     []int `json:"state"`
	TrunkInfo []int `json:"trunk_info"`
	SpdCfg    []int `json:"spd_cfg"`
	SpdAct    []int `json:"spd_act"`
	FcCfg     []int `json:"fc_cfg"`
	FcAct     []int `json:"fc_act"`
}

// ParsePortSettings extracts port state from the all_info JS variable on /PortSettingRpm.htm.
func ParsePortSettings(js string) ([]models.Port, error) {
	raw := ExtractVar(js, "all_info")
	if raw == "" {
		return nil, fmt.Errorf("all_info not found in JS")
	}
	portCount := extractIntVar(js, "max_port_num", 8)

	var data portSettingsJS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal all_info: %w", err)
	}

	now := time.Now()
	ports := make([]models.Port, portCount)
	for i := 0; i < portCount; i++ {
		p := models.Port{Number: i + 1, LastUpdated: now}
		if i < len(data.State) {
			p.Enabled = data.State[i] == 1
		}
		if i < len(data.FcCfg) {
			p.FlowControl = data.FcCfg[i] == 1
		}
		// Speed is the operator-configured value (spd_cfg), not the actual link speed.
		// This ensures round-trip writes preserve the setting even when the port has no link.
		if i < len(data.SpdCfg) {
			p.Speed = configuredSpeed(data.SpdCfg[i])
		}
		// Status and duplex reflect actual link state (spd_act).
		if i < len(data.SpdAct) {
			_, dpx, status := actualSpeedCode(data.SpdAct[i])
			p.Duplex = dpx
			if !p.Enabled {
				p.Status = models.PortStatusDisabled
			} else {
				p.Status = status
			}
		}
		ports[i] = p
	}
	return ports, nil
}

type portStatsJS struct {
	State      []int   `json:"state"`
	LinkStatus []int   `json:"link_status"`
	Pkts       []int64 `json:"pkts"`
}

// ParsePortStats extracts TX/RX counters from the all_info JS variable on /PortStatisticsRpm.htm.
// pkts layout: [TXGood, TXBad, RXGood, RXBad] per port.
func ParsePortStats(js string) ([]models.PortStats, error) {
	raw := ExtractVar(js, "all_info")
	if raw == "" {
		return nil, fmt.Errorf("all_info not found in JS")
	}
	portCount := extractIntVar(js, "max_port_num", 8)

	var data portStatsJS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal all_info stats: %w", err)
	}

	now := time.Now()
	stats := make([]models.PortStats, portCount)
	for i := 0; i < portCount; i++ {
		s := models.PortStats{PortNumber: i + 1, LastUpdated: now}
		base := i * 4
		if base+3 < len(data.Pkts) {
			s.TXPackets = uint64(data.Pkts[base])
			s.TXErrors = uint64(data.Pkts[base+1])
			s.RXPackets = uint64(data.Pkts[base+2])
			s.RXErrors = uint64(data.Pkts[base+3])
		}
		stats[i] = s
	}
	return stats, nil
}

// extractIntVar extracts a scalar integer variable from JS, returning fallback on failure.
func extractIntVar(js, name string, fallback int) int {
	raw := ExtractVar(js, name)
	if raw == "" {
		return fallback
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return fallback
	}
	return v
}

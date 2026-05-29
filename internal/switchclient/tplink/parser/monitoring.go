package parser

import (
	"encoding/json"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
)

var mirrorModeMap = map[int]models.MirrorMode{
	0: models.MirrorBoth,
	1: models.MirrorIngress,
	2: models.MirrorEgress,
}

type mirrInfoJS struct {
	Ingress []int `json:"ingress"`
	Egress  []int `json:"egress"`
}

// ParsePortMirror extracts mirroring config from /PortMirrorRpm.htm.
func ParsePortMirror(js string) (*models.PortMirror, error) {
	enabled := extractIntVar(js, "MirrEn", 0) != 0
	destPort := extractIntVar(js, "MirrPort", 0)
	modeCode := extractIntVar(js, "MirrMode", 0)

	raw := ExtractVar(js, "mirr_info")
	if raw == "" {
		return nil, fmt.Errorf("mirr_info not found")
	}
	var info mirrInfoJS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &info); err != nil {
		return nil, fmt.Errorf("unmarshal mirr_info: %w", err)
	}

	seen := map[int]bool{}
	for i, v := range info.Ingress {
		if v != 0 {
			seen[i+1] = true
		}
	}
	for i, v := range info.Egress {
		if v != 0 {
			seen[i+1] = true
		}
	}
	var src []int
	for p := 1; p <= 8; p++ {
		if seen[p] {
			src = append(src, p)
		}
	}

	mode := mirrorModeMap[modeCode]
	if mode == "" {
		mode = models.MirrorBoth
	}

	return &models.PortMirror{
		Enabled:     enabled,
		DestPort:    destPort,
		Mode:        mode,
		SourcePorts: src,
	}, nil
}

// ParseLoopPrevention extracts the lpEn scalar from /LoopPreventionRpm.htm.
func ParseLoopPrevention(js string) (bool, error) {
	raw := ExtractVar(js, "lpEn")
	if raw == "" {
		return false, fmt.Errorf("lpEn not found")
	}
	return raw == "1", nil
}

type igmpDS struct {
	State            int      `json:"state"`
	SuppressionState int      `json:"suppressionState"`
	Count            int      `json:"count"`
	IPStr            []string `json:"ipStr"`
	VlanStr          []string `json:"vlanStr"`
	PortStr          []string `json:"portStr"`
}

// ParseIGMP extracts IGMP snooping state from igmp_ds on /IgmpSnoopingRpm.htm.
func ParseIGMP(js string) (*models.IGMPStatus, error) {
	raw := ExtractVar(js, "igmp_ds")
	if raw == "" {
		return nil, fmt.Errorf("igmp_ds not found")
	}
	var data igmpDS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal igmp_ds: %w", err)
	}
	status := &models.IGMPStatus{
		Enabled:     data.State == 1,
		Suppression: data.SuppressionState == 1,
	}
	for i, ip := range data.IPStr {
		g := models.IGMPGroup{IP: ip}
		if i < len(data.VlanStr) {
			fmt.Sscanf(data.VlanStr[i], "%d", &g.VLAN)
		}
		status.Groups = append(status.Groups, g)
	}
	return status, nil
}

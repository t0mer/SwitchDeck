package parser

import (
	"encoding/json"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
)

type trunkConfJS struct {
	MaxTrunkNum int   `json:"maxTrunkNum"`
	PortNum     int   `json:"portNum"`
	PortStrG1   []int `json:"portStr_g1"`
	PortStrG2   []int `json:"portStr_g2"`
}

// ParseLAG extracts Link Aggregation Group config from trunk_conf on /PortTrunkRpm.htm.
// Returns only groups that have at least one member port.
func ParseLAG(js string) ([]models.LAGGroup, error) {
	raw := ExtractVar(js, "trunk_conf")
	if raw == "" {
		return nil, fmt.Errorf("trunk_conf not found")
	}
	var data trunkConfJS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal trunk_conf: %w", err)
	}

	portNum := data.PortNum
	if portNum == 0 {
		portNum = 8
	}

	memberPorts := func(arr []int) []int {
		var ports []int
		for i := 0; i < portNum && i < len(arr); i++ {
			if arr[i] != 0 {
				ports = append(ports, i+1)
			}
		}
		return ports
	}

	var groups []models.LAGGroup
	for idx, arr := range [][]int{data.PortStrG1, data.PortStrG2} {
		ports := memberPorts(arr)
		if len(ports) == 0 {
			continue
		}
		groups = append(groups, models.LAGGroup{
			ID:          idx + 1,
			Name:        fmt.Sprintf("LAG%d", idx+1),
			Ports:       ports,
			LACPEnabled: false,
			Active:      true,
		})
	}
	return groups, nil
}

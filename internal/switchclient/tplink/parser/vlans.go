package parser

import (
	"encoding/json"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
)

type vlan8021QJS struct {
	State     int      `json:"state"`
	PortNum   int      `json:"portNum"`
	MaxVids   int      `json:"maxVids"`
	Count     int      `json:"count"`
	Vids      []int    `json:"vids"`
	Names     []string `json:"names"`
	TagMbrs   []int    `json:"tagMbrs"`
	UntagMbrs []int    `json:"untagMbrs"`
}

// ParseVLAN8021Q extracts 802.1Q VLAN table from qvlan_ds on /Vlan8021QRpm.htm.
func ParseVLAN8021Q(js string) ([]models.VLAN, error) {
	raw := ExtractVar(js, "qvlan_ds")
	if raw == "" {
		return nil, fmt.Errorf("qvlan_ds not found")
	}
	var data vlan8021QJS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal qvlan_ds: %w", err)
	}
	portNum := data.PortNum
	if portNum == 0 {
		portNum = 8
	}
	vlans := make([]models.VLAN, len(data.Vids))
	for i, vid := range data.Vids {
		v := models.VLAN{ID: vid, PortMembers: make(map[int]models.VLANMode)}
		if i < len(data.Names) {
			v.Name = data.Names[i]
		}
		var tagMask, untagMask int
		if i < len(data.TagMbrs) {
			tagMask = data.TagMbrs[i]
		}
		if i < len(data.UntagMbrs) {
			untagMask = data.UntagMbrs[i]
		}
		for port := 1; port <= portNum; port++ {
			bit := 1 << (port - 1)
			switch {
			case tagMask&bit != 0:
				v.PortMembers[port] = models.VLANModeTagged
			case untagMask&bit != 0:
				v.PortMembers[port] = models.VLANModeUntagged
			default:
				v.PortMembers[port] = models.VLANModeExcluded
			}
		}
		vlans[i] = v
	}
	return vlans, nil
}

type pvlanDS struct {
	State   int   `json:"state"`
	PortNum int   `json:"portNum"`
	Vids    []int `json:"vids"`
	Count   int   `json:"count"`
	Mbrs    []int `json:"mbrs"`
}

// ParseVLANPortBased extracts port-based VLANs from pvlan_ds on /VlanPortBasicRpm.htm.
func ParseVLANPortBased(js string) ([]models.VLAN, error) {
	raw := ExtractVar(js, "pvlan_ds")
	if raw == "" {
		return nil, fmt.Errorf("pvlan_ds not found")
	}
	var data pvlanDS
	if err := json.Unmarshal([]byte(JSToJSON(raw)), &data); err != nil {
		return nil, fmt.Errorf("unmarshal pvlan_ds: %w", err)
	}
	portNum := data.PortNum
	if portNum == 0 {
		portNum = 8
	}
	vlans := make([]models.VLAN, len(data.Vids))
	for i, vid := range data.Vids {
		v := models.VLAN{ID: vid, PortMembers: make(map[int]models.VLANMode)}
		var mask int
		if i < len(data.Mbrs) {
			mask = data.Mbrs[i]
		}
		for port := 1; port <= portNum; port++ {
			if mask&(1<<(port-1)) != 0 {
				v.PortMembers[port] = models.VLANModeUntagged
			} else {
				v.PortMembers[port] = models.VLANModeExcluded
			}
		}
		vlans[i] = v
	}
	return vlans, nil
}

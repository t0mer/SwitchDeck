package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/t0mer/SwitchDeck/internal/models"
)

type systemInfoJS struct {
	DescriStr   []string `json:"descriStr"`
	MacStr      []string `json:"macStr"`
	IPStr       []string `json:"ipStr"`
	NetmaskStr  []string `json:"netmaskStr"`
	GatewayStr  []string `json:"gatewayStr"`
	FirmwareStr []string `json:"firmwareStr"`
	HardwareStr []string `json:"hardwareStr"`
}

// ParseSystemInfo extracts switch identity from the info_ds JS variable block.
func ParseSystemInfo(js string) (*models.Switch, error) {
	raw := ExtractVar(js, "info_ds")
	if raw == "" {
		return nil, fmt.Errorf("info_ds not found in JS")
	}
	j := JSToJSON(raw)
	var data systemInfoJS
	if err := json.Unmarshal([]byte(j), &data); err != nil {
		return nil, fmt.Errorf("unmarshal info_ds: %w", err)
	}
	sw := &models.Switch{}
	if len(data.DescriStr) > 0 {
		sw.Name = data.DescriStr[0]
	}
	if len(data.MacStr) > 0 {
		sw.MAC = data.MacStr[0]
	}
	if len(data.IPStr) > 0 {
		sw.IP = data.IPStr[0]
	}
	if len(data.FirmwareStr) > 0 {
		sw.Firmware = data.FirmwareStr[0]
	}
	if len(data.HardwareStr) > 0 {
		sw.Hardware = data.HardwareStr[0]
		parts := strings.Fields(sw.Hardware)
		if len(parts) > 0 {
			sw.Model = parts[0]
		}
	}
	return sw, nil
}

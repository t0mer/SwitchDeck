package tplink

import (
	"context"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink/parser"
)

// SetPort configures a single port's enable state, speed, and flow control.
// The switch uses a CGI endpoint that accepts the target port number plus
// single state/speed/flowcontrol values — not a full array of all ports.
func (t *TPLink) SetPort(ctx context.Context, port int, cfg models.PortConfig) error {
	if port < 1 || port > 8 {
		return fmt.Errorf("invalid port number %d (must be 1-8)", port)
	}
	state := "0"
	if cfg.Enabled {
		state = "1"
	}
	fc := "0"
	if cfg.FlowControl {
		fc = "1"
	}
	return t.postMultipart(ctx, "/port_setting.cgi", map[string]string{
		"portid":      fmt.Sprintf("%d", port),
		"state":       state,
		"speed":       parser.CfgSpeedCode(cfg.Speed),
		"flowcontrol": fc,
		"apply":       "Apply",
	})
}

// ResetPortCounters clears all port TX/RX counters.
func (t *TPLink) ResetPortCounters(ctx context.Context) error {
	return t.postAction(ctx, "/PortStatisticsRpm.htm", map[string]string{"clean": "clean"})
}

// SetVLANs writes 802.1Q VLAN configuration to the switch.
func (t *TPLink) SetVLANs(ctx context.Context, vlans []models.VLAN) error {
	data := map[string]string{"apply": "apply", "state": "1", "count": fmt.Sprintf("%d", len(vlans))}
	for i, v := range vlans {
		data[fmt.Sprintf("vid%d", i)] = fmt.Sprintf("%d", v.ID)
		data[fmt.Sprintf("name%d", i)] = v.Name
		var tagMask, untagMask int
		for port, mode := range v.PortMembers {
			bit := 1 << (port - 1)
			switch mode {
			case models.VLANModeTagged:
				tagMask |= bit
			case models.VLANModeUntagged:
				untagMask |= bit
			}
		}
		data[fmt.Sprintf("tagMbr%d", i)] = fmt.Sprintf("%d", tagMask)
		data[fmt.Sprintf("untagMbr%d", i)] = fmt.Sprintf("%d", untagMask)
	}
	return t.postAction(ctx, "/Vlan8021QRpm.htm", data)
}

// SetPortMirror configures port mirroring.
func (t *TPLink) SetPortMirror(ctx context.Context, m models.PortMirror) error {
	enabled := "0"
	if m.Enabled {
		enabled = "1"
	}
	modeCode := "0"
	switch m.Mode {
	case models.MirrorIngress:
		modeCode = "1"
	case models.MirrorEgress:
		modeCode = "2"
	}
	var ingressMask, egressMask int
	for _, p := range m.SourcePorts {
		bit := 1 << (p - 1)
		if m.Mode == models.MirrorIngress || m.Mode == models.MirrorBoth {
			ingressMask |= bit
		}
		if m.Mode == models.MirrorEgress || m.Mode == models.MirrorBoth {
			egressMask |= bit
		}
	}
	return t.postAction(ctx, "/PortMirrorRpm.htm", map[string]string{
		"mirrorEn":    enabled,
		"mirrorPort":  fmt.Sprintf("%d", m.DestPort),
		"mirrorMode":  modeCode,
		"ingressMask": fmt.Sprintf("%d", ingressMask),
		"egressMask":  fmt.Sprintf("%d", egressMask),
		"apply":       "apply",
	})
}

// SetLAG configures static trunk groups (no LACP on TL-SG108E).
func (t *TPLink) SetLAG(ctx context.Context, groups []models.LAGGroup) error {
	g1 := make([]string, 8)
	g2 := make([]string, 8)
	for i := range g1 {
		g1[i] = "0"
		g2[i] = "0"
	}
	for _, g := range groups {
		target := g1
		if g.ID == 2 {
			target = g2
		}
		for _, p := range g.Ports {
			if p >= 1 && p <= 8 {
				target[p-1] = "1"
			}
		}
	}
	return t.postAction(ctx, "/PortTrunkRpm.htm", map[string]string{
		"portStr_g1": joinStrings(g1),
		"portStr_g2": joinStrings(g2),
		"apply":      "apply",
	})
}

// SetQoS sets the global QoS mode and per-port priorities.
func (t *TPLink) SetQoS(ctx context.Context, qos models.QoSStatus) error {
	modeCode := "2"
	switch qos.Mode {
	case models.QoSModePort:
		modeCode = "1"
	case models.QoSModeDSCP:
		modeCode = "3"
	}
	pris := make([]string, 8)
	for i := range pris {
		pris[i] = "1"
	}
	for _, p := range qos.Ports {
		if p.PortNumber >= 1 && p.PortNumber <= 8 {
			pris[p.PortNumber-1] = fmt.Sprintf("%d", p.Priority)
		}
	}
	return t.postAction(ctx, "/QosBasicRpm.htm", map[string]string{
		"qosMode": modeCode,
		"pPri":    joinStrings(pris),
		"apply":   "apply",
	})
}

// SetBandwidth sets per-port ingress/egress rate limits.
func (t *TPLink) SetBandwidth(ctx context.Context, bw []models.BandwidthControl) error {
	vals := make([]string, 8*3+1)
	for i := range vals {
		vals[i] = "0"
	}
	for _, b := range bw {
		if b.PortNumber < 1 || b.PortNumber > 8 {
			continue
		}
		base := (b.PortNumber - 1) * 3
		if b.IngressEnabled {
			vals[base] = "1"
		}
		vals[base+1] = fmt.Sprintf("%d", b.IngressRateKbps)
		vals[base+2] = fmt.Sprintf("%d", b.EgressRateKbps)
	}
	return t.postAction(ctx, "/QosBandWidthControlRpm.htm", map[string]string{
		"bcInfo": joinStrings(vals),
		"apply":  "apply",
	})
}

// SetStormControl sets per-port storm control thresholds.
func (t *TPLink) SetStormControl(ctx context.Context, sc []models.StormControl) error {
	vals := make([]string, 8*3+1)
	for i := range vals {
		vals[i] = "0"
	}
	for _, s := range sc {
		if s.PortNumber < 1 || s.PortNumber > 8 {
			continue
		}
		base := (s.PortNumber - 1) * 3
		vals[base] = fmt.Sprintf("%d", s.BroadcastKbps)
		vals[base+1] = fmt.Sprintf("%d", s.MulticastKbps)
		vals[base+2] = fmt.Sprintf("%d", s.UnknownUnicastKbps)
	}
	return t.postAction(ctx, "/QosStormControlRpm.htm", map[string]string{
		"scInfo": joinStrings(vals),
		"apply":  "apply",
	})
}

// SetIGMP enables or disables IGMP snooping.
func (t *TPLink) SetIGMP(ctx context.Context, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return t.postAction(ctx, "/IgmpSnoopingRpm.htm", map[string]string{"enable": v, "apply": "apply"})
}

// SetLoopPrevention enables or disables loop prevention.
func (t *TPLink) SetLoopPrevention(ctx context.Context, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return t.postAction(ctx, "/LoopPreventionRpm.htm", map[string]string{"lpEn": v, "apply": "apply"})
}

// Reboot reboots the switch. The switch will be unreachable for ~30 seconds.
// A connection reset after the reboot POST is expected and treated as success.
func (t *TPLink) Reboot(ctx context.Context) error {
	return t.postReboot(ctx)
}

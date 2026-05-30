package tplink

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink/parser"
)

// GetSnapshot fetches all available switch data sequentially.
// A 2-second delay between pages prevents exhausting the switch's
// resource-constrained embedded web server.
func (t *TPLink) GetSnapshot(ctx context.Context) (*models.SwitchSnapshot, error) {
	snap := &models.SwitchSnapshot{CollectedAt: time.Now()}

	steps := []struct {
		name string
		path string
		fn   func(string) error
	}{
		{"system", "/SystemInfoRpm.htm", t.applySystemInfo(snap)},
		{"ports", "/PortSettingRpm.htm", t.applyPortSettings(snap)},
		{"lag", "/PortTrunkRpm.htm", t.applyLAG(snap)},
		{"vlan-802.1q", "/Vlan8021QRpm.htm", t.applyVLAN8021Q(snap)},
		{"vlan-port", "/VlanPortBasicRpm.htm", t.applyVLANPortBased(snap)},
		{"igmp", "/IgmpSnoopingRpm.htm", t.applyIGMP(snap)},
		{"qos", "/QosBasicRpm.htm", t.applyQoS(snap)},
		{"bandwidth", "/QosBandWidthControlRpm.htm", t.applyBandwidth(snap)},
		{"storm", "/QosStormControlRpm.htm", t.applyStormControl(snap)},
		{"mirror", "/PortMirrorRpm.htm", t.applyMirror(snap)},
		{"loop", "/LoopPreventionRpm.htm", t.applyLoopPrevention(snap)},
		{"stats", "/PortStatisticsRpm.htm", t.applyPortStats(snap)},
	}

	// The switch tolerates ~6 rapid sequential requests before needing recovery.
	// 200ms between pages within a batch is empirically safe; 1.5s between
	// batches gives the embedded web server enough time to reset. This yields
	// ~5-6s total collection time (down from ~8.5s at 500ms/2s).
	const batchSize = 6
	const interRequest = 200 * time.Millisecond
	const interBatch = 1500 * time.Millisecond

	for i, step := range steps {
		if err := ctx.Err(); err != nil {
			return snap, fmt.Errorf("context cancelled at step %s: %w", step.name, err)
		}
		js, err := t.fetchPage(ctx, step.path)
		if err != nil {
			log.Printf("switchclient: snapshot step %s: %v", step.name, err)
		} else if err := step.fn(js); err != nil {
			log.Printf("switchclient: parse step %s: %v", step.name, err)
		}
		if i == len(steps)-1 {
			break
		}
		delay := interRequest
		if (i+1)%batchSize == 0 {
			delay = interBatch
		}
		select {
		case <-ctx.Done():
			return snap, ctx.Err()
		case <-time.After(delay):
		}
	}
	return snap, nil
}

// RefreshStats fetches only port statistics (fast path for 60s polling).
func (t *TPLink) RefreshStats(ctx context.Context) ([]models.PortStats, error) {
	js, err := t.fetchPage(ctx, "/PortStatisticsRpm.htm")
	if err != nil {
		return nil, fmt.Errorf("refresh stats: %w", err)
	}
	return parser.ParsePortStats(js)
}

// RefreshPorts fetches only /PortSettingRpm.htm to confirm a write landed.
func (t *TPLink) RefreshPorts(ctx context.Context) ([]models.Port, error) {
	js, err := t.fetchPage(ctx, "/PortSettingRpm.htm")
	if err != nil {
		return nil, fmt.Errorf("refresh ports: %w", err)
	}
	return parser.ParsePortSettings(js)
}

func (t *TPLink) applySystemInfo(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		sw, err := parser.ParseSystemInfo(js)
		if err != nil {
			return err
		}
		snap.Switch.Name = sw.Name
		snap.Switch.MAC = sw.MAC
		snap.Switch.IP = sw.IP
		snap.Switch.Firmware = sw.Firmware
		snap.Switch.Hardware = sw.Hardware
		snap.Switch.Model = sw.Model
		return nil
	}
}

func (t *TPLink) applyPortSettings(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		ports, err := parser.ParsePortSettings(js)
		if err != nil {
			return err
		}
		snap.Ports = ports
		return nil
	}
}

func (t *TPLink) applyPortStats(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		stats, err := parser.ParsePortStats(js)
		if err != nil {
			return err
		}
		snap.PortStats = stats
		return nil
	}
}

func (t *TPLink) applyLAG(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		lags, err := parser.ParseLAG(js)
		if err != nil {
			return err
		}
		snap.LAGs = lags
		return nil
	}
}

func (t *TPLink) applyVLAN8021Q(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		vlans, err := parser.ParseVLAN8021Q(js)
		if err != nil {
			return err
		}
		if len(vlans) > 0 {
			snap.VLANs = vlans
		}
		return nil
	}
}

func (t *TPLink) applyVLANPortBased(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		vlans, err := parser.ParseVLANPortBased(js)
		if err != nil {
			return err
		}
		if len(snap.VLANs) == 0 && len(vlans) > 0 {
			snap.VLANs = vlans
		}
		return nil
	}
}

func (t *TPLink) applyIGMP(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		igmp, err := parser.ParseIGMP(js)
		if err != nil {
			return err
		}
		snap.IGMP = igmp
		return nil
	}
}

func (t *TPLink) applyQoS(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		qos, err := parser.ParseQoSBasic(js)
		if err != nil {
			return err
		}
		snap.QoS = qos
		return nil
	}
}

func (t *TPLink) applyBandwidth(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		bw, err := parser.ParseBandwidth(js)
		if err != nil {
			return err
		}
		snap.Bandwidth = bw
		return nil
	}
}

func (t *TPLink) applyStormControl(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		sc, err := parser.ParseStormControl(js)
		if err != nil {
			return err
		}
		snap.StormControl = sc
		return nil
	}
}

func (t *TPLink) applyMirror(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		m, err := parser.ParsePortMirror(js)
		if err != nil {
			return err
		}
		snap.Mirror = m
		return nil
	}
}

func (t *TPLink) applyLoopPrevention(snap *models.SwitchSnapshot) func(string) error {
	return func(js string) error {
		enabled, err := parser.ParseLoopPrevention(js)
		if err != nil {
			return err
		}
		snap.LoopPrevention = enabled
		return nil
	}
}

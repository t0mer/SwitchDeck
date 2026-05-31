// Package metrics implements a Prometheus collector that performs a live
// collection from every enabled switch on each scrape.
package metrics

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/store"
)

const ns = "switchdeck"

var (
	swLabels   = []string{"switch_id", "switch_name"}
	portLabels = []string{"switch_id", "switch_name", "port"}
)

// SwitchCollector implements prometheus.Collector. Every time Collect is
// called it triggers a fresh CollectNow on all enabled switches in parallel
// (with a 25-second deadline), then emits metrics from the returned snapshots.
type SwitchCollector struct {
	mgr    *manager.Manager
	st     store.Store
	encKey []byte

	// switch-level
	switchInfo          *prometheus.Desc
	switchUp            *prometheus.Desc
	switchCollecting    *prometheus.Desc
	switchPortsTotal    *prometheus.Desc
	switchPortsUp       *prometheus.Desc
	switchPortsDown     *prometheus.Desc
	switchLastCollected *prometheus.Desc

	// port state
	portUp        *prometheus.Desc
	portEnabled   *prometheus.Desc
	portSpeedMbps *prometheus.Desc

	// port statistics (counters)
	portRxBytes   *prometheus.Desc
	portTxBytes   *prometheus.Desc
	portRxPackets *prometheus.Desc
	portTxPackets *prometheus.Desc
	portRxErrors  *prometheus.Desc
	portTxErrors  *prometheus.Desc
	portRxDropped *prometheus.Desc
	portTxDropped *prometheus.Desc

	// PoE
	poeBudgetWatts   *prometheus.Desc
	poeConsumedWatts *prometheus.Desc
	poePortEnabled   *prometheus.Desc
	poePortWatts     *prometheus.Desc

	// STP
	stpEnabled   *prometheus.Desc
	stpPortState *prometheus.Desc

	// MAC / LLDP / IGMP
	macTableEntries *prometheus.Desc
	lldpNeighbors   *prometheus.Desc
	igmpEnabled     *prometheus.Desc
	igmpGroups      *prometheus.Desc

	// QoS / bandwidth / storm
	qosPortPriority      *prometheus.Desc
	bandwidthIngressKbps *prometheus.Desc
	bandwidthEgressKbps  *prometheus.Desc
	stormBroadcastKbps   *prometheus.Desc
	stormMulticastKbps   *prometheus.Desc
	stormUnknownKbps     *prometheus.Desc

	// misc
	loopPrevention *prometheus.Desc
	vlanCount      *prometheus.Desc
	lagCount       *prometheus.Desc
	scrapeSuccess  *prometheus.Desc
}

func d(name, help string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(ns, "", name), help, labels, nil)
}

// NewSwitchCollector creates a SwitchCollector wired to the given manager and store.
func NewSwitchCollector(mgr *manager.Manager, st store.Store, encKey []byte) *SwitchCollector {
	return &SwitchCollector{
		mgr:    mgr,
		st:     st,
		encKey: encKey,

		switchInfo: prometheus.NewDesc(
			prometheus.BuildFQName(ns, "switch", "info"),
			"Switch metadata. Value is always 1.",
			[]string{"switch_id", "switch_name", "ip", "model", "firmware", "hardware"}, nil,
		),
		switchUp:            d("switch_up", "1 if the switch is online, 0 otherwise.", swLabels),
		switchCollecting:    d("switch_collecting", "1 if a data collection is currently in progress.", swLabels),
		switchPortsTotal:    d("switch_ports_total", "Total number of physical ports.", swLabels),
		switchPortsUp:       d("switch_ports_up", "Number of ports with link up.", swLabels),
		switchPortsDown:     d("switch_ports_down", "Number of ports with link down.", swLabels),
		switchLastCollected: d("switch_last_collected_timestamp_seconds", "Unix timestamp of the last successful data collection.", swLabels),

		portUp:        d("port_up", "1 if port link is up, 0 otherwise.", portLabels),
		portEnabled:   d("port_enabled", "1 if port is administratively enabled.", portLabels),
		portSpeedMbps: d("port_speed_mbps", "Negotiated port speed in Mbps (0 if link is down).", portLabels),

		portRxBytes:   d("port_rx_bytes_total", "Total bytes received on this port.", portLabels),
		portTxBytes:   d("port_tx_bytes_total", "Total bytes transmitted on this port.", portLabels),
		portRxPackets: d("port_rx_packets_total", "Total packets received on this port.", portLabels),
		portTxPackets: d("port_tx_packets_total", "Total packets transmitted on this port.", portLabels),
		portRxErrors:  d("port_rx_errors_total", "Total receive errors on this port.", portLabels),
		portTxErrors:  d("port_tx_errors_total", "Total transmit errors on this port.", portLabels),
		portRxDropped: d("port_rx_dropped_total", "Total received packets dropped on this port.", portLabels),
		portTxDropped: d("port_tx_dropped_total", "Total transmitted packets dropped on this port.", portLabels),

		poeBudgetWatts:   d("poe_budget_watts", "Total PoE power budget in watts.", swLabels),
		poeConsumedWatts: d("poe_consumed_watts", "PoE power currently consumed in watts.", swLabels),
		poePortEnabled:   d("poe_port_enabled", "1 if PoE is enabled on this port.", portLabels),
		poePortWatts:     d("poe_port_watts", "PoE power consumed by this port in watts.", portLabels),

		stpEnabled:   d("stp_enabled", "1 if Spanning Tree Protocol is globally enabled.", swLabels),
		stpPortState: d("stp_port_state", "STP port state: forwarding=5, learning=4, listening=3, blocking=2, disabled=1.", portLabels),

		macTableEntries: d("mac_table_entries_total", "Number of entries in the MAC address table.", swLabels),
		lldpNeighbors:   d("lldp_neighbors_total", "Number of LLDP neighbors discovered.", swLabels),
		igmpEnabled:     d("igmp_enabled", "1 if IGMP snooping is enabled.", swLabels),
		igmpGroups:      d("igmp_groups_total", "Number of active IGMP multicast groups.", swLabels),

		qosPortPriority:      d("qos_port_priority", "QoS priority for the port (1=lowest … 4=highest).", portLabels),
		bandwidthIngressKbps: d("bandwidth_ingress_kbps", "Configured ingress bandwidth limit in Kbps (0 = unlimited).", portLabels),
		bandwidthEgressKbps:  d("bandwidth_egress_kbps", "Configured egress bandwidth limit in Kbps (0 = unlimited).", portLabels),
		stormBroadcastKbps:   d("storm_broadcast_kbps", "Storm control broadcast threshold in Kbps (0 = disabled).", portLabels),
		stormMulticastKbps:   d("storm_multicast_kbps", "Storm control multicast threshold in Kbps (0 = disabled).", portLabels),
		stormUnknownKbps:     d("storm_unknown_unicast_kbps", "Storm control unknown-unicast threshold in Kbps (0 = disabled).", portLabels),

		loopPrevention: d("loop_prevention_enabled", "1 if loop prevention is enabled.", swLabels),
		vlanCount:      d("vlan_count", "Number of configured VLANs.", swLabels),
		lagCount:       d("lag_count", "Number of configured Link Aggregation Groups.", swLabels),
		scrapeSuccess:  d("scrape_success", "1 if a cached snapshot is available for this switch.", swLabels),
	}
}

// Describe sends all metric descriptors to Prometheus.
func (c *SwitchCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.allDescs() {
		ch <- desc
	}
}

func (c *SwitchCollector) allDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		c.switchInfo, c.switchUp, c.switchCollecting,
		c.switchPortsTotal, c.switchPortsUp, c.switchPortsDown, c.switchLastCollected,
		c.portUp, c.portEnabled, c.portSpeedMbps,
		c.portRxBytes, c.portTxBytes, c.portRxPackets, c.portTxPackets,
		c.portRxErrors, c.portTxErrors, c.portRxDropped, c.portTxDropped,
		c.poeBudgetWatts, c.poeConsumedWatts, c.poePortEnabled, c.poePortWatts,
		c.stpEnabled, c.stpPortState,
		c.macTableEntries, c.lldpNeighbors,
		c.igmpEnabled, c.igmpGroups,
		c.qosPortPriority,
		c.bandwidthIngressKbps, c.bandwidthEgressKbps,
		c.stormBroadcastKbps, c.stormMulticastKbps, c.stormUnknownKbps,
		c.loopPrevention, c.vlanCount, c.lagCount,
		c.scrapeSuccess,
	}
}

type scrapeResult struct {
	cfg  models.SwitchConfig
	snap *models.SwitchSnapshot
	err  error
}

// Collect reads the most-recent cached snapshot from each enabled switch's
// background worker and emits all available metrics. Using the cache avoids
// triggering a new login + collection cycle on every Prometheus scrape, which
// would amplify load on the switches and could be abused as a DoS vector.
// The cache is kept current by the workers' poll tickers (poll_stats_secs /
// poll_config_secs), so scrape data is always at most one poll interval old.
func (c *SwitchCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfgs, err := c.st.ListSwitches(ctx, c.encKey)
	if err != nil {
		return
	}

	for _, cfg := range cfgs {
		if !cfg.Enabled {
			continue
		}
		snap, err := c.mgr.LastSnapshot(cfg.ID)
		c.emitSwitch(ch, scrapeResult{cfg: cfg, snap: snap, err: err})
	}
}

func (c *SwitchCollector) emitSwitch(ch chan<- prometheus.Metric, r scrapeResult) {
	id, name := r.cfg.ID, r.cfg.Name

	success := 1.0
	if r.err != nil {
		success = 0
	}
	ch <- g(c.scrapeSuccess, success, id, name)

	if r.err != nil || r.snap == nil {
		ch <- g(c.switchUp, 0, id, name)
		return
	}
	snap := r.snap
	sw := snap.Switch

	ch <- prometheus.MustNewConstMetric(c.switchInfo, prometheus.GaugeValue, 1,
		id, name, r.cfg.IP, sw.Model, sw.Firmware, sw.Hardware)

	ch <- g(c.switchUp, 1, id, name)

	_, collecting := c.mgr.CollectingStartedAt(id)
	ch <- gb(c.switchCollecting, collecting, id, name)

	var total, up, down int
	for _, p := range snap.Ports {
		total++
		switch p.Status {
		case models.PortStatusUp:
			up++
		case models.PortStatusDown:
			down++
		}
	}
	ch <- g(c.switchPortsTotal, float64(total), id, name)
	ch <- g(c.switchPortsUp, float64(up), id, name)
	ch <- g(c.switchPortsDown, float64(down), id, name)

	if !snap.CollectedAt.IsZero() {
		ch <- g(c.switchLastCollected, float64(snap.CollectedAt.Unix()), id, name)
	}

	// ── per-port state ────────────────────────────────────────────────────
	for _, p := range snap.Ports {
		port := strconv.Itoa(p.Number)
		ch <- gb(c.portUp, p.Status == models.PortStatusUp, id, name, port)
		ch <- gb(c.portEnabled, p.Enabled, id, name, port)
		ch <- g(c.portSpeedMbps, speedMbps(p.Speed), id, name, port)
	}

	// ── port statistics ───────────────────────────────────────────────────
	for _, s := range snap.PortStats {
		port := strconv.Itoa(s.PortNumber)
		ch <- cnt(c.portRxBytes, float64(s.RXBytes), id, name, port)
		ch <- cnt(c.portTxBytes, float64(s.TXBytes), id, name, port)
		ch <- cnt(c.portRxPackets, float64(s.RXPackets), id, name, port)
		ch <- cnt(c.portTxPackets, float64(s.TXPackets), id, name, port)
		ch <- cnt(c.portRxErrors, float64(s.RXErrors), id, name, port)
		ch <- cnt(c.portTxErrors, float64(s.TXErrors), id, name, port)
		ch <- cnt(c.portRxDropped, float64(s.RXDropped), id, name, port)
		ch <- cnt(c.portTxDropped, float64(s.TXDropped), id, name, port)
	}

	// ── PoE ───────────────────────────────────────────────────────────────
	if snap.PoE != nil {
		ch <- g(c.poeBudgetWatts, snap.PoE.TotalBudgetW, id, name)
		ch <- g(c.poeConsumedWatts, snap.PoE.ConsumedWatts, id, name)
		for _, pp := range snap.PoE.Ports {
			port := strconv.Itoa(pp.PortNumber)
			ch <- gb(c.poePortEnabled, pp.Enabled, id, name, port)
			ch <- g(c.poePortWatts, pp.PowerWatts, id, name, port)
		}
	}

	// ── STP ───────────────────────────────────────────────────────────────
	if snap.STP != nil {
		ch <- gb(c.stpEnabled, snap.STP.Enabled, id, name)
		for _, sp := range snap.STP.Ports {
			port := strconv.Itoa(sp.PortNumber)
			ch <- g(c.stpPortState, stpStateNum(sp.State), id, name, port)
		}
	}

	// ── MAC / LLDP ────────────────────────────────────────────────────────
	ch <- g(c.macTableEntries, float64(len(snap.MACTable)), id, name)
	ch <- g(c.lldpNeighbors, float64(len(snap.LLDPNeighbors)), id, name)

	// ── IGMP ──────────────────────────────────────────────────────────────
	if snap.IGMP != nil {
		ch <- gb(c.igmpEnabled, snap.IGMP.Enabled, id, name)
		ch <- g(c.igmpGroups, float64(len(snap.IGMP.Groups)), id, name)
	}

	// ── QoS ───────────────────────────────────────────────────────────────
	if snap.QoS != nil {
		for _, q := range snap.QoS.Ports {
			port := strconv.Itoa(q.PortNumber)
			ch <- g(c.qosPortPriority, float64(q.Priority), id, name, port)
		}
	}

	// ── Bandwidth control ─────────────────────────────────────────────────
	for _, bw := range snap.Bandwidth {
		port := strconv.Itoa(bw.PortNumber)
		ch <- g(c.bandwidthIngressKbps, float64(bw.IngressRateKbps), id, name, port)
		ch <- g(c.bandwidthEgressKbps, float64(bw.EgressRateKbps), id, name, port)
	}

	// ── Storm control ─────────────────────────────────────────────────────
	for _, sc := range snap.StormControl {
		port := strconv.Itoa(sc.PortNumber)
		ch <- g(c.stormBroadcastKbps, float64(sc.BroadcastKbps), id, name, port)
		ch <- g(c.stormMulticastKbps, float64(sc.MulticastKbps), id, name, port)
		ch <- g(c.stormUnknownKbps, float64(sc.UnknownUnicastKbps), id, name, port)
	}

	// ── Misc ──────────────────────────────────────────────────────────────
	ch <- gb(c.loopPrevention, snap.LoopPrevention, id, name)
	ch <- g(c.vlanCount, float64(len(snap.VLANs)), id, name)
	ch <- g(c.lagCount, float64(len(snap.LAGs)), id, name)
}

// ── metric constructors ───────────────────────────────────────────────────

func g(desc *prometheus.Desc, v float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, labels...)
}

func cnt(desc *prometheus.Desc, v float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v, labels...)
}

func gb(desc *prometheus.Desc, b bool, labels ...string) prometheus.Metric {
	v := 0.0
	if b {
		v = 1
	}
	return g(desc, v, labels...)
}

// ── helpers ───────────────────────────────────────────────────────────────

func speedMbps(s models.PortSpeed) float64 {
	switch s {
	case models.PortSpeed10M:
		return 10
	case models.PortSpeed100M:
		return 100
	case models.PortSpeed1G:
		return 1000
	case models.PortSpeed2_5G:
		return 2500
	case models.PortSpeed10G:
		return 10000
	default:
		return 0
	}
}

func stpStateNum(s models.STPState) float64 {
	switch s {
	case models.STPStateForwarding:
		return 5
	case models.STPStateLearning:
		return 4
	case models.STPStateListening:
		return 3
	case models.STPStateBlocking:
		return 2
	case models.STPStateDisabled:
		return 1
	default:
		return 0
	}
}

package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink/parser"
)

// ── Task 3: shared utilities ──────────────────────────────────────────────────

func TestExtractFirstScript(t *testing.T) {
	html := `<html><head><script>
var x = 1;
</script></head><body></body></html>`
	got := parser.ExtractFirstScript(html)
	if got == "" {
		t.Fatal("expected non-empty script content")
	}
	if !strings.Contains(got, "var x = 1") {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestExtractVar_scalar(t *testing.T) {
	js := `var lpEn=1;`
	got := parser.ExtractVar(js, "lpEn")
	if got != "1" {
		t.Fatalf("expected '1', got %q", got)
	}
}

func TestExtractVar_object(t *testing.T) {
	js := `var info_ds = {
descriStr:["hello"]
};`
	got := parser.ExtractVar(js, "info_ds")
	if got == "" {
		t.Fatal("expected non-empty value")
	}
}

func TestJSToJSON_keys(t *testing.T) {
	js := `{state:[1,2],trunk_info:[0,0]}`
	got := parser.JSToJSON(js)
	if !strings.Contains(got, `"state"`) {
		t.Fatalf("expected quoted keys, got: %s", got)
	}
}

func TestJSToJSON_hex(t *testing.T) {
	js := `{mbrs:[255,0]}`
	got := parser.JSToJSON(js)
	if strings.Contains(got, "0x") {
		t.Fatalf("hex values not converted: %s", got)
	}
}

func TestJSToJSON_newArray(t *testing.T) {
	js := `new Array(1,2,3)`
	got := parser.JSToJSON(js)
	if got != "[1,2,3]" {
		t.Fatalf("expected [1,2,3], got %q", got)
	}
}

// ── Task 4: system info ───────────────────────────────────────────────────────

func TestParseSystemInfo(t *testing.T) {
	js := `var info_ds = {
descriStr:[
"TestSwitch"
],
macStr:[
"AA:BB:CC:DD:EE:01"
],
ipStr:[
"192.168.1.10"
],
netmaskStr:[
"255.255.255.0"
],
gatewayStr:[
"192.168.1.1"
],
firmwareStr:[
"1.0.0 Build 20201208 Rel.40304"
],
hardwareStr:[
"TL-SG108E 6.0"
]
};`
	sw, err := parser.ParseSystemInfo(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sw.Name != "TestSwitch" {
		t.Errorf("Name: got %q, want %q", sw.Name, "TestSwitch")
	}
	if sw.MAC != "AA:BB:CC:DD:EE:01" {
		t.Errorf("MAC: got %q", sw.MAC)
	}
	if sw.IP != "192.168.1.10" {
		t.Errorf("IP: got %q", sw.IP)
	}
	if sw.Firmware != "1.0.0 Build 20201208 Rel.40304" {
		t.Errorf("Firmware: got %q", sw.Firmware)
	}
	if sw.Hardware != "TL-SG108E 6.0" {
		t.Errorf("Hardware: got %q", sw.Hardware)
	}
	if sw.Model != "TL-SG108E" {
		t.Errorf("Model: got %q, want %q", sw.Model, "TL-SG108E")
	}
}

// ── Task 5: port settings and stats ──────────────────────────────────────────

func TestParsePortSettings(t *testing.T) {
	js := `var max_port_num = 8;
var all_info = {
state:[1,1,0,0,1,1,0,1,0,0],
trunk_info:[0,0,0,0,0,0,0,0,0,0],
spd_cfg:[6,5,1,1,6,5,1,5,0,0],
spd_act:[6,5,0,0,6,5,0,5,0,0],
fc_cfg:[0,0,0,0,0,0,0,0,0,0],
fc_act:[0,0,0,0,0,0,0,0,0,0]
};`
	ports, err := parser.ParsePortSettings(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 8 {
		t.Fatalf("expected 8 ports, got %d", len(ports))
	}
	// Port 1: enabled, spd_cfg=6 (configured 1G), spd_act=6 (actual 1G)
	if !ports[0].Enabled {
		t.Error("port 1 should be enabled")
	}
	if ports[0].Speed != models.PortSpeed1G {
		t.Errorf("port 1 Speed (actual): got %v, want 1G", ports[0].Speed)
	}
	if ports[0].SpeedConfig != models.PortSpeed1G {
		t.Errorf("port 1 SpeedConfig: got %v, want 1G", ports[0].SpeedConfig)
	}
	if ports[0].Duplex != models.DuplexFull {
		t.Errorf("port 1 duplex: got %v, want full", ports[0].Duplex)
	}
	if ports[0].Status != models.PortStatusUp {
		t.Errorf("port 1 status: got %v, want up", ports[0].Status)
	}
	// Port 3: disabled (state=0)
	if ports[2].Status != models.PortStatusDisabled {
		t.Errorf("port 3 status: got %v, want disabled", ports[2].Status)
	}
	// Port 5: spd_cfg=6 → SpeedConfig=1G; spd_act=6 → Speed=1G
	if ports[4].Speed != models.PortSpeed1G {
		t.Errorf("port 5 Speed (actual): got %v, want 1G", ports[4].Speed)
	}
	if ports[4].SpeedConfig != models.PortSpeed1G {
		t.Errorf("port 5 SpeedConfig: got %v, want 1G", ports[4].SpeedConfig)
	}
}

func TestParsePortStats(t *testing.T) {
	js := `var max_port_num = 8;
var all_info = {
state:[1,1,1,1,1,1,1,1,0,0],
link_status:[6,5,0,0,6,5,0,5,0,0],
pkts:[1000,0,500,10, 2000,0,1000,20, 0,0,0,0, 50,0,25,5, 3000,0,2000,30, 2500,0,1500,25, 0,0,0,0, 75,0,40,10, 0,0]
};`
	stats, err := parser.ParsePortStats(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 8 {
		t.Fatalf("expected 8 stats, got %d", len(stats))
	}
	if stats[0].TXPackets != 1000 {
		t.Errorf("port 1 TXPackets: got %d, want 1000", stats[0].TXPackets)
	}
	if stats[0].RXPackets != 500 {
		t.Errorf("port 1 RXPackets: got %d, want 500", stats[0].RXPackets)
	}
	if stats[0].RXErrors != 10 {
		t.Errorf("port 1 RXErrors: got %d, want 10", stats[0].RXErrors)
	}
}

// ── Task 6: VLANs ─────────────────────────────────────────────────────────────

func TestParseVLAN8021Q(t *testing.T) {
	js := `var qvlan_ds = {
state:1,
portNum:8,
vids:[10,20],
count:2,
maxVids:32,
names:["Mgmt","Servers"],
tagMbrs:[2,4],
untagMbrs:[1,2],
lagIds:[0,0,0,0,0,0,0,0],
lagMbrs:[0,0,0]
};`
	vlans, err := parser.ParseVLAN8021Q(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vlans) != 2 {
		t.Fatalf("expected 2 VLANs, got %d", len(vlans))
	}
	if vlans[0].ID != 10 {
		t.Errorf("VLAN[0].ID: got %d, want 10", vlans[0].ID)
	}
	if vlans[0].Name != "Mgmt" {
		t.Errorf("VLAN[0].Name: got %q, want Mgmt", vlans[0].Name)
	}
	// tagMbrs[0]=2 = 0b00000010 → port 2 tagged
	if vlans[0].PortMembers[2] != models.VLANModeTagged {
		t.Errorf("port 2 should be tagged in VLAN 10, got %v", vlans[0].PortMembers[2])
	}
	// untagMbrs[0]=1 = 0b00000001 → port 1 untagged
	if vlans[0].PortMembers[1] != models.VLANModeUntagged {
		t.Errorf("port 1 should be untagged in VLAN 10, got %v", vlans[0].PortMembers[1])
	}
}

func TestParseVLANPortBased(t *testing.T) {
	js := `var pvlan_ds = {
state:1,
portNum:8,
vids:[1],
count:1,
mbrs:[255],
lagIds:[0,0,0,0,0,0,0,0],
lagMbrs:[0,0,0]
};`
	vlans, err := parser.ParseVLANPortBased(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vlans) != 1 {
		t.Fatalf("expected 1 VLAN, got %d", len(vlans))
	}
	// 0xFF=255 = all 8 ports
	if len(vlans[0].PortMembers) != 8 {
		t.Errorf("expected 8 port members, got %d", len(vlans[0].PortMembers))
	}
}

// ── Task 7: LAG ───────────────────────────────────────────────────────────────

func TestParseLAG(t *testing.T) {
	g1 := make([]int, 32)
	g1[0], g1[1] = 1, 1
	g2 := make([]int, 32)
	g2[4], g2[5] = 1, 1

	js := fmt.Sprintf(`var trunk_conf = {
maxTrunkNum:2,
portNum:8,
portStr_g1:[%s],
portStr_g2:[%s]
};`, intSliceJS(g1), intSliceJS(g2))

	lags, err := parser.ParseLAG(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lags) != 2 {
		t.Fatalf("expected 2 LAG groups, got %d", len(lags))
	}
	if lags[0].ID != 1 {
		t.Errorf("LAG[0].ID: got %d, want 1", lags[0].ID)
	}
	if len(lags[0].Ports) != 2 || lags[0].Ports[0] != 1 || lags[0].Ports[1] != 2 {
		t.Errorf("LAG1 ports: got %v, want [1 2]", lags[0].Ports)
	}
	if len(lags[1].Ports) != 2 || lags[1].Ports[0] != 5 || lags[1].Ports[1] != 6 {
		t.Errorf("LAG2 ports: got %v, want [5 6]", lags[1].Ports)
	}
}

func intSliceJS(s []int) string {
	parts := make([]string, len(s))
	for i, v := range s {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ",")
}

// ── Task 8: QoS, bandwidth, storm control ────────────────────────────────────

func TestParseQoSBasic(t *testing.T) {
	js := `var qosMode = 2;
var pPri = new Array(1,2,3,4,1,2,3,4);
var portNumber = 8;`
	qos, err := parser.ParseQoSBasic(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qos.Mode != models.QoSMode8021p {
		t.Errorf("mode: got %v, want 802.1p", qos.Mode)
	}
	if len(qos.Ports) != 8 {
		t.Fatalf("expected 8 ports, got %d", len(qos.Ports))
	}
	if qos.Ports[0].Priority != 1 {
		t.Errorf("port 1 priority: got %d, want 1", qos.Ports[0].Priority)
	}
}

func TestParseBandwidth(t *testing.T) {
	js := `var portNumber = 8;
var bcInfo = new Array(
1,512,1024,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0);`
	bw, err := parser.ParseBandwidth(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bw) != 8 {
		t.Fatalf("expected 8 entries, got %d", len(bw))
	}
	if !bw[0].IngressEnabled {
		t.Error("port 1 ingress should be enabled")
	}
	if bw[0].IngressRateKbps != 512 {
		t.Errorf("port 1 ingress rate: got %d, want 512", bw[0].IngressRateKbps)
	}
}

func TestParseStormControl(t *testing.T) {
	js := `var portNumber = 8;
var scInfo = new Array(
1024,512,256,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0,0,0,
0);`
	sc, err := parser.ParseStormControl(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sc) != 8 {
		t.Fatalf("expected 8 entries, got %d", len(sc))
	}
	if sc[0].BroadcastKbps != 1024 {
		t.Errorf("port 1 broadcast: got %d, want 1024", sc[0].BroadcastKbps)
	}
}

// ── Task 9: monitoring ────────────────────────────────────────────────────────

func TestParsePortMirror(t *testing.T) {
	js := `var MirrEn = 1;
var MirrPort = 3;
var MirrMode = 0;
var mirr_info= {
ingress:[0,1,1,0,0,0,0,0,0,0],
egress:[0,1,1,0,0,0,0,0,0,0]};`
	m, err := parser.ParsePortMirror(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.Enabled {
		t.Error("mirror should be enabled")
	}
	if m.DestPort != 3 {
		t.Errorf("dest port: got %d, want 3", m.DestPort)
	}
	if m.Mode != models.MirrorBoth {
		t.Errorf("mode: got %v, want both", m.Mode)
	}
	if len(m.SourcePorts) != 2 {
		t.Errorf("source ports: got %v, expected 2 ports", m.SourcePorts)
	}
}

func TestParseLoopPrevention(t *testing.T) {
	if enabled, err := parser.ParseLoopPrevention(`var lpEn=1;`); err != nil || !enabled {
		t.Errorf("expected true, nil; got %v, %v", enabled, err)
	}
	if enabled, err := parser.ParseLoopPrevention(`var lpEn=0;`); err != nil || enabled {
		t.Errorf("expected false, nil; got %v, %v", enabled, err)
	}
}

func TestParseIGMP(t *testing.T) {
	js := `var igmp_ds = {
state:1,
suppressionState:1,
count:0,
ipStr:[],
vlanStr:[],
portStr:[],
lagMbrs:[0,0]
};`
	igmp, err := parser.ParseIGMP(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !igmp.Enabled {
		t.Error("IGMP should be enabled")
	}
	if !igmp.Suppression {
		t.Error("suppression should be enabled")
	}
}

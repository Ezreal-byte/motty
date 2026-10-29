//go:build darwin || windows || linux

package workbench

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"motty/internal/i18n"
	"motty/internal/monitoring"
	"motty/internal/remotestats"
	"motty/internal/sshconn"
)

func dashboardNativeFixture(t *testing.T) *State {
	s := newTestState(t, testOptions(t))
	s.setLanguage(i18n.Chinese)
	s.setAppearance(appearanceDark)
	s.activeSection = sectionDashboard
	s.hosts = nil
	p := monitoring.Panel{ID: "production", Name: "生产环境", Note: "Web、API 与数据库", Interval: 5, Retention: 60}
	for i, name := range []string{"prod-web-01", "prod-api-02", "db-primary", "worker-01", "cache-01", "backup-01"} {
		id := fmt.Sprintf("host-%d", i)
		p.Hosts = append(p.Hosts, id)
		s.hosts = append(s.hosts, Host{ID: uint32(i + 1), CatalogID: id, Label: name, Address: fmt.Sprintf("192.168.1.%d", 10+i), Port: 22, User: "deploy", System: sshconn.SystemUbuntu, Icon: hostIconOf(sshconn.SystemUbuntu), Group: "生产环境"})
	}
	s.dashboard.panels = []monitoring.Panel{p}
	s.dashboard.active = p.ID
	s.dashboard.running = map[string]bool{p.ID: true}
	s.dashboard.engine.Set(p, true)
	end := time.Date(2026, 10, 29, 15, 0, 5, 0, time.Local)
	for i, id := range p.Hosts {
		cpu := []float64{24, 41, 62, 18, 37, 29}[i]
		mem := []uint64{42, 56, 78, 38, 49, 46}[i]
		disk := []float64{61, 48, 71, 35, 58, 52}[i]
		s.dashboard.engine.Details(id, remotestats.Details{CPUCount: 4, Networks: []remotestats.Network{{Name: "eth0", Received: 20 << 30, Sent: 4 << 30, ReceivePerSecond: 128 << 10, SendPerSecond: 54 << 10, HasRate: true}, {Name: "docker0", Received: 8 << 30, Sent: 2 << 30, ReceivePerSecond: 32 << 10, SendPerSecond: 12 << 10, HasRate: true}}, Disks: []remotestats.Disk{{Path: "/", Percent: disk, Used: uint64(disk) * (1 << 30), Total: 100 << 30, Available: uint64(100-disk) * (1 << 30)}, {Path: "/data", Percent: 32, Used: 64 << 30, Total: 200 << 30, Available: 136 << 30}}}, false)
		for n := 0; n <= 720; n++ {
			noise := math.Sin(float64(n)*.031+float64(i))*5 + math.Sin(float64(n)*.19)*2
			s.dashboard.engine.Publish(id, end.Add(time.Duration(n-720)*5*time.Second), remotestats.Sample{CPUPercent: cpu + noise, MemoryUsedBytes: uint64(float64(mem)*(1<<30)/100*16 + noise*(1<<24)), MemoryTotalBytes: 16 << 30, ReceiveBytesPerSecond: uint64(128*1024 + float64(i)*40000 + noise*8000), SendBytesPerSecond: uint64(54*1024 + float64(i)*18000 + noise*2000), NetworkInterface: "eth0"})
		}
	}
	return s
}
func TestDashboardNativeViewsAndModal(t *testing.T) {
	for _, scale := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			s := dashboardNativeFixture(t)
			view, err := headless.Open(app.Config{Title: "Motty", Width: 1487, Height: 1058, Tree: Tree(), Paint: s.Paint, OnPointer: s.Pointer, OnKey: s.Key, OnShortcut: s.Shortcut, OnScroll: s.Scroll, OnResize: s.Resize}, nil, headless.Options{Scale: scale})
			if err != nil {
				t.Fatal(err)
			}
			defer view.Close()
			shot := func(name string) {
				if err := view.Rest(); err != nil {
					t.Fatal(err)
				}
				if dir := os.Getenv("MOTTY_DASHBOARD_SCREENSHOTS"); dir != "" {
					if err := view.Screenshot(filepath.Join(dir, fmt.Sprintf("%s-%g.png", name, scale))); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err = view.Rest(); err != nil {
				t.Fatal(err)
			}
			shot("dashboard-detailed")
			for page := 1; page <= 2; page++ {
				s.Click(hitDashboardViewList.At(3 + page))
				if err = view.Resize(1487, 1058); err != nil {
					t.Fatal(err)
				}
				if s.dashboardRow(s.dashboardPanel(), "host-0", int(s.dashboard.body.Width)).page != page {
					t.Fatal("host device tab did not switch")
				}
				shot(fmt.Sprintf("dashboard-detailed-view-%d", page))
			}
			s.Click(hitDashboardViewList.At(3))
			if err = view.Resize(1487, 1058); err != nil {
				t.Fatal(err)
			}
			if err = view.ClickLabel(s.messages.Dashboard.Simple); err != nil {
				t.Fatal(err)
			}
			if !s.dashboardPanel().Simple {
				t.Fatal("simple control did not switch view")
			}
			shot("dashboard-simple")
			for page := 1; page <= 2; page++ {
				s.Click(hitDashboardSimpleViewList.At(3 + page))
				if err = view.Resize(1487, 1058); err != nil {
					t.Fatal(err)
				}
				first := s.dashboardRow(s.dashboardPanel(), "host-0", int(s.dashboard.body.Width))
				second := s.dashboardRow(s.dashboardPanel(), "host-1", int(s.dashboard.body.Width))
				if first.simplePage != page || second.simplePage != 0 {
					t.Fatal("a host view switch affected another server")
				}
				shot(fmt.Sprintf("dashboard-simple-view-%d", page))
			}
			s.Click(hitDashboardSimpleViewList.At(4))
			s.Click(hitDashboardSimpleViewList.At(8))
			if err = view.Resize(1487, 1058); err != nil {
				t.Fatal(err)
			}
			shot("dashboard-simple-mixed")
			s.Click(hitDashboardSimpleViewList.At(3))
			s.Click(hitDashboardSimpleViewList.At(6))
			if err = view.Resize(1487, 1058); err != nil {
				t.Fatal(err)
			}

			for _, host := range s.hosts {
				if _, ok := view.Hit(s.messages.Dashboard.Terminal); !ok {
					t.Fatalf("terminal shortcut absent for %s", host.Label)
				}
			}
			if err = view.ClickLabel(s.messages.Dashboard.New); err != nil {
				t.Fatal(err)
			}
			if s.modal != modalDashboard {
				t.Fatal("new panel did not open")
			}
			shot("dashboard-create")
			if err = view.Keys("escape"); err != nil {
				t.Fatal(err)
			}
			if s.modal != modalNone {
				t.Fatal("escape did not dismiss")
			}
			s.setAppearance(appearanceLight)
			if err = view.Resize(1487, 1058); err != nil {
				t.Fatal(err)
			}
			shot("dashboard-light")
			s.setLanguage(i18n.English)
			if err = view.Resize(1024, 768); err != nil {
				t.Fatal(err)
			}
			shot("dashboard-english")
			if err = view.Resize(800, 720); err != nil {
				t.Fatal(err)
			}
			shot("dashboard-narrow")
		})
	}
}

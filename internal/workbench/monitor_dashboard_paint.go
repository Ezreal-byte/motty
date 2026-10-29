package workbench

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"motty/internal/monitoring"
	"motty/internal/remotestats"
	"motty/internal/workbench/icons"
)

type dashboardRow struct {
	page                                            int
	simplePage                                      int
	chartIDs                                        [4]ui.ID
	snapshot                                        monitoring.Snapshot
	width                                           int
	x                                               []float32
	values                                          [5][]float32
	charts                                          [4]monitorChart
	origin                                          time.Time
	clock                                           func([]byte, float32) []byte
	label, cpu, memory, capacity, down, up, updated string
	disks                                           []remotestats.Disk
}

var dashboardGreen = ui.RGB(38, 214, 163)
var dashboardBlue = ui.RGB(99, 180, 248)
var dashboardAmber = ui.RGB(239, 180, 64)

func (s *State) dashboardRow(p *monitoring.Panel, id string, width int) *dashboardRow {
	d := &s.dashboard
	if d.rows == nil {
		d.rows = make(map[string]*dashboardRow)
	}
	key := p.ID + "/" + id
	r := d.rows[key]
	if r == nil {
		r = &dashboardRow{}
		for i := range r.chartIDs {
			r.chartIDs[i] = tree.ID(dashboardHits, fmt.Sprintf("chart-%s-%s-%d", p.ID, id, i))
		}
		d.rows[key] = r
	}
	version := r.snapshot.Version
	if width != r.width {
		version = 0
	}
	if snap, changed := d.engine.Read(p.ID, id, max(1, width/8), version); changed {
		r.snapshot = snap
		r.width = width
		r.cpu = "—"
		r.memory = "—"
		r.capacity = ""
		r.down = "—"
		r.up = "—"
		r.updated = ""
		if snap.HasSample {
			if !snap.Sample.CPUUnavailable {
				r.cpu = fmt.Sprintf("%.0f%%", snap.Sample.CPUPercent)
			}
			if snap.Sample.MemoryTotalBytes > 0 {
				r.memory = fmt.Sprintf("%.0f%%", 100*float64(snap.Sample.MemoryUsedBytes)/float64(snap.Sample.MemoryTotalBytes))
				r.capacity = monitorCapacity(snap.Sample.MemoryUsedBytes, snap.Sample.MemoryTotalBytes)
			}
			if !snap.Sample.NetworkUnavailable {
				r.down = dashboardRate(snap.Sample.ReceiveBytesPerSecond)
				r.up = dashboardRate(snap.Sample.SendBytesPerSecond)
			}
			r.updated = snap.Updated.Format("2006-01-02 15:04:05")
		}
		r.disks = r.disks[:0]
		for _, disk := range snap.Details.Disks {
			if !disk.Virtual {
				r.disks = append(r.disks, disk)
			}
		}
		r.x = r.x[:0]
		for i := range r.values {
			r.values[i] = r.values[i][:0]
		}
		if len(snap.Points) > 0 {
			last := snap.Points[len(snap.Points)-1].At
			available := last.Sub(snap.Points[0].At)
			duration := min(time.Duration(p.Retention)*time.Minute, max(time.Minute, time.Duration(p.Interval*2)*time.Second, available))
			r.origin = last.Add(-duration)
			for _, point := range snap.Points {
				r.x = append(r.x, float32(float64(point.At.Sub(r.origin))/float64(duration)))
				for i, v := range point.Values {
					if i == 2 || i == 3 {
						v /= 1024
					}
					r.values[i] = append(r.values[i], v)
				}
			}
			origin := r.origin
			layout := "15:04"
			if duration <= 5*time.Minute {
				layout = "15:04:05"
			}
			if duration >= 24*time.Hour {
				layout = "01-02 15:04"
			}
			r.clock = func(dst []byte, v float32) []byte {
				return origin.Add(time.Duration(float64(v)*float64(duration))).AppendFormat(dst, layout)
			}
		}
	}
	return r
}
func dashboardRate(n uint64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MiB/s", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.0f KiB/s", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B/s", n)
}
func dashboardPercent(dst []byte, v float32) []byte {
	dst = strconv.AppendFloat(dst, float64(v), 'f', 0, 32)
	return append(dst, '%')
}
func dashboardNetworkAxis(dst []byte, v float32) []byte {
	if v >= 1024 {
		dst = strconv.AppendFloat(dst, float64(v/1024), 'f', 0, 32)
		return append(dst, " MiB/s"...)
	}
	dst = strconv.AppendFloat(dst, float64(v), 'f', 0, 32)
	return append(dst, " KiB/s"...)
}

func (s *State) paintDashboard(c *ui.Canvas, b ui.Rect) {
	d, m := &s.dashboard, s.messages.Dashboard
	pal := s.pal
	x, w := b.X+18, b.Width-36
	y := b.Y + 14
	titleW := max(float32(76), c.TextWidthWeight(m.Title, 24, ui.WeightSemibold)+4)
	monitorLabel(c, m.Title, ui.Rect{X: x, Y: y, Width: titleW, Height: 40}, 24, pal.text, ui.WeightSemibold)
	p := s.dashboardPanel()
	compact := w < 720
	buttonW := float32(112)
	if compact {
		buttonW = 36
	}
	newBox := ui.Rect{X: x + w - buttonW, Y: y, Width: buttonW, Height: 36}
	if p != nil {
		newBox.X -= buttonW + 12
	}
	s.paintToolbarButtonIn(c, hitDashboardNew, newBox, m.New, iconPlus, true, true, compact)
	if p == nil {
		monitorLabel(c, m.Empty, ui.Rect{X: x + 20, Y: y + 170, Width: w - 40, Height: 40}, 22, pal.text, ui.WeightSemibold)
		monitorLabel(c, m.EmptyHint, ui.Rect{X: x + 20, Y: y + 215, Width: w - 40, Height: 50}, 14, pal.muted, ui.WeightRegular)
		return
	}
	switchBox := ui.Rect{X: x + titleW + 8, Y: y, Width: min(218, max(120, newBox.X-x-106)), Height: 36}
	components.BeginButton(c, s.Theme(), hitDashboardSwitch, switchBox).Secondary(p.Name + "  ⌄")
	if d.menu == 1 {
		d.anchor = switchBox
	}
	noteX := switchBox.X + switchBox.Width + 20
	monitorLabel(c, p.Note, ui.Rect{X: noteX, Y: y, Width: max(0, newBox.X-noteX-16-dashboardStatusReserve(w)), Height: 36}, 13, pal.muted, ui.WeightRegular)
	running := d.running[p.ID]
	toggle := m.Start
	ic := uint8(iconPlay)
	if running {
		toggle = m.Stop
		ic = iconPause
	}
	s.paintToolbarButtonIn(c, hitDashboardToggle, ui.Rect{X: x + w - buttonW, Y: y, Width: buttonW, Height: 36}, toggle, ic, !running, true, compact)
	y += 58
	controlH := float32(42)
	if w < 860 {
		controlH = 86
	}
	modeW := float32(68)
	for i, label := range []string{m.Simple, m.Detailed} {
		id := hitDashboardSimple
		selected := p.Simple
		if i == 1 {
			id = hitDashboardDetailed
			selected = !p.Simple
		}
		r := ui.Rect{X: x + float32(i)*modeW, Y: y, Width: modeW, Height: 34}
		if selected {
			components.BeginButton(c, s.Theme(), id, r).Secondary(label)
		} else {
			components.BeginButton(c, s.Theme(), id, r).Ghost(label)
		}
	}
	settingsY := y
	searchW := max(100, w-136-16-316)
	if w < 860 {
		settingsY = y + 42
		searchW = w - 152
	}
	searchBox := ui.Rect{X: x + 152, Y: y, Width: searchW, Height: 34}
	components.PaintInputGroup(c, s.Theme(), searchBox, components.InputGroupSnapshot{ID: hitDashboardSearch, Label: m.Search, Placeholder: m.Search, Value: d.search.String(), NativeEditing: c.Focused() == hitDashboardSearch && d.search.String() != "", InlineStart: components.InputGroupSlot{Kind: components.InputGroupSlotIcon, Icon: lucideIcons[iconSearch]}})
	intervalBox := ui.Rect{X: x + w - 304, Y: settingsY, Width: 152, Height: 34}
	retainBox := ui.Rect{X: x + w - 142, Y: settingsY, Width: 142, Height: 34}

	components.BeginButton(c, s.Theme(), hitDashboardRetention, retainBox).Secondary(m.Retention + ": " + s.dashboardRetention(p.Retention) + " ⌄")
	if d.menu == 2 {
		d.anchor = intervalBox
	}
	if d.menu == 3 {
		d.anchor = retainBox
	}
	y += controlH + 10
	c.Fill(ui.Rect{X: x, Y: y, Width: w, Height: 1}, pal.line)
	y += 8
	footer := ui.Rect{X: x, Y: b.Y + b.Height - 36, Width: w, Height: 34}
	d.body = ui.Rect{X: x, Y: y, Width: w, Height: max(0, footer.Y-y-5)}

	width := int(d.body.Width)
	rowW := d.body.Width
	if p.Simple {
		rowW = max(rowW, 960)
	}
	ids := make([]string, 0, len(p.Hosts))
	live := 0
	query := strings.ToLower(strings.TrimSpace(d.search.String()))
	for _, id := range p.Hosts {
		r := s.dashboardRow(p, id, width)
		h, exists := s.dashboardHost(id)
		if !exists {
			r.snapshot.Status = monitoring.Missing
		}
		if r.snapshot.Status == monitoring.Live && running {
			live++
		}
		if query == "" || strings.Contains(strings.ToLower(h.Label+" "+h.Address+" "+h.Tags+" "+h.Group+" "+id), query) {
			ids = append(ids, id)
		}
	}
	d.content = 0
	if p.Simple {
		d.content = 38
		for _, id := range ids {
			d.content += dashboardSimpleHeight(s.dashboardRow(p, id, width))
		}
	} else {
		for _, id := range ids {
			d.content += dashboardHostHeight(s.dashboardRow(p, id, width), rowW-12)
		}
	}
	area := components.BeginScrollArea(c, s.Theme(), d.body, ui.Size{Width: rowW, Height: d.content}).Offset(d.scrollX, d.scroll).Viewport(hitDashboardScroll, m.Title).VerticalScrollbar(hitDashboardThumb, m.Title, "").HorizontalScrollbar(hitDashboardXThumb, m.Title, "")
	geo := area.BeginContent()
	d.scroll = geo.OffsetY
	d.scrollX = geo.OffsetX
	y = geo.Content.Y
	if p.Simple {
		s.paintDashboardSimpleHead(c, ui.Rect{X: geo.Content.X, Y: y, Width: geo.Content.Width - 12, Height: 36})
		y += 38
	}
	for _, id := range ids {
		r := s.dashboardRow(p, id, width)
		host, exists := s.dashboardHost(id)
		if !exists {
			host.Label = m.Missing
			host.CatalogID = id
		}
		h := dashboardSimpleHeight(r)
		if !p.Simple {
			h = dashboardHostHeight(r, rowW-12)
		}
		box := ui.Rect{X: geo.Content.X, Y: y, Width: geo.Content.Width - 12, Height: h}
		if box.Y+box.Height > d.body.Y && box.Y < d.body.Y+d.body.Height {
			if p.Simple {
				s.paintDashboardSimpleRow(c, p, host, r, box)
			} else {
				s.paintDashboardHost(c, p, host, r, box)
			}
		}
		y += h
	}
	if len(ids) == 0 {
		monitorLabel(c, s.messages.KnownHostsNoMatches, ui.Rect{X: geo.Content.X + 18, Y: y + 30, Width: rowW - 36, Height: 40}, 14, pal.muted, ui.WeightRegular)
	}
	area.EndContent()
	d.refresh.interval = p.Interval
	d.refresh.intervalText = fmt.Sprintf(m.Every, p.Interval)
	d.refresh.paused = !running
	d.refresh.count = 0
	for _, id := range p.Hosts {
		r := s.dashboardRow(p, id, width)
		if r.snapshot.Status == monitoring.Live {
			d.refresh.count = 1
			d.refresh.version = uint32(r.snapshot.Updated.UnixMilli())
			break
		}
	}
	s.paintIntervalClock(c, &d.refresh, intervalBox, running && live > 0, hitDashboardInterval, m.Sampling, d.menu == 2)
	if w >= 1000 {
		monitorRight(c, fmt.Sprintf(m.Collecting, live, len(p.Hosts)), ui.Rect{X: newBox.X - 174, Y: b.Y + 14, Width: 160, Height: 36}, 12, s.dashboardLiveColor(running), ui.WeightMedium)
	}
	c.Fill(ui.Rect{X: x, Y: footer.Y, Width: w, Height: 1}, pal.line)
	monitorLabel(c, fmt.Sprintf(m.Count, len(p.Hosts)), footer, 12, pal.muted, ui.WeightRegular)
	stateText := m.Waiting
	if running {
		stateText = fmt.Sprintf(m.Collecting, live, len(p.Hosts))
	} else {
		for _, id := range p.Hosts {
			if s.dashboardRow(p, id, width).snapshot.HasSample {
				stateText = m.Stopped
				break
			}
		}
	}
	monitorRight(c, stateText, footer, 12, pal.muted, ui.WeightRegular)
}

func dashboardBar(c *ui.Canvas, b ui.Rect, value float64, color, track ui.Color) {
	c.Rounded(b, 3, track)
	if !math.IsNaN(value) && value > 0 {
		b.Width *= float32(min(value, 100) / 100)
		c.Rounded(b, 3, color)
	}
}
func (s *State) paintDashboardIdentity(c *ui.Canvas, h Host, b ui.Rect) {
	s.paintHostCatalogIcon(c, h, ui.Rect{X: b.X, Y: b.Y + 9, Width: 34, Height: 34}, s.pal.workspace)
	monitorLabel(c, h.Label, ui.Rect{X: b.X + 46, Y: b.Y + 3, Width: b.Width - 50, Height: 27}, 16, s.pal.text, ui.WeightSemibold)
	monitorLabel(c, h.Address, ui.Rect{X: b.X + 46, Y: b.Y + 30, Width: b.Width - 50, Height: 22}, 12, s.pal.muted, ui.WeightRegular)
}
func (s *State) paintDashboardState(c *ui.Canvas, p *monitoring.Panel, h Host, r *dashboardRow, b ui.Rect) {
	dashboardGreen, _, _ := s.dashboardColors()
	running := s.dashboard.running[p.ID]
	label := s.dashboardStatus(r.snapshot.Status, running, r.snapshot.HasSample)
	color := s.pal.muted
	if running && r.snapshot.Status == monitoring.Live {
		color = dashboardGreen
	}
	if running && r.snapshot.Status >= monitoring.Failed {
		color = s.pal.warning
	}
	monitorLabel(c, label, b, 12, color, ui.WeightMedium)
	if running && r.snapshot.Status == monitoring.Failed && h.ID != 0 {
		c.HitWithOptions(hitDashboardRetryList.At(int(h.ID)), b, s.messages.Dashboard.Retry, ui.HitOptions{Role: ui.RoleButton})
	}
}
func dashboardHostHeight(r *dashboardRow, w float32) float32 {
	header := dashboardHeadingHeight(w)
	if r.page == 1 {
		return header + 178 + float32(max(1, len(r.snapshot.Details.Disks)))*38 + 44
	}
	if r.page == 2 {
		return header + 178 + float32(max(1, len(r.snapshot.Details.Networks)))*38 + 44
	}
	cols := 2
	if w < 760 {
		cols = 1
	}
	charts := float32(164)
	if w < 760 {
		charts = 3*164 + 16
	}
	return header + charts + float32(max(1, (len(r.disks)+cols-1)/cols))*48 + 18
}

func (s *State) paintDashboardHost(c *ui.Canvas, p *monitoring.Panel, h Host, r *dashboardRow, b ui.Rect) {
	_, _, dashboardAmber := s.dashboardColors()
	m, pal := s.messages.Dashboard, s.pal
	header := s.paintDashboardHeading(c, p, h, r, b)
	if r.page != 0 {
		s.paintDashboardDeviceView(c, p, h, r, ui.Rect{X: b.X, Y: b.Y + header, Width: b.Width, Height: b.Height - header})
		return
	}
	y := b.Y + header
	chartW := (b.Width - 16) / 3
	vertical := b.Width < 760
	if vertical {
		chartW = b.Width
	}
	for i, label := range []string{m.CPU, m.Memory, m.Network} {
		box := ui.Rect{X: b.X + float32(i)*(chartW+8), Y: y, Width: chartW, Height: 156}
		if vertical {
			box.X = b.X
			box.Y = y + float32(i)*172
		}
		c.RoundedBorder(box, 7, 1, pal.line, pal.workspace)
		c.DrawIcon([]ui.Icon{icons.Cpu, icons.MemoryStick, icons.Wifi}[i], ui.Rect{X: box.X + 12, Y: box.Y + 13, Width: 14, Height: 14}, pal.muted)
		monitorLabel(c, label, ui.Rect{X: box.X + 34, Y: box.Y + 6, Width: 74, Height: 28}, 13, pal.text, ui.WeightMedium)
		value := r.cpu
		if i == 1 {
			value = r.memory
		}
		if i == 2 {
			value = "↓ " + r.down + "   ↑ " + r.up
		}
		font := float32(20)
		if i == 2 {
			font = 12
		}
		monitorRight(c, value, ui.Rect{X: box.X + 85, Y: box.Y + 6, Width: max(0, box.Width-97), Height: 28}, font, pal.text, ui.WeightSemibold)
		plot := ui.Rect{X: box.X + 4, Y: box.Y + 39, Width: box.Width - 8, Height: box.Height - 43}
		s.paintDashboardChart(c, p, h, r, plot, i, label)
	}
	y += 164
	if vertical {
		y += 344
	}
	cols := 2
	if vertical {
		cols = 1
	}
	diskW := (b.Width - float32(cols-1)*8) / float32(cols)
	if len(r.disks) == 0 || r.snapshot.DetailsFailed {
		monitorLabel(c, s.dashboardDiskMessage(r), ui.Rect{X: b.X + 12, Y: y, Width: b.Width - 24, Height: 40}, 12, pal.muted, ui.WeightRegular)
	} else {
		for i, disk := range r.disks {
			box := ui.Rect{X: b.X + float32(i%cols)*(diskW+8), Y: y + float32(i/cols)*48, Width: diskW, Height: 40}
			c.RoundedBorder(box, 6, 1, pal.line, pal.workspace)
			c.DrawIcon(icons.HardDrive, ui.Rect{X: box.X + 10, Y: box.Y + 13, Width: 14, Height: 14}, dashboardAmber)
			monitorLabel(c, disk.Path, ui.Rect{X: box.X + 32, Y: box.Y, Width: max(0, box.Width*.18-24), Height: 40}, 12, pal.text, ui.WeightMedium)
			monitorLabel(c, fmt.Sprintf("%.0f%%", disk.Percent), ui.Rect{X: box.X + box.Width*.2, Y: box.Y, Width: 52, Height: 40}, 16, pal.text, ui.WeightSemibold)
			dashboardBar(c, ui.Rect{X: box.X + box.Width*.35, Y: box.Y + 17, Width: box.Width * .3, Height: 6}, disk.Percent, dashboardAmber, pal.line)
			monitorRight(c, monitorCapacity(disk.Used, disk.Total), ui.Rect{X: box.X + box.Width*.67, Y: box.Y, Width: box.Width*.31 - 8, Height: 40}, 11, pal.muted, ui.WeightRegular)
		}
	}
	c.Fill(ui.Rect{X: b.X, Y: b.Y + b.Height - 6, Width: b.Width, Height: 1}, pal.line)
}
func (s *State) paintDashboardChart(c *ui.Canvas, p *monitoring.Panel, h Host, r *dashboardRow, b ui.Rect, metric int, label string) {
	dashboardGreen, dashboardBlue, dashboardAmber := s.dashboardColors()
	if len(r.x) < 2 {
		monitorLabel(c, s.dashboardStatus(r.snapshot.Status, s.dashboard.running[p.ID], r.snapshot.HasSample), b, 12, s.pal.muted, ui.WeightRegular)
		return
	}
	cache := &r.charts[metric]
	color := dashboardGreen
	valueIndex := metric
	if metric == 1 {
		color = dashboardBlue
	}
	if metric == 3 {
		color = dashboardAmber
		valueIndex = 4
	}
	count := 1
	if metric == 2 {
		count = 2
	}
	for i := 0; i < count; i++ {
		tint := color
		name := label
		if metric == 2 {
			name = s.messages.Dashboard.Receive
			if i == 1 {
				tint = dashboardBlue
				name = s.messages.Dashboard.Send
			}
		}
		cache.gradient[i] = components.ChartAreaGradient{From: tint, To: tint}
		cache.gradient[i].To.Alpha = 0
		cache.style[i] = components.ChartAreaStyle{Opacity: .22, Gradient: &cache.gradient[i]}
		cache.series[i] = components.ChartSeries{Key: name, Label: name, Color: tint, AreaStyle: &cache.style[i]}
		cache.values[i] = r.values[valueIndex+i]
	}
	yaxis := components.ChartAxis{Fixed: true, Min: 0, Max: 100, Format: dashboardPercent}
	if metric == 2 {
		yaxis = components.ChartAxis{Format: dashboardNetworkAxis}
	}
	theme := s.Theme()
	theme.Border = ui.Color{}
	components.PaintAreaChart(c, theme, b, r.chartIDs[metric], components.AreaChart{Series: cache.series[:count], X: r.x, Values: cache.values[:count], Curve: components.ChartCurveMonotone, Fill: components.ChartFillGradient, StrokeWidth: 1.5, XAxis: components.ChartAxis{Linear: true, Fixed: true, Min: 0, Max: 1, Ticks: 3, Format: r.clock}, YAxis: yaxis, Label: label, Version: uint32(r.snapshot.Version), HideTooltip: true}, &cache.state)
}
func (s *State) paintDashboardMenu(c *ui.Canvas) {
	d := &s.dashboard
	if d.menu == 0 || s.activeSection != sectionDashboard || s.modal != modalNone {
		return
	}
	m := s.messages.Dashboard
	b := d.anchor
	count := len(d.panels) + 2
	if d.menu == 2 {
		count = len(dashboardIntervals)
	}
	if d.menu == 3 {
		count = len(dashboardRetentions)
	}
	menu := components.BeginDropdownMenu(c, s.Theme(), ui.Rect{X: b.X, Y: b.Y + b.Height + 4, Width: max(210, b.Width), Height: min(420, float32(count*34+12))}, components.DropdownMenuOptions{Layer: popupLayer, Scope: hitDashboardDismiss, DismissID: hitDashboardDismiss, Viewport: s.windowRect(), Scroll: s.selectScroll(true, hitDashboardDismiss)})
	if d.menu == 1 {
		for i, p := range d.panels {
			menu.AddItem(components.DropdownMenuItem{ID: hitDashboardPanelList.At(i), Label: p.Name, Kind: components.DropdownMenuRadio, Selected: p.ID == d.active})
		}
		menu.AddItem(components.DropdownMenuItem{ID: hitDashboardEdit, Label: m.Edit})
		menu.AddItem(components.DropdownMenuItem{ID: hitDashboardDelete, Label: m.Delete})
	}
	if p := s.dashboardPanel(); p != nil {
		if d.menu == 2 {
			for i, n := range dashboardIntervals {
				menu.AddItem(components.DropdownMenuItem{ID: hitDashboardIntervalList.At(i), Label: fmt.Sprintf(m.Every, n), Kind: components.DropdownMenuRadio, Selected: n == p.Interval})
			}
		}
		if d.menu == 3 {
			for i, n := range dashboardRetentions {
				menu.AddItem(components.DropdownMenuItem{ID: hitDashboardRetentionList.At(i), Label: s.dashboardRetention(n), Kind: components.DropdownMenuRadio, Selected: n == p.Retention})
			}
		}
	}
	s.menuPlaced(menu.Bounds())
	menu.End()
}

func (s *State) paintDashboardDialog(c *ui.Canvas, viewport ui.Rect) {
	e, m := &s.dashboard.editor, s.messages.Dashboard
	theme := s.Theme()
	title := m.New
	submit := m.Create
	if e.id != "" {
		title = m.Edit
		submit = m.Save
	}
	dialog := components.BeginDialog(c, theme, viewport, components.DialogOptions{Layer: dialogLayer, Scope: dialogScope, DismissID: hitDashboardDismiss, CloseID: hitDashboardDialogCancel, InitialFocusID: hitDashboardName, ContentSize: ui.Size{Width: 620}, FitBody: true, BodyHeight: 500})
	layout := dialog.Begin(title, "")
	b := layout.Body
	y := b.Y + 4
	for i, field := range []struct {
		id    ui.ID
		label string
		value string
	}{{hitDashboardName, m.Name, e.name.String()}, {hitDashboardNote, m.Note, e.note.String()}, {hitDashboardHostQuery, m.HostSearch, e.query.String()}} {
		box := components.BeginField(c, theme, ui.Rect{X: b.X + 4, Y: y, Width: b.Width - 8, Height: 62}).Paint(field.label, "", "")
		components.PaintInput(c, theme, box, components.InputSnapshot{ID: field.id, Label: field.label, Value: field.value, Focused: c.Focused() == field.id, PlatformEditing: c.Focused() == field.id && field.value != ""})
		y += 64
		if i == 1 {
			monitorLabel(c, fmt.Sprintf("%s · %d", m.Hosts, len(e.selected)), ui.Rect{X: b.X + 4, Y: y, Width: b.Width - 8, Height: 28}, 13, s.pal.text, ui.WeightMedium)
			y += 28
		}
	}
	query := strings.ToLower(strings.TrimSpace(e.query.String()))
	hosts := make([]Host, 0, len(s.hosts))
	for _, h := range s.hosts {
		if h.isSSH() && h.ID != localHostID && (query == "" || strings.Contains(strings.ToLower(h.Label+" "+h.Address+" "+h.Group+" "+h.Tags), query)) {
			hosts = append(hosts, h)
		}
	}
	e.bounds = ui.Rect{X: b.X + 4, Y: y + 4, Width: b.Width - 8, Height: max(60, b.Y+b.Height-y-64)}
	missing := []int{}
	for i, id := range e.selected {
		if _, ok := s.dashboardHost(id); !ok && (query == "" || strings.Contains(strings.ToLower(id), query)) {
			missing = append(missing, i)
		}
	}
	e.content = float32(len(hosts)+len(missing)) * 46
	area := components.BeginScrollArea(c, theme, e.bounds, ui.Size{Width: e.bounds.Width, Height: e.content}).Offset(0, e.scroll).Viewport(hitDashboardHostScroll, m.Hosts).VerticalScrollbar(hitDashboardHostThumb, m.Hosts, "")
	geo := area.BeginContent()
	e.scroll = geo.OffsetY
	for i, h := range hosts {
		box := ui.Rect{X: geo.Content.X + 4, Y: geo.Content.Y + float32(i)*46, Width: geo.Viewport.Width - 16, Height: 45}
		if box.Y+box.Height < e.bounds.Y || box.Y > e.bounds.Y+e.bounds.Height {
			continue
		}
		id := hitDashboardHostList.At(int(h.ID))
		side := components.CheckboxExtent(theme)
		components.BeginCheckbox(c, theme, id, ui.Rect{X: box.X, Y: box.Y + (45-side)/2}, slices.Contains(e.selected, h.CatalogID)).Label(h.Label).Paint()
		monitorLabel(c, h.Label, ui.Rect{X: box.X + 30, Y: box.Y, Width: box.Width - 35, Height: 24}, 13, s.pal.text, ui.WeightMedium)
		monitorLabel(c, h.Address+" · "+h.Group, ui.Rect{X: box.X + 30, Y: box.Y + 23, Width: box.Width - 35, Height: 20}, 11, s.pal.muted, ui.WeightRegular)
		c.HitLabel(id, ui.Rect{X: box.X + 26, Y: box.Y, Width: box.Width - 26, Height: 45}, h.Label, false)
	}
	for row, index := range missing {
		box := ui.Rect{X: geo.Content.X + 4, Y: geo.Content.Y + float32(len(hosts)+row)*46, Width: geo.Viewport.Width - 16, Height: 45}
		id := hitDashboardMissingList.At(index)
		components.BeginCheckbox(c, theme, id, ui.Rect{X: box.X, Y: box.Y + 12}, true).Label(m.Missing).Paint()
		monitorLabel(c, m.Missing+" · "+e.selected[index], ui.Rect{X: box.X + 30, Y: box.Y, Width: box.Width - 35, Height: 45}, 12, s.pal.warning, ui.WeightRegular)
		c.HitLabel(id, box, m.Missing, false)
	}
	area.EndContent()
	hint := m.PickerHint
	color := s.pal.muted
	if e.error != "" {
		hint = e.error
		color = s.pal.danger
	}
	monitorLabel(c, hint, ui.Rect{X: b.X + 4, Y: b.Y + b.Height - 48, Width: b.Width - 8, Height: 44}, 12, color, ui.WeightRegular)
	dialog.Actions(layout, components.ButtonAction{ID: hitDashboardDialogCancel, Label: s.messages.Cancel, Variant: components.ButtonSecondary}, components.ButtonAction{ID: hitDashboardDialogSave, Label: submit})
	dialog.End()
}

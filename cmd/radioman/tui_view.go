package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	radiobrowser "github.com/erbekin/radioman"
)

// ---------------------------------------------------------------------------
// Palette & styles
// ---------------------------------------------------------------------------

var (
	colAccent  = lipgloss.Color("#00E5FF")
	colText    = lipgloss.Color("#CDD6F4")
	colSubtext = lipgloss.Color("#A6ADC8")
	colDim     = lipgloss.Color("#6C7086")
	colBorder  = lipgloss.Color("#45475A")
	colSurface = lipgloss.Color("#313244")
	colMantle  = lipgloss.Color("#181825")
	colCrust   = lipgloss.Color("#11111B")
	colGreen   = lipgloss.Color("#A6E3A1")
	colYellow  = lipgloss.Color("#F9E2AF")
	colRed     = lipgloss.Color("#F38BA8")
	colBlue    = lipgloss.Color("#89B4FA")
)

var (
	appTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colCrust).Background(colAccent).Padding(0, 1)

	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Background(colSurface).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(colDim).Padding(0, 1)

	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	textStyle         = lipgloss.NewStyle().Foreground(colText)
	subtextStyle      = lipgloss.NewStyle().Foreground(colSubtext)
	dimStyle          = lipgloss.NewStyle().Foreground(colDim)
	labelStyle        = lipgloss.NewStyle().Bold(true).Foreground(colBlue)
	keyStyle          = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	playingBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(colGreen)
	favBadgeStyle     = lipgloss.NewStyle().Bold(true).Foreground(colYellow)
	errorStyle        = lipgloss.NewStyle().Bold(true).Foreground(colRed)
	statusStyle       = lipgloss.NewStyle().Foreground(colYellow)

	pillStyle       = lipgloss.NewStyle().Foreground(colSubtext).Background(colSurface).Padding(0, 1)
	pillActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(colCrust).Background(colAccent).Padding(0, 1)
)

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

func (m model) View() string {
	sections := []string{m.renderHeader()}
	if m.showSearchBar() {
		sections = append(sections, m.renderSearchBar())
	}
	sections = append(sections,
		m.renderMainContent(m.listHeight()),
		m.renderNowPlaying(),
		m.renderFooter(),
	)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m model) showSearchBar() bool {
	return m.activeTab == tabSearch || m.searching
}

// listHeight is the number of station rows that fit between the fixed chrome
// (header, search bar, panel borders, now-playing bar and footer).
func (m model) listHeight() int {
	chrome := 1 + 2 + 1 + lipgloss.Height(m.renderFooter())
	if m.showSearchBar() {
		chrome += lipgloss.Height(m.renderSearchBar())
	}
	return max(3, m.height-chrome)
}

func (m *model) ensureCursorVisible() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m model) totalWidth() int {
	return max(40, m.width)
}

// renderPanel draws a rounded box of the given outer size with a title (left)
// and info text (right) embedded in the top border.
func renderPanel(title, info, body string, width, height int, focused bool) string {
	borderColor := colBorder
	titleSt := lipgloss.NewStyle().Bold(true).Foreground(colSubtext)
	if focused {
		borderColor = colAccent
		titleSt = titleSt.Foreground(colAccent)
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)
	b := lipgloss.RoundedBorder()

	innerW := max(1, width-2)
	contentW := max(1, innerW-2)
	contentH := max(1, height-2)

	body = lipgloss.NewStyle().MaxWidth(contentW).MaxHeight(contentH).Render(body)
	box := lipgloss.NewStyle().
		Border(b, false, true, true, true).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(innerW).
		Height(contentH).
		Render(body)

	t, i := "", ""
	if title != "" {
		t = titleSt.Render(" " + title + " ")
	}
	if info != "" {
		i = dimStyle.Render(" " + info + " ")
	}
	fill := innerW - 2 - lipgloss.Width(t) - lipgloss.Width(i)
	if fill < 0 {
		i = ""
		fill = innerW - 2 - lipgloss.Width(t)
	}
	if fill < 0 {
		t = ""
		fill = max(0, innerW-2)
	}
	top := bc.Render(b.TopLeft+b.Top) + t + bc.Render(strings.Repeat(b.Top, fill)) + i + bc.Render(b.Top+b.TopRight)
	return top + "\n" + box
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

func (m model) renderHeader() string {
	tabs := []struct {
		idx   tabIndex
		label string
	}{
		{tabTop, "1 Top Stations"},
		{tabSearch, "2 Search"},
		{tabFavorites, fmt.Sprintf("3 Favorites (%d)", m.favorites.Len())},
	}

	parts := []string{appTitleStyle.Render("📻 RADIOMAN"), " "}
	for _, t := range tabs {
		if m.activeTab == t.idx {
			parts = append(parts, activeTabStyle.Render(t.label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(t.label))
		}
	}
	return lipgloss.NewStyle().MaxWidth(m.totalWidth()).Render(strings.Join(parts, ""))
}

// ---------------------------------------------------------------------------
// Search bar
// ---------------------------------------------------------------------------

func (m model) renderSearchBar() string {
	if !m.searching {
		return m.renderSearchSummary()
	}

	w := m.totalWidth()
	contentW := w - 4
	const gap = "   "

	fields := []struct {
		label string
		field searchField
		input textinput.Model
	}{
		{"Name", fieldName, m.nameInput},
		{"Country", fieldCountry, m.countryInput},
		{"Tag", fieldTag, m.tagInput},
	}

	fixed := len(gap) * (len(fields) - 1)
	for _, f := range fields {
		fixed += lipgloss.Width(f.label) + 2 + 1 // pill padding + space
	}
	inputW := max(6, (contentW-fixed)/len(fields))

	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		pill := pillStyle
		if m.activeField == f.field {
			pill = pillActiveStyle
		}
		in := f.input
		in.Width = inputW - 1 // leave a cell for the cursor
		v := lipgloss.NewStyle().MaxWidth(inputW).Render(in.View())
		v += strings.Repeat(" ", max(0, inputW-lipgloss.Width(v)))
		parts = append(parts, pill.Render(f.label)+" "+v)
	}

	return renderPanel("Search", "", strings.Join(parts, gap), w, 3, true)
}

// renderSearchSummary is the compact, single-line view of the active filters
// shown on the Search tab while the filters are not being edited.
func (m model) renderSearchSummary() string {
	var chips []string
	add := func(label, val string) {
		if val = strings.TrimSpace(val); val != "" {
			chips = append(chips, dimStyle.Render(label+" ")+textStyle.Bold(true).Render(val))
		}
	}
	add("name", m.nameInput.Value())
	add("country", m.countryInput.Value())
	add("tag", m.tagInput.Value())

	filters := dimStyle.Render("no filters")
	if len(chips) > 0 {
		filters = strings.Join(chips, dimStyle.Render("  ·  "))
	}
	line := " " + labelStyle.Render("Filters") + "  " + filters + "   " + keyStyle.Render("/") + dimStyle.Render(" edit")
	return lipgloss.NewStyle().MaxWidth(m.totalWidth()).Render(line)
}

// ---------------------------------------------------------------------------
// Main content
// ---------------------------------------------------------------------------

func (m model) listTitle() (string, string) {
	var title string
	switch m.activeTab {
	case tabTop:
		title = "Top Stations"
	case tabSearch:
		title = "Results"
	case tabFavorites:
		title = "Favorites"
	}
	info := ""
	if m.loading {
		info = "loading…"
	} else if len(m.stations) > 0 {
		info = fmt.Sprintf("%d/%d", min(m.cursor+1, len(m.stations)), len(m.stations))
	}
	return title, info
}

func (m model) renderMainContent(listH int) string {
	total := m.totalWidth()
	panelH := listH + 2
	focused := !m.searching && !m.editingSname
	title, info := m.listTitle()

	// Narrow terminals: list only.
	if total < 72 {
		return renderPanel(title, info, m.renderStationList(total-4, listH), total, panelH, focused)
	}

	listW := total * 58 / 100
	detailW := total - listW - 1

	list := renderPanel(title, info, m.renderStationList(listW-4, listH), listW, panelH, focused)
	detail := renderPanel("Details", "", m.renderStationDetails(detailW-4), detailW, panelH, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", detail)
}

func (m model) renderStationList(width, height int) string {
	if m.loading {
		return "\n " + m.spinner.View() + dimStyle.Render("Fetching stations from RadioBrowser…")
	}

	if len(m.stations) == 0 {
		if m.activeTab == tabFavorites {
			return "\n " + subtextStyle.Render("No favorite stations yet.") + "\n " +
				dimStyle.Render("Press ") + keyStyle.Render("f") + dimStyle.Render(" on any station to save it here.")
		}
		return "\n " + subtextStyle.Render("No stations found.") + "\n " +
			dimStyle.Render("Press ") + keyStyle.Render("/") + dimStyle.Render(" to adjust the search filters.")
	}

	offset := m.offset
	if m.cursor >= offset+height {
		offset = m.cursor - height + 1
	}
	if m.cursor < offset {
		offset = m.cursor
	}
	offset = max(0, offset)
	end := min(len(m.stations), offset+height)
	numW := len(fmt.Sprint(len(m.stations)))

	rows := make([]string, 0, end-offset)
	for i := offset; i < end; i++ {
		rows = append(rows, m.renderStationRow(i, width, numW))
	}
	return strings.Join(rows, "\n")
}

func stationMeta(st radiobrowser.Station) string {
	var meta []string
	if st.CountryCode != "" {
		meta = append(meta, st.CountryCode)
	} else if st.Country != "" {
		meta = append(meta, truncate(st.Country, 10))
	}
	if st.Codec != "" {
		if st.Bitrate > 0 {
			meta = append(meta, fmt.Sprintf("%s %dk", st.Codec, st.Bitrate))
		} else {
			meta = append(meta, st.Codec)
		}
	}
	return strings.Join(meta, " · ")
}

func (m model) renderStationRow(i, width, numW int) string {
	st := m.stations[i]
	selected := i == m.cursor
	playing := m.player.IsPlaying(st)

	// Every segment carries the row background so the highlight is continuous.
	bg := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(colSurface)
		}
		return s
	}
	sp := func(n int) string { return bg(lipgloss.NewStyle()).Render(strings.Repeat(" ", max(0, n))) }

	marker := sp(1)
	if selected {
		marker = bg(lipgloss.NewStyle().Foreground(colAccent)).Render("▌")
	}
	playIcon, favIcon := " ", " "
	if playing {
		playIcon = "▶"
	}
	if m.favorites.IsFavorite(st) {
		favIcon = "★"
	}

	idx := fmt.Sprintf("%*d", numW, i+1)
	sname := ""
	if m.activeTab == tabFavorites {
		if fav, ok := m.favorites.GetByStation(st); ok {
			idx = fmt.Sprintf("#%-*d", numW, fav.ID)
			sname = fav.ShortName
		}
	}

	prefix := marker + sp(1) + bg(playingBadgeStyle).Render(playIcon) + bg(favBadgeStyle).Render(favIcon) +
		sp(1) + bg(dimStyle).Render(idx) + sp(1)
	if sname != "" {
		prefix += bg(lipgloss.NewStyle().Foreground(colYellow)).Render(sname) + sp(1)
	}

	avail := width - lipgloss.Width(prefix)
	meta := stationMeta(st)
	metaW := lipgloss.Width(meta)
	if metaW+12 > avail {
		meta, metaW = "", 0
	}
	nameMax := avail - metaW
	if metaW > 0 {
		nameMax--
	}
	name := truncate(strings.TrimSpace(st.Name), max(1, nameMax))

	nameStyle := textStyle
	switch {
	case selected:
		nameStyle = nameStyle.Bold(true).Foreground(colAccent)
	case playing:
		nameStyle = nameStyle.Foreground(colGreen)
	}

	gap := avail - lipgloss.Width(name) - metaW
	return prefix + bg(nameStyle).Render(name) + sp(gap) + bg(dimStyle).Render(meta)
}

func (m model) renderStationDetails(width int) string {
	var lines []string

	if len(m.stations) == 0 || m.cursor >= len(m.stations) {
		lines = append(lines, dimStyle.Render("Select a station to see its details."))
	} else {
		st := m.stations[m.cursor]
		lines = append(lines, titleStyle.Render(truncate(strings.TrimSpace(st.Name), width)))

		var badges []string
		if m.player.IsPlaying(st) {
			badges = append(badges, playingBadgeStyle.Render("▶ playing"))
		}
		if fav, ok := m.favorites.GetByStation(st); ok {
			badges = append(badges, favBadgeStyle.Render(fmt.Sprintf("★ #%d %s", fav.ID, fav.ShortName)))
		}
		if len(badges) > 0 {
			lines = append(lines, strings.Join(badges, "  "))
		}
		lines = append(lines, "")

		const labelW = 9
		addField := func(label, val string) {
			if strings.TrimSpace(val) == "" {
				return
			}
			lines = append(lines, labelStyle.Width(labelW).Render(label)+" "+
				textStyle.Render(truncate(val, max(4, width-labelW-1))))
		}

		country := st.Country
		if st.CountryCode != "" {
			country = fmt.Sprintf("%s (%s)", st.Country, st.CountryCode)
		}
		addField("Country", country)
		addField("State", st.State)
		addField("Language", st.Language)

		codecInfo := st.Codec
		if st.Bitrate > 0 {
			codecInfo = fmt.Sprintf("%s · %d kbps", st.Codec, st.Bitrate)
		}
		addField("Audio", codecInfo)
		addField("Tags", strings.ReplaceAll(st.Tags, ",", ", "))
		addField("Votes", fmt.Sprintf("%d · %d clicks", st.Votes, st.ClickCount))
		addField("Homepage", st.Homepage)
		if streamURL, err := st.StreamURL(); err == nil {
			addField("Stream", streamURL)
		}
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Now playing bar
// ---------------------------------------------------------------------------

func (m model) renderNowPlaying() string {
	w := m.totalWidth()
	on := func(s lipgloss.Style) lipgloss.Style { return s.Background(colMantle) }

	cur := m.player.CurrentStation()
	var left string
	if cur == nil {
		left = on(dimStyle).Render(" ⏹  Stopped")
	} else {
		left = on(playingBadgeStyle).Render(" ▶  ") + on(textStyle.Bold(true)).Render(truncate(cur.Name, 200))
		if track := strings.TrimSpace(m.player.StreamTitle()); track != "" {
			left += on(dimStyle).Render("  ♪ ") + on(subtextStyle).Render(truncate(track, 200))
		}
	}

	var meta []string
	if cur != nil && cur.Codec != "" {
		if cur.Bitrate > 0 {
			meta = append(meta, fmt.Sprintf("%s %dk", cur.Codec, cur.Bitrate))
		} else {
			meta = append(meta, cur.Codec)
		}
	}
	meta = append(meta, m.player.BackendName())
	if cur != nil {
		if stopAt := m.player.StopAt(); !stopAt.IsZero() {
			if rem := time.Until(stopAt); rem > 0 {
				meta = append(meta, "stop "+FormatRemaining(rem))
			}
		}
	}
	right := on(dimStyle).Render(strings.Join(meta, " · ") + " · bg-exit ")
	if m.settings != nil && m.settings.ContinueOnExit {
		right += on(playingBadgeStyle).Render("on ")
	} else {
		right += on(dimStyle).Render("off ")
	}

	left = lipgloss.NewStyle().MaxWidth(max(0, w-lipgloss.Width(right)-1)).Render(left)
	gap := max(0, w-lipgloss.Width(left)-lipgloss.Width(right))
	return left + on(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap)) + right
}

// ---------------------------------------------------------------------------
// Footer: status + context-aware key hints
// ---------------------------------------------------------------------------

type keyHint struct{ key, desc string }

func renderHint(h keyHint) string {
	return keyStyle.Render(h.key) + " " + dimStyle.Render(h.desc)
}

// renderHints fits as many hints as possible into maxW, always keeping tail.
func renderHints(hints []keyHint, tail keyHint, maxW int) string {
	const sep = "  "
	tailStr := renderHint(tail)
	budget := maxW - lipgloss.Width(tailStr)
	out := ""
	for _, h := range hints {
		s := renderHint(h) + sep
		if lipgloss.Width(out)+lipgloss.Width(s) > budget {
			break
		}
		out += s
	}
	return out + tailStr
}

func (m model) statusLine() string {
	switch {
	case m.errMsg != "":
		return errorStyle.Render("⚠ " + truncate(m.errMsg, 300))
	case m.statusMsg != "":
		return statusStyle.Render(truncate(m.statusMsg, 300))
	default:
		return dimStyle.Render("Ready")
	}
}

func (m model) renderFooter() string {
	w := m.totalWidth()

	var left string
	var hints []keyHint
	tail := keyHint{"?", "help"}

	switch {
	case m.editingSname:
		left = pillActiveStyle.Render("Short name") + " " + m.snameInput.View()
		if m.errMsg != "" {
			left += "  " + errorStyle.Render("⚠ "+truncate(m.errMsg, 200))
		}
		hints = []keyHint{{"↵", "save"}}
		tail = keyHint{"esc", "cancel"}
	case m.searching:
		left = m.statusLine()
		hints = []keyHint{{"tab", "next field"}, {"↵", "search"}}
		tail = keyHint{"esc", "close"}
	case m.showHelp:
		return m.renderHelp()
	default:
		left = m.statusLine()
		hints = []keyHint{{"↵", "play/stop"}, {"/", "search"}, {"f", "favorite"}, {"q", "quit"}}
	}

	right := renderHints(hints, tail, w*3/5)
	left = lipgloss.NewStyle().MaxWidth(max(0, w-lipgloss.Width(right)-4)).Render(left)
	gap := max(1, w-2-lipgloss.Width(left)-lipgloss.Width(right))
	return " " + left + strings.Repeat(" ", gap) + right
}

func (m model) renderHelp() string {
	bgExit := "off"
	if m.settings != nil && m.settings.ContinueOnExit {
		bgExit = "on"
	}

	groups := []struct {
		title string
		keys  []keyHint
	}{
		{"Navigate", []keyHint{{"↑↓ / jk", "move"}, {"pgup/pgdn", "page"}, {"g / G", "first / last"}, {"1 2 3 tab", "switch view"}}},
		{"Playback", []keyHint{{"↵ / p", "play / stop"}, {"x / space", "stop"}, {"b", "bg-exit (" + bgExit + ")"}}},
		{"Stations", []keyHint{{"/ s", "search"}, {"f", "favorite"}, {"e", "short name"}, {"r", "refresh"}}},
		{"General", []keyHint{{"?", "close help"}, {"esc", "close help"}, {"q", "quit"}}},
	}

	cols := make([]string, 0, len(groups))
	for _, g := range groups {
		keyW := 0
		for _, k := range g.keys {
			keyW = max(keyW, lipgloss.Width(k.key))
		}
		lines := []string{labelStyle.Render(g.title)}
		for _, k := range g.keys {
			lines = append(lines, keyStyle.Width(keyW).Render(k.key)+"  "+dimStyle.Render(k.desc))
		}
		cols = append(cols, strings.Join(lines, "\n"))
	}

	w := m.totalWidth()
	const colGap = "    "
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols[0], colGap, cols[1], colGap, cols[2], colGap, cols[3])
	if lipgloss.Width(body) > w-4 {
		top := lipgloss.JoinHorizontal(lipgloss.Top, cols[0], colGap, cols[1])
		bottom := lipgloss.JoinHorizontal(lipgloss.Top, cols[2], colGap, cols[3])
		body = top + "\n\n" + bottom
	}
	return renderPanel("Keyboard shortcuts", "? close", body, w, lipgloss.Height(body)+2, false)
}

// truncate shortens s to at most maxWidth terminal cells, adding an ellipsis.
func truncate(s string, maxWidth int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 0 {
		return ""
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

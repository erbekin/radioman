package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	radiobrowser "github.com/erbekin/radioman"
)

type tabIndex int

const (
	tabTop tabIndex = iota
	tabSearch
	tabFavorites
)

type searchField int

const (
	fieldName searchField = iota
	fieldCountry
	fieldTag
)

type stationsLoadedMsg struct {
	tab      tabIndex
	stations []radiobrowser.Station
	err      error
}

type playResultMsg struct {
	station radiobrowser.Station
	err     error
}

type tickMsg time.Time
type vizTickMsg time.Time
type playerSyncMsg struct{}

type TUIOptions struct {
	InitialTab     tabIndex
	InitialName    string
	InitialCountry string
	InitialTag     string
	Limit          int
	Visualizer     bool
	PlayerWarning  string
}

type model struct {
	client     *radiobrowser.Client
	player     *Player
	favorites  *FavoritesStore
	settings   *SettingsStore
	visualizer *Visualizer
	limit      int

	activeTab tabIndex
	stations  []radiobrowser.Station
	cursor    int
	offset    int

	searching    bool
	activeField  searchField
	nameInput    textinput.Model
	countryInput textinput.Model
	tagInput     textinput.Model

	editingSname   bool
	snameInput     textinput.Model
	editingStation radiobrowser.Station

	loading   bool
	spinner   spinner.Model
	statusMsg string
	errMsg    string

	width  int
	height int
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00E5FF")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(lipgloss.Color("#00E5FF")).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6ADC8")).
				Background(lipgloss.Color("#313244")).
				Padding(0, 2)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00E5FF")).
				Background(lipgloss.Color("#1E1E2E"))

	normalRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CDD6F4"))

	playingBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#A6E3A1"))

	favBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9E2AF"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6C7086"))

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89B4FA"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#45475A")).
			Padding(0, 1)

	nowPlayingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A6E3A1")).
			Background(lipgloss.Color("#181825")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F38BA8"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9E2AF"))
)

func NewTUIModel(
	client *radiobrowser.Client,
	player *Player,
	favorites *FavoritesStore,
	settings *SettingsStore,
	opts TUIOptions,
) model {
	if settings == nil {
		settings = &SettingsStore{Settings: DefaultSettings()}
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00E5FF"))

	nameIn := textinput.New()
	nameIn.Placeholder = "Station name (e.g. jazz, bbc)"
	nameIn.SetValue(opts.InitialName)
	nameIn.CharLimit = 80
	nameIn.Width = 26

	countryIn := textinput.New()
	countryIn.Placeholder = "Country (e.g. Germany, US)"
	countryIn.SetValue(opts.InitialCountry)
	countryIn.CharLimit = 60
	countryIn.Width = 24

	tagIn := textinput.New()
	tagIn.Placeholder = "Tag (e.g. rock, classical)"
	tagIn.SetValue(opts.InitialTag)
	tagIn.CharLimit = 60
	tagIn.Width = 24

	snameIn := textinput.New()
	snameIn.Placeholder = "Short name (a-zA-Z0-9-_.)"
	snameIn.CharLimit = 48
	snameIn.Width = 30

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	m := model{
		client:       client,
		player:       player,
		favorites:    favorites,
		settings:     settings,
		visualizer:   NewVisualizer(opts.Visualizer),
		limit:        limit,
		activeTab:    opts.InitialTab,
		nameInput:    nameIn,
		countryInput: countryIn,
		tagInput:     tagIn,
		snameInput:   snameIn,
		spinner:      sp,
		statusMsg:    opts.PlayerWarning,
		width:        100,
		height:       30,
	}

	if opts.InitialTab == tabFavorites {
		m.stations = favorites.Stations()
	} else {
		m.loading = true
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		tickCmd(),
	}
	if m.visualizer.Enabled {
		cmds = append(cmds, vizTickCmd())
	}
	switch m.activeTab {
	case tabTop:
		cmds = append(cmds, m.fetchTopStationsCmd())
	case tabSearch:
		if m.hasSearchFilters() {
			cmds = append(cmds, m.fetchSearchStationsCmd())
		} else {
			cmds = append(cmds, m.fetchTopStationsCmd())
		}
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func vizTickCmd() tea.Cmd {
	return tea.Tick(65*time.Millisecond, func(t time.Time) tea.Msg {
		return vizTickMsg(t)
	})
}

func (m model) hasSearchFilters() bool {
	return strings.TrimSpace(m.nameInput.Value()) != "" ||
		strings.TrimSpace(m.countryInput.Value()) != "" ||
		strings.TrimSpace(m.tagInput.Value()) != ""
}

func (m model) fetchTopStationsCmd() tea.Cmd {
	limit := m.limit
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		stations, err := m.client.TopStations(ctx, limit)
		return stationsLoadedMsg{tab: tabTop, stations: stations, err: err}
	}
}

func (m model) fetchSearchStationsCmd() tea.Cmd {
	query := radiobrowser.StationsQuery{
		Name:       strings.TrimSpace(m.nameInput.Value()),
		Country:    strings.TrimSpace(m.countryInput.Value()),
		Tag:        strings.TrimSpace(m.tagInput.Value()),
		Order:      "clickcount",
		Reverse:    true,
		HideBroken: true,
		Limit:      m.limit,
	}
	if len(query.Country) == 2 {
		query.CountryCode = strings.ToUpper(query.Country)
		query.Country = ""
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		stations, err := m.client.SearchStationsWithQuery(ctx, query)
		return stationsLoadedMsg{tab: tabSearch, stations: stations, err: err}
	}
}

func (m model) playStationCmd(station radiobrowser.Station) tea.Cmd {
	var favID uint
	var sname string
	if m.favorites != nil {
		if fav, ok := m.favorites.GetByStation(station); ok {
			favID = fav.ID
			sname = fav.ShortName
		}
	}
	return func() tea.Msg {
		err := m.player.Play(m.client, station, favID, sname)
		return playResultMsg{station: station, err: err}
	}
}

func (m model) syncPlayerCmd() tea.Cmd {
	return func() tea.Msg {
		m.player.SyncFromState(m.client)
		return playerSyncMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil

	case tickMsg:
		return m, tea.Batch(tickCmd(), m.syncPlayerCmd())

	case playerSyncMsg:
		return m, nil

	case vizTickMsg:
		if !m.visualizer.Enabled {
			return m, nil
		}
		playing := m.player.CurrentStation() != nil
		bands, hasSignal := m.player.AudioSpectrum()
		m.visualizer.Step(playing, bands, hasSignal)
		return m, vizTickCmd()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case stationsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to load stations: %v", msg.err)
			return m, nil
		}
		m.errMsg = ""
		m.stations = msg.stations
		m.cursor = 0
		m.offset = 0
		m.statusMsg = fmt.Sprintf("Loaded %d stations", len(msg.stations))
		return m, nil

	case playResultMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Playback error: %v", msg.err)
		} else {
			m.errMsg = ""
			m.statusMsg = fmt.Sprintf("Playing %s via %s", msg.station.Name, m.player.BackendName())
		}
		return m, nil

	case tea.KeyMsg:
		if m.editingSname {
			return m.updateSnameInput(msg)
		}
		if m.searching {
			return m.updateSearchInputs(msg)
		}
		return m.updateNavigation(msg)
	}

	return m, nil
}

func (m model) updateSnameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editingSname = false
		m.snameInput.Blur()
		m.statusMsg = "Cancelled short name edit"
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.snameInput.Value())
		updated, err := m.favorites.SetShortNameByStation(m.editingStation, newName)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.editingSname = false
		m.snameInput.Blur()
		m.errMsg = ""
		m.statusMsg = fmt.Sprintf("Updated #%d sname to %q", updated.ID, updated.ShortName)
		if m.activeTab == tabFavorites {
			m.stations = m.favorites.Stations()
		}
		return m, nil

	case "ctrl+c":
		m.player.Shutdown(m.settings.ContinueOnExit)
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.snameInput, cmd = m.snameInput.Update(msg)
	return m, cmd
}

func (m model) updateSearchInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.blurAllInputs()
		return m, nil

	case "tab", "down":
		m.activeField = (m.activeField + 1) % 3
		return m, m.focusActiveInput()

	case "shift+tab", "up":
		m.activeField = (m.activeField + 2) % 3
		return m, m.focusActiveInput()

	case "enter":
		m.searching = false
		m.blurAllInputs()
		m.activeTab = tabSearch
		m.loading = true
		m.errMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.fetchSearchStationsCmd())

	case "ctrl+c":
		m.player.Shutdown(m.settings.ContinueOnExit)
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.activeField {
	case fieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case fieldCountry:
		m.countryInput, cmd = m.countryInput.Update(msg)
	case fieldTag:
		m.tagInput, cmd = m.tagInput.Update(msg)
	}
	return m, cmd
}

func (m *model) blurAllInputs() {
	m.nameInput.Blur()
	m.countryInput.Blur()
	m.tagInput.Blur()
}

func (m *model) focusActiveInput() tea.Cmd {
	m.blurAllInputs()
	switch m.activeField {
	case fieldName:
		return m.nameInput.Focus()
	case fieldCountry:
		return m.countryInput.Focus()
	case fieldTag:
		return m.tagInput.Focus()
	}
	return nil
}

func (m model) updateNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.player.Shutdown(m.settings.ContinueOnExit)
		return m, tea.Quit

	case "1":
		return m.switchTab(tabTop)
	case "2":
		return m.switchTab(tabSearch)
	case "3":
		return m.switchTab(tabFavorites)
	case "tab":
		next := (m.activeTab + 1) % 3
		return m.switchTab(next)
	case "shift+tab":
		prev := (m.activeTab + 2) % 3
		return m.switchTab(prev)

	case "/", "s":
		m.activeTab = tabSearch
		m.searching = true
		m.activeField = fieldName
		return m, m.focusActiveInput()

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.stations)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
		return m, nil

	case "pgup":
		step := max(1, m.listHeight()-2)
		m.cursor = max(0, m.cursor-step)
		m.ensureCursorVisible()
		return m, nil

	case "pgdown":
		if len(m.stations) > 0 {
			step := max(1, m.listHeight()-2)
			m.cursor = min(len(m.stations)-1, m.cursor+step)
			m.ensureCursorVisible()
		}
		return m, nil

	case "home", "g":
		m.cursor = 0
		m.ensureCursorVisible()
		return m, nil

	case "end", "G":
		if len(m.stations) > 0 {
			m.cursor = len(m.stations) - 1
			m.ensureCursorVisible()
		}
		return m, nil

	case "enter", "p":
		if len(m.stations) == 0 || m.cursor >= len(m.stations) {
			return m, nil
		}
		selected := m.stations[m.cursor]
		if m.player.IsPlaying(selected) {
			m.player.Stop()
			m.statusMsg = fmt.Sprintf("Stopped %s", selected.Name)
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Connecting to %s...", selected.Name)
		return m, m.playStationCmd(selected)

	case "x", " ":
		if cur := m.player.CurrentStation(); cur != nil {
			m.player.Stop()
			m.statusMsg = fmt.Sprintf("Stopped %s", cur.Name)
		}
		return m, nil

	case "v":
		enabled := m.visualizer.Toggle()
		if enabled {
			m.statusMsg = fmt.Sprintf("Visualizer enabled (%s)", m.visualizer.Mode.String())
			return m, vizTickCmd()
		}
		m.statusMsg = "Visualizer hidden"
		return m, nil

	case "m":
		if !m.visualizer.Enabled {
			m.visualizer.Enabled = true
			m.statusMsg = fmt.Sprintf("Visualizer enabled (%s)", m.visualizer.Mode.String())
			return m, vizTickCmd()
		}
		mode := m.visualizer.NextMode()
		m.statusMsg = fmt.Sprintf("Visualizer mode: %s", mode.String())
		return m, nil

	case "f":
		if len(m.stations) == 0 || m.cursor >= len(m.stations) {
			return m, nil
		}
		selected := m.stations[m.cursor]
		added, err := m.favorites.Toggle(selected)
		if err != nil {
			m.errMsg = fmt.Sprintf("Save favorites error: %v", err)
			return m, nil
		}
		if added {
			if fav, ok := m.favorites.GetByStation(selected); ok {
				m.statusMsg = fmt.Sprintf("★ Added #%d (%s) %q to favorites", fav.ID, fav.ShortName, selected.Name)
			} else {
				m.statusMsg = fmt.Sprintf("★ Added %q to favorites", selected.Name)
			}
		} else {
			m.statusMsg = fmt.Sprintf("Removed %q from favorites", selected.Name)
		}
		if m.activeTab == tabFavorites {
			m.stations = m.favorites.Stations()
			if m.cursor >= len(m.stations) && m.cursor > 0 {
				m.cursor--
			}
			m.ensureCursorVisible()
		}
		return m, nil

	case "e":
		if len(m.stations) == 0 || m.cursor >= len(m.stations) {
			return m, nil
		}
		selected := m.stations[m.cursor]
		fav, ok := m.favorites.GetByStation(selected)
		if !ok {
			m.errMsg = "Press 'f' to favorite this station before setting an sname"
			return m, nil
		}
		m.errMsg = ""
		m.editingSname = true
		m.editingStation = selected
		m.snameInput.SetValue(fav.ShortName)
		m.snameInput.CursorEnd()
		return m, m.snameInput.Focus()

	case "b":
		enabled, err := m.settings.ToggleContinueOnExit()
		if err != nil {
			m.errMsg = fmt.Sprintf("Save settings error: %v", err)
			return m, nil
		}
		m.errMsg = ""
		if enabled {
			m.statusMsg = "Continue playing on exit: ON"
		} else {
			m.statusMsg = "Continue playing on exit: OFF"
		}
		return m, nil

	case "r":
		switch m.activeTab {
		case tabTop:
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchTopStationsCmd())
		case tabSearch:
			m.loading = true
			if m.hasSearchFilters() {
				return m, tea.Batch(m.spinner.Tick, m.fetchSearchStationsCmd())
			}
			return m, tea.Batch(m.spinner.Tick, m.fetchTopStationsCmd())
		case tabFavorites:
			m.stations = m.favorites.Stations()
			m.statusMsg = fmt.Sprintf("Favorites (%d stations)", len(m.stations))
			return m, nil
		}
	}

	return m, nil
}

func (m model) switchTab(target tabIndex) (tea.Model, tea.Cmd) {
	m.activeTab = target
	m.cursor = 0
	m.offset = 0
	m.errMsg = ""

	switch target {
	case tabTop:
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchTopStationsCmd())
	case tabSearch:
		if m.hasSearchFilters() {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchSearchStationsCmd())
		}
		m.searching = true
		m.activeField = fieldName
		return m, m.focusActiveInput()
	case tabFavorites:
		m.loading = false
		m.stations = m.favorites.Stations()
		m.statusMsg = fmt.Sprintf("Saved favorites (%d)", len(m.stations))
		return m, nil
	}
	return m, nil
}

func (m model) listHeight() int {
	reserved := 12
	if m.activeTab == tabSearch || m.searching {
		reserved += 3
	}
	h := m.height - reserved
	if h < 5 {
		return 5
	}
	return h
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

func (m model) View() string {
	var sections []string

	// Header & Tabs
	sections = append(sections, m.renderHeader())

	// Search Bar (shown on Search tab or while editing filters)
	if m.activeTab == tabSearch || m.searching {
		sections = append(sections, m.renderSearchBar())
	}

	// Main Content: Station List + Details Panel
	sections = append(sections, m.renderMainContent())

	// Now Playing Bar
	sections = append(sections, m.renderNowPlaying())

	// Footer Status & Keybindings
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m model) renderHeader() string {
	title := titleStyle.Render("📻 RADIOMAN")

	tabs := []struct {
		idx   tabIndex
		label string
	}{
		{tabTop, "1: Top Stations"},
		{tabSearch, "2: Search"},
		{tabFavorites, fmt.Sprintf("3: Favorites (%d)", m.favorites.Len())},
	}

	var renderedTabs []string
	for _, t := range tabs {
		if m.activeTab == t.idx {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(t.label))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(t.label))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Center, renderedTabs...)
	return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", tabRow)
}

func (m model) renderSearchBar() string {
	hint := dimStyle.Render("[Tab: next field | Enter: run search | Esc: cancel]")
	if !m.searching {
		hint = dimStyle.Render("[Press '/' or 's' to edit search filters]")
	}

	row := fmt.Sprintf(
		"%s %s   %s %s   %s %s   %s",
		labelStyle.Render("Name:"),
		m.nameInput.View(),
		labelStyle.Render("Country:"),
		m.countryInput.View(),
		labelStyle.Render("Tag:"),
		m.tagInput.View(),
		hint,
	)
	return boxStyle.Width(max(40, m.width-4)).Render(row)
}

func (m model) renderMainContent() string {
	totalWidth := max(60, m.width-4)
	listWidth := int(float64(totalWidth) * 0.58)
	detailWidth := totalWidth - listWidth - 3

	listHeight := m.listHeight()

	listBox := boxStyle.
		Width(listWidth).
		Height(listHeight).
		Render(m.renderStationList(listWidth-2, listHeight))

	detailBox := boxStyle.
		Width(detailWidth).
		Height(listHeight).
		Render(m.renderStationDetails(detailWidth-2, listHeight))

	return lipgloss.JoinHorizontal(lipgloss.Top, listBox, " ", detailBox)
}

func (m model) renderStationList(width, height int) string {
	if m.loading {
		return fmt.Sprintf("\n  %s Fetching stations from RadioBrowser...", m.spinner.View())
	}

	if len(m.stations) == 0 {
		if m.activeTab == tabFavorites {
			return dimStyle.Render("\n  No favorite stations yet.\n  Press 'f' on any station to save it here.")
		}
		return dimStyle.Render("\n  No stations found.\n  Press '/' or 's' to search with different filters.")
	}

	var rows []string
	end := min(len(m.stations), m.offset+height)

	for i := m.offset; i < end; i++ {
		st := m.stations[i]

		playIcon := "  "
		if m.player.IsPlaying(st) {
			playIcon = playingBadgeStyle.Render("▶ ")
		}

		favIcon := "  "
		if m.favorites.IsFavorite(st) {
			favIcon = favBadgeStyle.Render("★ ")
		}

		meta := []string{}
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

		metaStr := ""
		if len(meta) > 0 {
			metaStr = " [" + strings.Join(meta, " · ") + "]"
		}

		var line string
		if m.activeTab == tabFavorites {
			if fav, ok := m.favorites.GetByStation(st); ok {
				prefix := fmt.Sprintf("#%-2d [%s] ", fav.ID, fav.ShortName)
				maxNameLen := max(10, width-len(metaStr)-len(prefix)-6)
				nameStr := truncate(strings.TrimSpace(st.Name), maxNameLen)
				line = fmt.Sprintf("%s%s%s", prefix, nameStr, dimStyle.Render(metaStr))
			}
		}
		if line == "" {
			maxNameLen := max(12, width-len(metaStr)-10)
			nameStr := truncate(strings.TrimSpace(st.Name), maxNameLen)
			line = fmt.Sprintf("%2d. %s%s", i+1, nameStr, dimStyle.Render(metaStr))
		}

		if i == m.cursor {
			line = selectedRowStyle.Width(max(10, width-4)).Render(line)
		} else {
			line = normalRowStyle.Render(line)
		}

		rows = append(rows, playIcon+favIcon+line)
	}

	return strings.Join(rows, "\n")
}

func (m model) renderStationDetails(width, height int) string {
	var lines []string

	if len(m.stations) == 0 || m.cursor >= len(m.stations) {
		lines = append(lines, dimStyle.Render("Select a station to view details."))
	} else {
		st := m.stations[m.cursor]

		header := st.Name
		if m.favorites.IsFavorite(st) {
			header = "★ " + header
		}
		lines = append(lines, titleStyle.Render(truncate(header, width)))
		lines = append(lines, "")

		addField := func(label, val string) {
			if strings.TrimSpace(val) == "" {
				return
			}
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render(label+":"), truncate(val, max(10, width-len(label)-3))))
		}

		if fav, ok := m.favorites.GetByStation(st); ok {
			addField("Favorite", fmt.Sprintf("#%d (%s)", fav.ID, fav.ShortName))
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
			codecInfo = fmt.Sprintf("%s @ %d kbps", st.Codec, st.Bitrate)
		}
		addField("Audio", codecInfo)
		addField("Tags", st.Tags)
		addField("Votes", fmt.Sprintf("%d  |  Clicks: %d", st.Votes, st.ClickCount))
		addField("Homepage", st.Homepage)

		if streamURL, err := st.StreamURL(); err == nil {
			addField("Stream", streamURL)
		}
	}

	if m.visualizer != nil && m.visualizer.Enabled {
		usedLines := len(lines) + 1
		vizHeight := max(5, height-usedLines)
		playing := m.player.CurrentStation() != nil
		lines = append(lines, "")
		lines = append(lines, m.visualizer.Render(width, vizHeight, playing))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderNowPlaying() string {
	totalWidth := max(40, m.width-2)
	cur := m.player.CurrentStation()
	if cur == nil {
		return nowPlayingStyle.Width(totalWidth).Render(
			dimStyle.Render(fmt.Sprintf("⏹ Stopped  (Player backend: %s)", m.player.BackendName())),
		)
	}

	track := m.player.StreamTitle()
	info := fmt.Sprintf("▶ NOW PLAYING: %s", cur.Name)
	if cur.Codec != "" {
		info += fmt.Sprintf(" [%s", cur.Codec)
		if cur.Bitrate > 0 {
			info += fmt.Sprintf(" %dkbps", cur.Bitrate)
		}
		info += "]"
	}
	if track != "" {
		info += fmt.Sprintf("  ♪ %s", track)
	}
	return nowPlayingStyle.Width(totalWidth).Render(truncate(info, totalWidth-2))
}

func (m model) renderFooter() string {
	if m.editingSname {
		editLine := fmt.Sprintf(
			"%s %s  %s",
			labelStyle.Render("Edit sname:"),
			m.snameInput.View(),
			dimStyle.Render("[Enter: save | Esc: cancel]"),
		)
		if m.errMsg != "" {
			return lipgloss.JoinVertical(lipgloss.Left, errorStyle.Render("⚠ "+m.errMsg), editLine)
		}
		return editLine
	}

	var statusLine string
	if m.errMsg != "" {
		statusLine = errorStyle.Render("⚠ " + m.errMsg)
	} else if m.statusMsg != "" {
		statusLine = statusStyle.Render(m.statusMsg)
	} else {
		statusLine = dimStyle.Render("Ready")
	}

	bgExitState := "OFF"
	if m.settings != nil && m.settings.ContinueOnExit {
		bgExitState = "ON"
	}
	help := dimStyle.Render(
		fmt.Sprintf("↑/↓/j/k: navigate • Enter/p: play/stop • x/Space: stop • f: fav ★ • e: sname • b: bg-exit (%s) • v: visualizer • /: search • 1/2/3/Tab: views • q: quit", bgExitState),
	)

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, help)
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

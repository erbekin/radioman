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

	showHelp bool

	loading   bool
	spinner   spinner.Model
	statusMsg string
	errMsg    string

	width  int
	height int
}

func newStyledInput(placeholder, value string, charLimit int) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	in.SetValue(value)
	in.CharLimit = charLimit
	in.TextStyle = textStyle
	in.PlaceholderStyle = dimStyle
	in.Cursor.Style = lipgloss.NewStyle().Foreground(colAccent)
	return in
}

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
	sp.Style = lipgloss.NewStyle().Foreground(colAccent)

	nameIn := newStyledInput("jazz, bbc…", opts.InitialName, 80)
	countryIn := newStyledInput("Germany, US…", opts.InitialCountry, 60)
	tagIn := newStyledInput("rock, classical…", opts.InitialTag, 60)

	snameIn := newStyledInput("a-z A-Z 0-9 - _ .", "", 48)
	snameIn.Width = 24

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
		m.errMsg = ""
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
		m.ensureCursorVisible()
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

	case "?":
		m.showHelp = !m.showHelp
		m.ensureCursorVisible()
		return m, nil

	case "esc":
		if m.showHelp {
			m.showHelp = false
			m.ensureCursorVisible()
		}
		return m, nil

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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	radiobrowser "github.com/erbekin/radioman"
)

func main() {
	var (
		cliMode       = flag.Bool("c", false, "Run in CLI mode (non-interactive)")
		searchName    = flag.String("search", "", "Initial station name search filter")
		searchCountry = flag.String("country", "", "Initial country or 2-letter country code filter")
		searchTag     = flag.String("tag", "", "Initial tag/genre filter (e.g. jazz, rock, news)")
		showFavorites = flag.Bool("favorites", false, "Start directly in the Favorites view")
		visualizer    = flag.Bool("visualizer", false, "Enable the music visualizer on startup (toggle in TUI with 'v')")
		limit         = flag.Int("limit", 50, "Maximum number of stations to fetch")
		playerFlag    = flag.String("player", "", "Preferred audio player binary (mpv, ffplay, vlc)")
	)
	flag.Parse()

	isSettingsCmd := flag.NArg() > 0 &&
		(flag.Arg(0) == "settings" || flag.Arg(0) == "config") &&
		*searchName == "" && *searchCountry == "" && *searchTag == "" &&
		!*showFavorites && !*visualizer

	if *cliMode || isSettingsCmd {
		if err := RunCLI(flag.Args(), *playerFlag, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "radioman: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// If positional args are passed without -search, treat them as a station name query
	if *searchName == "" && flag.NArg() > 0 {
		*searchName = flag.Arg(0)
	}

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	client, err := radiobrowser.NewClient(httpClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create RadioBrowser client: %v\n", err)
		os.Exit(1)
	}

	favorites, err := LoadFavorites()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load favorites: %v\n", err)
	}

	settings, err := LoadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load settings: %v\n", err)
	}

	backend, playerErr := DetectPlayer(*playerFlag)
	bg := NewBackgroundController(DefaultStatePath(), *playerFlag)
	player := NewPlayer(backend, bg)
	player.SyncFromState(client)
	defer func() {
		player.Shutdown(settings.ContinueOnExit)
	}()

	var playerWarning string
	if playerErr != nil {
		playerWarning = playerErr.Error()
	}

	initialTab := tabTop
	if *showFavorites {
		initialTab = tabFavorites
	} else if *searchName != "" || *searchCountry != "" || *searchTag != "" {
		initialTab = tabSearch
	}

	m := NewTUIModel(client, player, favorites, settings, TUIOptions{
		InitialTab:     initialTab,
		InitialName:    *searchName,
		InitialCountry: *searchCountry,
		InitialTag:     *searchTag,
		Limit:          *limit,
		Visualizer:     *visualizer,
		PlayerWarning:  playerWarning,
	})

	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "radioman error: %v\n", err)
		os.Exit(1)
	}
}

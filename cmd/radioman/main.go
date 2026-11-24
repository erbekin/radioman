package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	radiobrowser "github.com/erbekin/radioman"
)

func resolveAutoStop(settings *SettingsStore, override *time.Duration) (time.Duration, error) {
	if override != nil {
		return *override, nil
	}
	if settings == nil {
		return 0, nil
	}
	return settings.AutoStopDuration()
}

func main() {
	if len(os.Args) >= 5 && os.Args[1] == "__auto-stop-worker" {
		targetPID, err := strconv.Atoi(os.Args[3])
		if err != nil {
			os.Exit(1)
		}
		stopAtNano, err := strconv.ParseInt(os.Args[4], 10, 64)
		if err != nil {
			os.Exit(1)
		}
		if err := runAutoStopWorker(os.Args[2], targetPID, stopAtNano); err != nil {
			os.Exit(1)
		}
		return
	}

	var (
		cliMode       = flag.Bool("c", false, "Run in CLI mode (non-interactive)")
		searchName    = flag.String("search", "", "Initial station name search filter")
		searchCountry = flag.String("country", "", "Initial country or 2-letter country code filter")
		searchTag     = flag.String("tag", "", "Initial tag/genre filter (e.g. jazz, rock, news)")
		showFavorites = flag.Bool("favorites", false, "Start directly in the Favorites view")
		limit         = flag.Int("limit", 50, "Maximum number of stations to fetch")
		playerFlag    = flag.String("player", "", "Preferred audio player binary (mpv, ffplay, vlc)")
		autoStopFlag  = flag.String("auto-stop", "", "Auto-stop timer override (e.g. 10m, 1.5h, 0 to disable)")
	)
	flag.Parse()

	var autoStopOverride *time.Duration
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "auto-stop" {
			dur, err := ParseAutoStopDuration(*autoStopFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "radioman: invalid -auto-stop value: %v\n", err)
				os.Exit(1)
			}
			autoStopOverride = &dur
		}
	})

	isSettingsCmd := flag.NArg() > 0 &&
		(flag.Arg(0) == "settings" || flag.Arg(0) == "config") &&
		*searchName == "" && *searchCountry == "" && *searchTag == "" &&
		!*showFavorites

	if *cliMode || isSettingsCmd {
		if err := RunCLI(flag.Args(), *playerFlag, autoStopOverride, os.Stdout); err != nil {
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
	if autoStopDur, err := resolveAutoStop(settings, autoStopOverride); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid auto_stop setting: %v\n", err)
	} else {
		bg.SetAutoStop(autoStopDur)
	}
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
		PlayerWarning:  playerWarning,
	})

	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "radioman error: %v\n", err)
		os.Exit(1)
	}
}

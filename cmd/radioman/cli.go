package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/erbekin/radioman/models"
)

func RunCLI(args []string, preferredPlayer string, autoStopOverride *time.Duration, out io.Writer) error {
	favs, err := LoadFavorites()
	if err != nil {
		return fmt.Errorf("load favorites: %w", err)
	}
	settings, err := LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	bg := NewBackgroundController(DefaultStatePath(), preferredPlayer)
	if autoStopDur, err := resolveAutoStop(settings, autoStopOverride); err != nil {
		return fmt.Errorf("auto_stop setting: %w", err)
	} else {
		bg.SetAutoStop(autoStopDur)
	}
	return runCLIWithDeps(args, favs, bg, settings, out)
}

func expandCLIAliases(args []string) []string {
	if len(args) == 0 {
		return args
	}
	rest := args[1:]
	switch args[0] {
	case "fl":
		return append([]string{"favs", "list"}, rest...)
	case "fp":
		return append([]string{"favs", "play"}, rest...)
	case "pp":
		return append([]string{"player", "play"}, rest...)
	case "ps":
		return append([]string{"player", "stop"}, rest...)
	case "pc":
		return append([]string{"player", "current"}, rest...)
	default:
		return args
	}
}

func runCLIWithDeps(args []string, favs *FavoritesStore, bg *BackgroundController, settings *SettingsStore, out io.Writer) error {
	if settings == nil {
		settings = &SettingsStore{Settings: DefaultSettings()}
	}
	args = expandCLIAliases(args)
	if len(args) == 0 {
		printCLIHelp(out)
		return errors.New("no command specified")
	}

	switch args[0] {
	case "help", "-h", "--help":
		printCLIHelp(out)
		return nil

	case "favs":
		return runFavsCLI(args[1:], favs, bg, out)

	case "player":
		return runPlayerCLI(args[1:], bg, out)

	case "settings", "config":
		return runSettingsCLI(args[1:], settings, out)

	default:
		printCLIHelp(out)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runFavsCLI(args []string, favs *FavoritesStore, bg *BackgroundController, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("favs requires a subcommand: list, play <idx|sname>, or rename <idx|sname> <new-sname>")
	}

	switch args[0] {
	case "list":
		if favs.Len() == 0 {
			fmt.Fprintln(out, "No favorite stations saved.")
			return nil
		}
		for _, fav := range favs.Entries {
			meta := formatStationMeta(fav.Station)
			if meta != "" {
				fmt.Fprintf(out, "%3d  %-24s  %s [%s]\n", fav.ID, fav.ShortName, fav.Station.Name, meta)
			} else {
				fmt.Fprintf(out, "%3d  %-24s  %s\n", fav.ID, fav.ShortName, fav.Station.Name)
			}
		}
		return nil

	case "play":
		if len(args) < 2 {
			return errors.New("usage: radioman -c favs play <idx or sname>")
		}
		fav, ok := favs.Find(args[1])
		if !ok {
			return fmt.Errorf("favorite %q not found", args[1])
		}
		st, err := bg.PlayStation(fav.Station, fav.ID, fav.ShortName)
		if err != nil {
			return err
		}
		autoStopSuffix := ""
		if !st.StopAt.IsZero() {
			autoStopSuffix = fmt.Sprintf(" (auto-stop in %s)", FormatRemaining(time.Until(st.StopAt)))
		}
		fmt.Fprintf(out, "Playing #%d (%s) %s via %s [pid %d]%s\n", fav.ID, fav.ShortName, fav.Station.Name, st.Backend, st.PID, autoStopSuffix)
		return nil

	case "rename", "sname":
		if len(args) < 3 {
			return errors.New("usage: radioman -c favs rename <idx or sname> <new-sname>")
		}
		updated, err := favs.SetShortName(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Updated #%d (%s): sname is now %s\n", updated.ID, updated.Station.Name, updated.ShortName)
		return nil

	default:
		return fmt.Errorf("unknown favs subcommand %q", args[0])
	}
}

func runPlayerCLI(args []string, bg *BackgroundController, out io.Writer) error {
	if len(args) == 0 {
		st, backend, err := bg.Status()
		if err != nil {
			return err
		}
		backendLabel := backend.Name
		if backendLabel == "" {
			backendLabel = "none"
		}
		if backend.Path != "" {
			fmt.Fprintf(out, "Player: %s (%s)\n", backendLabel, backend.Path)
		} else {
			fmt.Fprintf(out, "Player: %s\n", backendLabel)
		}
		if st.Playing {
			fmt.Fprintf(out, "Status: playing (pid %d)\n", st.PID)
			if !st.StopAt.IsZero() {
				fmt.Fprintf(out, "Auto-stop: in %s (at %s)\n", FormatRemaining(time.Until(st.StopAt)), st.StopAt.Local().Format("15:04:05"))
			}
		} else {
			fmt.Fprintln(out, "Status: stopped")
		}
		if st.Station != nil {
			fmt.Fprintf(out, "Current: %s\n", formatCurrentLabel(st))
		} else {
			fmt.Fprintln(out, "Current: none")
		}
		return nil
	}

	switch args[0] {
	case "play":
		st, err := bg.PlayCurrent()
		if err != nil {
			return err
		}
		autoStopSuffix := ""
		if !st.StopAt.IsZero() {
			autoStopSuffix = fmt.Sprintf(" (auto-stop in %s)", FormatRemaining(time.Until(st.StopAt)))
		}
		fmt.Fprintf(out, "Playing %s via %s [pid %d]%s\n", formatCurrentLabel(st), st.Backend, st.PID, autoStopSuffix)
		return nil

	case "stop":
		st, wasPlaying, err := bg.Stop()
		if err != nil {
			return err
		}
		if !wasPlaying {
			if st.Station != nil {
				fmt.Fprintf(out, "Player is already stopped (current: %s).\n", formatCurrentLabel(st))
			} else {
				fmt.Fprintln(out, "Player is already stopped.")
			}
			return nil
		}
		if st.Station != nil {
			fmt.Fprintf(out, "Stopped %s (remembered as current).\n", formatCurrentLabel(st))
		} else {
			fmt.Fprintln(out, "Stopped player.")
		}
		return nil

	case "current":
		st, _, err := bg.Status()
		if err != nil {
			return err
		}
		if st.Station == nil {
			fmt.Fprintln(out, "No current station.")
			return nil
		}
		status := "stopped"
		if st.Playing {
			status = fmt.Sprintf("playing (pid %d)", st.PID)
		}
		fmt.Fprintf(out, "Name: %s\n", st.Station.Name)
		if st.FavoriteID > 0 || st.ShortName != "" {
			fmt.Fprintf(out, "Favorite: #%d (%s)\n", st.FavoriteID, st.ShortName)
		}
		fmt.Fprintf(out, "Status: %s\n", status)
		if st.Playing && !st.StopAt.IsZero() {
			fmt.Fprintf(out, "Auto-stop: in %s (at %s)\n", FormatRemaining(time.Until(st.StopAt)), st.StopAt.Local().Format("15:04:05"))
		}
		if st.Station.Country != "" {
			if st.Station.CountryCode != "" {
				fmt.Fprintf(out, "Country: %s (%s)\n", st.Station.Country, st.Station.CountryCode)
			} else {
				fmt.Fprintf(out, "Country: %s\n", st.Station.Country)
			}
		}
		if meta := formatStationMeta(*st.Station); meta != "" {
			fmt.Fprintf(out, "Audio: %s\n", meta)
		}
		if st.Station.Tags != "" {
			fmt.Fprintf(out, "Tags: %s\n", st.Station.Tags)
		}
		if streamURL, err := st.Station.StreamURL(); err == nil {
			fmt.Fprintf(out, "Stream: %s\n", streamURL)
		}
		if st.Station.Homepage != "" {
			fmt.Fprintf(out, "Homepage: %s\n", st.Station.Homepage)
		}
		return nil

	default:
		return fmt.Errorf("unknown player subcommand %q", args[0])
	}
}

func formatCurrentLabel(st *PlayerState) string {
	if st == nil || st.Station == nil {
		return "none"
	}
	if st.FavoriteID > 0 && st.ShortName != "" {
		return fmt.Sprintf("#%d (%s) %s", st.FavoriteID, st.ShortName, st.Station.Name)
	}
	return st.Station.Name
}

func formatStationMeta(st models.Station) string {
	var parts []string
	if st.CountryCode != "" {
		parts = append(parts, st.CountryCode)
	} else if st.Country != "" {
		parts = append(parts, st.Country)
	}
	if st.Codec != "" {
		if st.Bitrate > 0 {
			parts = append(parts, fmt.Sprintf("%s %dkbps", st.Codec, st.Bitrate))
		} else {
			parts = append(parts, st.Codec)
		}
	}
	return strings.Join(parts, " · ")
}

func runSettingsCLI(args []string, settings *SettingsStore, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintf(out, "continue_on_exit: %v\n", settings.ContinueOnExit)
		fmt.Fprintf(out, "auto_stop: %s\n", settings.AutoStop)
		return nil
	}

	switch args[0] {
	case "continue_on_exit":
		if len(args) < 2 {
			fmt.Fprintf(out, "continue_on_exit: %v\n", settings.ContinueOnExit)
			return nil
		}

		var val bool
		switch strings.ToLower(strings.TrimSpace(args[1])) {
		case "true", "1", "on", "yes":
			val = true
		case "false", "0", "off", "no":
			val = false
		default:
			return fmt.Errorf("invalid boolean value %q (use true/false or on/off)", args[1])
		}

		if err := settings.SetContinueOnExit(val); err != nil {
			return err
		}
		fmt.Fprintf(out, "continue_on_exit: %v\n", settings.ContinueOnExit)
		return nil

	case "auto_stop":
		if len(args) < 2 {
			fmt.Fprintf(out, "auto_stop: %s\n", settings.AutoStop)
			return nil
		}
		raw := strings.Join(args[1:], " ")
		if _, _, err := settings.SetAutoStop(raw); err != nil {
			return err
		}
		fmt.Fprintf(out, "auto_stop: %s\n", settings.AutoStop)
		return nil

	default:
		return fmt.Errorf("unknown setting %q (available: continue_on_exit, auto_stop)", args[0])
	}
}

func printCLIHelp(out io.Writer) {
	fmt.Fprintln(out, `Usage: radioman -c <command> [args...]

Commands:
  favs list                        List favorite stations with IDs and short names
  favs play <idx|sname>            Play a favorite station in the background
  favs rename <idx|sname> <sname>  Change a favorite station's short name
  player                           Print player info, status, and current station
  player play                      Resume/play the current station in the background
  player stop                      Stop the background player (remembers current station)
  player current                   Print details for the current station
  settings                         Show current settings (also works without -c)
  settings continue_on_exit <on|off>  Toggle whether TUI playback continues on exit
  settings auto_stop <dur|0|off>      Automatically stop playback after T (e.g. 10m, 1.5h, 0)

Flags:
  -auto-stop <dur|0|off>           Override auto_stop timer for this command/session

Short aliases:
  fl                               favs list
  fp <idx|sname>                   favs play <idx|sname>
  pp                               player play
  ps                               player stop
  pc                               player current`)
}

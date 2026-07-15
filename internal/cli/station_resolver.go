package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

type ResolvedStation struct {
	UUID string
	Name string
}

type StationResolver struct {
	client *api.Client
	cfg    *config.Config
	out    *output.Output
}

func NewStationResolver(client *api.Client, cfg *config.Config, out *output.Output) *StationResolver {
	return &StationResolver{
		client: client,
		cfg:    cfg,
		out:    out,
	}
}

func (r *StationResolver) Resolve(ctx context.Context, stationFlag string) (*ResolvedStation, error) {
	stations, err := r.client.ListStations(ctx)
	if err != nil {
		return nil, handleAPIError(r.out, err)
	}

	if stationFlag != "" {
		return resolveStationByIDOrName(stations, stationFlag)
	}

	if r.cfg.HasDefaultStation() {
		for _, s := range stations {
			if s.UUID == r.cfg.DefaultStationUUID {
				return &ResolvedStation{UUID: s.UUID, Name: s.Name}, nil
			}
		}
	}

	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	isJSON := r.out.IsJSON()

	if isTTY && !isJSON {
		r.out.PrintWarning("No default station is configured.")
		r.out.Println()
		r.out.PrintInfo("Select the destination station:")
		r.out.Println()

		opts := make([]huh.Option[string], len(stations))
		for i, s := range stations {
			opts[i] = huh.NewOption(s.Name, s.UUID)
		}

		var selectedUUID string
		selectPrompt := huh.NewSelect[string]().
			Title("").
			Options(opts...).
			Value(&selectedUUID)

		if err := huh.NewForm(huh.NewGroup(selectPrompt)).Run(); err != nil {
			return nil, fmt.Errorf("station selection cancelled")
		}

		var selectedStation api.Station
		for _, s := range stations {
			if s.UUID == selectedUUID {
				selectedStation = s
				break
			}
		}

		r.out.Println()
		r.out.Print(fmt.Sprintf("Use \"%s\" as the default station for future commands? [Y/n] ", selectedStation.Name))

		var response string
		fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			r.cfg.DefaultStationUUID = selectedStation.UUID
			r.cfg.DefaultStationName = selectedStation.Name
			if err := config.SaveWithPreservation(r.cfg); err != nil {
				r.out.PrintWarning(fmt.Sprintf("Could not save default station: %s", err))
			}
		}

		return &ResolvedStation{UUID: selectedStation.UUID, Name: selectedStation.Name}, nil
	}

	msg := `No destination station is configured.

Run:
  media-cli stations use <uuid-or-name>

Or provide:
  --station <uuid-or-name>`

	if isJSON {
		r.out.PrintJSONError(map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    "missing_station",
				"message": "No destination station is configured.",
				"details": map[string]interface{}{
					"suggested_commands": []string{
						"media-cli stations use <uuid-or-name>",
						"media-cli media upload song.mp3 --station <uuid-or-name>",
					},
				},
			},
		})
	} else {
		r.out.PrintStdErr(msg)
	}

	return nil, fmt.Errorf("no destination station is configured")
}

func resolveStationByIDOrName(stations []api.Station, input string) (*ResolvedStation, error) {
	if len(stations) == 0 {
		return nil, fmt.Errorf("no stations available")
	}

	for _, s := range stations {
		if s.UUID == input {
			return &ResolvedStation{UUID: s.UUID, Name: s.Name}, nil
		}
	}

	var uuidMatches []api.Station
	for _, s := range stations {
		if strings.HasPrefix(s.UUID, input) {
			uuidMatches = append(uuidMatches, s)
		}
	}
	if len(uuidMatches) == 1 {
		return &ResolvedStation{UUID: uuidMatches[0].UUID, Name: uuidMatches[0].Name}, nil
	}
	if len(uuidMatches) > 1 {
		return nil, ambiguousStationError(uuidMatches, input)
	}

	for _, s := range stations {
		if strings.EqualFold(s.Name, input) {
			return &ResolvedStation{UUID: s.UUID, Name: s.Name}, nil
		}
	}

	var nameMatches []api.Station
	lowerInput := strings.ToLower(input)
	for _, s := range stations {
		if strings.Contains(strings.ToLower(s.Name), lowerInput) {
			nameMatches = append(nameMatches, s)
		}
	}
	if len(nameMatches) == 1 {
		return &ResolvedStation{UUID: nameMatches[0].UUID, Name: nameMatches[0].Name}, nil
	}
	if len(nameMatches) > 1 {
		return nil, ambiguousStationError(nameMatches, input)
	}

	return nil, fmt.Errorf("station %q not found. Use 'media-cli stations list' to see available stations", input)
}

func ambiguousStationError(matches []api.Station, input string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Multiple stations match %q:\n\n", input))
	for _, s := range matches {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", s.UUID, s.Name))
	}
	sb.WriteString("\nUse a more specific UUID or name.")
	return fmt.Errorf("%s", sb.String())
}

package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"radioplatform-media-ci/internal/config"
	"radioplatform-media-ci/internal/output"
	"radioplatform-media-ci/pkg/api"
)

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Radio Platform CLI API",
		Long: `Interactively authenticate with the Radio Platform CLI API.

You must have a CLI API key from Account Settings -> CLI API keys.

The CLI will validate your credentials and prompt you to select
a default station.

Non-interactive environments should use the RADIO_PLATFORM_CLI_KEY
and RADIO_PLATFORM_CLI_URL environment variables instead.`,
		Args: cobra.NoArgs,
		RunE: runLogin,
	}

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	out := output.New(jsonFlag, noColorFlag, debugFlag)

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		msg := `Interactive login requires a terminal.

For non-interactive commands, set RADIO_PLATFORM_CLI_KEY, set RADIO_PLATFORM_CLI_URL, and provide --station.`
		if out.IsJSON() {
			out.PrintJSONError(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "no_tty",
					"message": msg,
				},
			})
		} else {
			out.PrintStdErr(msg)
		}
		return fmt.Errorf("interactive login requires a terminal")
	}

	cfg, err := config.Load()
	if err != nil {
		out.PrintError("Warning: could not load existing config: " + err.Error())
	}

	var apiKey string
	serverURL := cfg.EffectiveServerURL()

	prompt := huh.NewInput().
		Title("CLI API key").
		Prompt("").
		EchoMode(huh.EchoModePassword).
		Value(&apiKey)

	fields := []huh.Field{prompt}
	if !cfg.HasServerURL() {
		serverPrompt := huh.NewInput().
			Title("Server URL").
			Prompt("").
			Placeholder("https://radio.example.com").
			Value(&serverURL)
		fields = append(fields, serverPrompt)
	}

	out.PrintTitle("Radioplatform Media CLI")
	out.Println()
	if cfg.HasServerURL() {
		out.PrintKV("Server", serverURL)
	}

	err = huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeBase16()).Run()
	if err != nil {
		return fmt.Errorf("input cancelled: %w", err)
	}

	if apiKey == "" {
		return fmt.Errorf("CLI API key cannot be empty")
	}
	if serverURL == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	out.Println()

	client := api.NewClient(apiKey, api.WithBaseURL(serverURL))

	stations, err := client.ListStations(cmd.Context())
	if err != nil {
		if ae := api.AsAPIError(err); ae != nil {
			out.PrintError(ae.FriendlyMessage())
		} else {
			out.PrintError(err.Error())
		}
		return err
	}

	out.PrintOK("\u2713 Credentials validated")
	out.Println()

	cfg.APIKey = apiKey
	if cfg.ServerSource() != "environment" {
		cfg.ServerURL = serverURL
	}

	if len(stations) == 0 {
		if err := config.SaveWithPreservation(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		configPath, _ := config.ConfigPath()
		out.PrintOK("\u2713 Logged in")
		if configPath != "" {
			out.PrintInfo("Credentials saved to " + configPath)
		}
		out.PrintWarning("No stations found. Use --station or 'stations use' to set a station when one becomes available.")
		return nil
	}

	if err := config.SaveWithPreservation(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	out.PrintInfo("Select the default station:")
	out.Println()

	var selectedStation api.Station
	opts := make([]huh.Option[string], len(stations))
	for i, s := range stations {
		opts[i] = huh.NewOption(s.Name, s.UUID)
	}

	var selectedUUID string
	selectPrompt := huh.NewSelect[string]().
		Title("").
		Options(opts...).
		Value(&selectedUUID)

	if err := huh.NewForm(huh.NewGroup(selectPrompt)).WithTheme(huh.ThemeBase16()).Run(); err != nil {
		return fmt.Errorf("station selection cancelled: %w", err)
	}

	for _, s := range stations {
		if s.UUID == selectedUUID {
			selectedStation = s
			break
		}
	}

	cfg.DefaultStationUUID = selectedStation.UUID
	cfg.DefaultStationName = selectedStation.Name

	if err := config.SaveWithPreservation(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, _ := config.ConfigPath()

	out.PrintOK("\u2713 Logged in")
	out.PrintOK(fmt.Sprintf("\u2713 Default station set to %s", selectedStation.Name))
	if configPath != "" {
		out.PrintInfo("Credentials saved to " + configPath)
	}

	if out.IsJSON() {
		out.PrintJSON(map[string]interface{}{
			"success":      true,
			"logged_in":    true,
			"station_uuid": selectedStation.UUID,
			"station_name": selectedStation.Name,
		})
	}

	return nil
}

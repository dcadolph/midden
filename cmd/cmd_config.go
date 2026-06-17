package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dcadolph/midden/internal/config"
	"github.com/dcadolph/midden/internal/jsonutil"
)

// configCmd groups configuration subcommands.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect the loaded midden configuration.",
}

// configPathCmd prints the resolved path to the config file.
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path midden uses to read its configuration.",
	RunE:  runConfigPath,
}

// configShowCmd prints the loaded configuration.
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the loaded configuration (YAML by default).",
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

// runConfigPath prints the absolute path to the configuration file.
func runConfigPath(cmd *cobra.Command, _ []string) error {
	path, err := config.ResolvePath()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("resolve config path: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), path)
	return nil
}

// runConfigShow prints the loaded configuration.
func runConfigShow(cmd *cobra.Command, _ []string) error {
	cfg, err := userConfig()
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	w := cmd.OutOrStdout()
	if jsonOutput {
		return jsonutil.Encode(w, cfg, jsonPretty)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

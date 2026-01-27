package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/simulot/immich-go/app/root"
	"github.com/simulot/immich-go/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type EnvVarInfo struct {
	Path    string
	Flag    string
	Usage   string
	Default pflag.Value
}

// collectEnvVars recursively collects environment variable information from cobra commands
func collectEnvVars(cmd *cobra.Command, envVars map[string]EnvVarInfo) {
	config.TraverseCommands(cmd, []string{}, func(cmd *cobra.Command, path []string) map[string]any {
		visitor := func(f *pflag.Flag) {
			if f.Name != "config" && f.Name != "help" {
				varName := "IMMICH_GO_"
				if len(path) > 0 {
					varName += strings.ToUpper(strings.ReplaceAll(strings.Join(path, "_"), "-", "_")) + "_"
				}
				varName += strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
				current, ok := envVars[varName]
				if !ok || len(current.Path) > len(strings.Join(path, " ")) {
					envVars[varName] = EnvVarInfo{
						Path:    strings.Join(path, " "),
						Flag:    f.Name,
						Usage:   f.Usage,
						Default: f.Value,
					}
				}
			}
		}
		// Process local flags for this command
		cmd.Flags().VisitAll(visitor)

		if cmd.HasPersistentFlags() {
			cmd.PersistentFlags().VisitAll(visitor)
		}
		return map[string]any{}
	})
}

// generateEnvVarsDoc generates markdown documentation for environment variables
func generateEnvVarsDoc(rootCmd *cobra.Command, p string) {
	envVars := map[string]EnvVarInfo{}
	collectEnvVars(rootCmd, envVars)

	f, err := os.Create(path.Join(p, "environment.md"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Fprintln(f, "# Environment Variables")
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "The following environment variables can be used to configure `immich-go`.")
	fmt.Fprintln(f, "")

	// Group by path
	varsByPath := map[string][]struct {
		Name string
		Info EnvVarInfo
	}{}

	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		info := envVars[k]
		path := info.Path
		if path == "" {
			path = "Global"
		}
		varsByPath[path] = append(varsByPath[path], struct {
			Name string
			Info EnvVarInfo
		}{k, info})
	}

	// Get sorted paths
	paths := make([]string, 0, len(varsByPath))
	for p := range varsByPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		fmt.Fprintf(f, "## %s\n\n", p)
		fmt.Fprintln(f, "| Variable | Flag | Default | Description |")
		fmt.Fprintln(f, "|----------|------|---------|-------------|")
		for _, v := range varsByPath[p] {
			defaultValue := v.Info.Default.String()
			if v.Info.Flag == "device-uuid" || v.Info.Flag == "from-device-uuid" {
				defaultValue = "HOSTNAME"
			}
			if defaultValue != "" {
				defaultValue = "`" + defaultValue + "`"
			}

			fmt.Fprintf(f, "| `%s` | `--%s` | %s | %s |\n", v.Name, v.Info.Flag, defaultValue, v.Info.Usage)
		}
		fmt.Fprintln(f, "")
	}
}

// main generates documentation for environment variables
func main() {
	p := path.Base(path.Dir(os.Args[0]))
	rootCmd, _ := root.RootImmichGoCommand(context.Background())

	if p == "docs" {
		p = "."
	} else {
		p = "docs/"
	}
	// Generate documentation
	generateEnvVarsDoc(rootCmd, p)
}

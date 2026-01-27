package config

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cm := New()
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.v)
	assert.NotNil(t, cm.origins)
	assert.False(t, cm.processed)
	assert.Nil(t, cm.command)
}

func TestInit(t *testing.T) {
	cm := New()
	err := cm.Init()
	assert.NoError(t, err)
}

func TestProcessCommand(t *testing.T) {
	tests := []struct {
		name        string
		setupCmd    func() *cobra.Command
		setupEnv    func()
		cleanupEnv  func()
		checkOrigin func(t *testing.T, cm *ConfigurationManager, cmd *cobra.Command)
		wantErr     bool
	}{
		{
			name: "simple command with flags",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("test-flag", "default", "test flag")
				return cmd
			},
			checkOrigin: func(t *testing.T, cm *ConfigurationManager, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("test-flag")
				origin := cm.GetFlagOrigin(cmd, flag)
				assert.Equal(t, OriginDefault, origin)
			},
		},
		{
			name: "command with CLI provided flag",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("test-flag", "default", "test flag")
				err := cmd.Flags().Set("test-flag", "cli-value")
				require.NoError(t, err)
				return cmd
			},
			checkOrigin: func(t *testing.T, cm *ConfigurationManager, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("test-flag")
				origin := cm.GetFlagOrigin(cmd, flag)
				assert.Equal(t, OriginCLI, origin)
			},
		},
		{
			name: "command with environment variable",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("test-flag", "default", "test flag")
				return cmd
			},
			setupEnv: func() {
				os.Setenv("IMMICH_GO_TEST_FLAG", "env-value")
			},
			cleanupEnv: func() {
				os.Unsetenv("IMMICH_GO_TEST_FLAG")
			},
			checkOrigin: func(t *testing.T, cm *ConfigurationManager, cmd *cobra.Command) {
				flag := cmd.Flags().Lookup("test-flag")
				origin := cm.GetFlagOrigin(cmd, flag)
				assert.Equal(t, OriginEnvironment, origin)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := New()
			err := cm.Init()
			require.NoError(t, err)

			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			cmd := tt.setupCmd()
			err = cm.ProcessCommand(cmd)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkOrigin != nil {
					tt.checkOrigin(t, cm, cmd)
				}
			}
		})
	}
}

func TestGetViperKey(t *testing.T) {
	tests := []struct {
		name     string
		setupCmd func() (*cobra.Command, *pflag.Flag)
		expected string
	}{
		{
			name: "root command flag",
			setupCmd: func() (*cobra.Command, *pflag.Flag) {
				cmd := &cobra.Command{Use: "root"}
				cmd.Flags().String("test", "value", "")
				flag := cmd.Flags().Lookup("test")
				return cmd, flag
			},
			expected: "test",
		},
		{
			name: "subcommand local flag",
			setupCmd: func() (*cobra.Command, *pflag.Flag) {
				rootCmd := &cobra.Command{Use: "root"}
				subCmd := &cobra.Command{Use: "sub"}
				rootCmd.AddCommand(subCmd)
				subCmd.Flags().String("local-flag", "value", "")
				flag := subCmd.Flags().Lookup("local-flag")
				return subCmd, flag
			},
			expected: "sub.local-flag",
		},
		{
			name: "subcommand inherited persistent flag",
			setupCmd: func() (*cobra.Command, *pflag.Flag) {
				rootCmd := &cobra.Command{Use: "root"}
				rootCmd.PersistentFlags().String("persistent-flag", "value", "")
				subCmd := &cobra.Command{Use: "sub"}
				rootCmd.AddCommand(subCmd)
				// The flag should be available on the subcommand
				flag := subCmd.PersistentFlags().Lookup("persistent-flag")
				if flag == nil {
					// If not found, it might be inherited but not yet looked up
					flag = rootCmd.PersistentFlags().Lookup("persistent-flag")
				}
				return subCmd, flag
			},
			expected: "persistent-flag",
		},
		{
			name: "deeply nested subcommand",
			setupCmd: func() (*cobra.Command, *pflag.Flag) {
				rootCmd := &cobra.Command{Use: "root"}
				midCmd := &cobra.Command{Use: "mid"}
				rootCmd.AddCommand(midCmd)
				subCmd := &cobra.Command{Use: "sub"}
				midCmd.AddCommand(subCmd)
				subCmd.Flags().String("deep-flag", "value", "")
				flag := subCmd.Flags().Lookup("deep-flag")
				return subCmd, flag
			},
			expected: "mid.sub.deep-flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flag := tt.setupCmd()
			key := getViperKey(cmd, flag)
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestGetFlagOrigin(t *testing.T) {
	cm := New()
	err := cm.Init()
	require.NoError(t, err)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("test-flag", "default", "test flag")

	// Test default origin
	flag := cmd.Flags().Lookup("test-flag")
	origin := cm.GetFlagOrigin(cmd, flag)
	assert.Equal(t, OriginDefault, origin)

	// Test after processing with CLI value
	err = cmd.Flags().Set("test-flag", "cli-value")
	require.NoError(t, err)
	err = cm.ProcessCommand(cmd)
	require.NoError(t, err)
	origin = cm.GetFlagOrigin(cmd, flag)
	assert.Equal(t, OriginCLI, origin)
}

func TestConfigurationManager_ProcessCommand_Idempotent(t *testing.T) {
	cm := New()
	err := cm.Init()
	require.NoError(t, err)
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("flag", "value", "")

	// First call
	err1 := cm.ProcessCommand(cmd)
	assert.NoError(t, err1)
	assert.True(t, cm.processed)

	// Second call should be idempotent
	err2 := cm.ProcessCommand(cmd)
	assert.NoError(t, err2)
	assert.True(t, cm.processed)
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"fmt"

	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/spf13/cobra"
)

var (
	authLoginDevice    bool
	authLoginNoBrowser bool
)

var runOfficialAuth = llm.RunOAuthAuth

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Sign in with ChatGPT and official provider CLI authentication",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with ChatGPT or through an official provider CLI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if official, err := runOfficialProviderAuth(cmd, "login"); official {
			return err
		}
		if err := validateAuthProvider(); err != nil {
			return err
		}
		return runChatGPTLogin(cmd.Context(), cmd.OutOrStdout())
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show saved ChatGPT registrations or official provider CLI login status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if official, err := runOfficialProviderAuth(cmd, "status"); official {
			return err
		}
		if err := validateAuthProvider(); err != nil {
			return err
		}
		return runChatGPTStatus(cmd.OutOrStdout())
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the ChatGPT session or log out of an official provider CLI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if official, err := runOfficialProviderAuth(cmd, "logout"); official {
			return err
		}
		if err := validateAuthProvider(); err != nil {
			return err
		}
		return runChatGPTLogout(cmd.Context(), cmd.OutOrStdout())
	},
}

// runOfficialProviderAuth hands codex-oauth and anthropic-oauth to their
// official CLIs, which own those credential stores. The ChatGPT registration
// flags have no meaning there, so they are rejected rather than ignored.
func runOfficialProviderAuth(cmd *cobra.Command, action string) (bool, error) {
	preset, ok := llm.LookupProvider(authProvider)
	if !ok || !llm.IsOAuthProtocol(preset.Protocol) {
		return false, nil
	}
	for _, name := range []string{"account", "new-account", "enable-plan", "no-browser"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return true, fmt.Errorf("--%s applies only to --provider chatgpt", name)
		}
	}
	device, _ := cmd.Flags().GetBool("device")
	return true, runOfficialAuth(cmd.Context(), authProvider, action, device, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	authLoginCmd.Flags().BoolVar(&authLoginDevice, "device", false, "use device authentication (codex-oauth only; the ChatGPT preview uses the local browser loopback flow)")
	authLoginCmd.Flags().BoolVar(&authLoginNoBrowser, "no-browser", false, "print a one-time local URL instead of opening a browser (chatgpt only)")
	authLoginCmd.MarkFlagsMutuallyExclusive("device", "no-browser")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/chatgptauth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAuthDefaultsToChatGPT(t *testing.T) {
	flag := authCmd.PersistentFlags().Lookup("provider")
	if flag == nil || flag.DefValue != "chatgpt" {
		t.Fatalf("provider flag = %v, want default chatgpt", flag)
	}
	commandChatGPTAccount(t)
	oldProvider, oldLogin, oldLogout := authProvider, loginChatGPT, logoutChatGPT
	t.Cleanup(func() { authProvider, loginChatGPT, logoutChatGPT = oldProvider, oldLogin, oldLogout })
	authProvider = flag.DefValue
	loginCalled, logoutCalled := false, false
	loginChatGPT = func(context.Context, chatgptauth.Store, string, bool, bool, bool, io.Writer) (*chatgptauth.Auth, error) {
		loginCalled = true
		return &chatgptauth.Auth{Scopes: strings.Fields(chatgptauth.Scope)}, nil
	}
	logoutChatGPT = func(context.Context, chatgptauth.Store, string) error {
		logoutCalled = true
		return nil
	}
	var out bytes.Buffer
	for _, cmd := range []*cobra.Command{authLoginCmd, authStatusCmd, authLogoutCmd} {
		oldOut, oldContext := cmd.OutOrStdout(), cmd.Context()
		cmd.SetOut(&out)
		cmd.SetContext(context.Background())
		err := cmd.RunE(cmd, nil)
		cmd.SetOut(oldOut)
		cmd.SetContext(oldContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !loginCalled || !logoutCalled || !strings.Contains(out.String(), "Using ChatGPT plan") || !strings.Contains(out.String(), "local tokens cleared") {
		t.Fatalf("login=%t logout=%t output=%q", loginCalled, logoutCalled, out.String())
	}
}

func TestAuthRejectsUnsupportedProviderWithoutAccountFlags(t *testing.T) {
	old := authProvider
	t.Cleanup(func() { authProvider = old })
	authProvider = "codex"
	if err := validateAuthProvider(); err == nil || !strings.Contains(err.Error(), "expected chatgpt") {
		t.Fatalf("validateAuthProvider = %v", err)
	}
	authProvider = "codex-oauth"
	if err := validateAuthProvider(); err == nil || !strings.Contains(err.Error(), "official CLI") {
		t.Fatalf("validateAuthProvider = %v", err)
	}
}

func TestAuthLoginFlagsAreMutuallyExclusive(t *testing.T) {
	rootCmd.SetArgs([]string{"auth", "login", "--device", "--no-browser"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		for _, name := range []string{"device", "no-browser"} {
			flag := authLoginCmd.Flags().Lookup(name)
			_ = flag.Value.Set("false")
			flag.Changed = false
		}
	})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "group [device no-browser]") {
		t.Errorf("Execute error = %v", err)
	}
}

// executeAuthCommand runs the global command tree and restores every auth
// flag afterwards, because cobra keeps parsed values on the shared commands.
func executeAuthCommand(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	defer func() {
		rootCmd.SetArgs(nil)
		for _, flags := range []*pflag.FlagSet{authCmd.PersistentFlags(), authLoginCmd.Flags(), authStatusCmd.Flags(), authLogoutCmd.Flags()} {
			flags.VisitAll(func(flag *pflag.Flag) {
				_ = flag.Value.Set(flag.DefValue)
				flag.Changed = false
			})
		}
	}()
	return rootCmd.Execute()
}

func TestAuthOfficialProvidersUseTheirCLI(t *testing.T) {
	old := runOfficialAuth
	t.Cleanup(func() { runOfficialAuth = old })
	var got []string
	runOfficialAuth = func(_ context.Context, provider, action string, device bool, _ io.Reader, _, _ io.Writer) error {
		got = append(got, fmt.Sprintf("%s %s %t", provider, action, device))
		return nil
	}
	for _, args := range [][]string{
		{"auth", "login", "--provider", "codex-oauth", "--device"},
		{"auth", "login", "--provider", "anthropic-oauth"},
		{"auth", "status", "--provider", "anthropic-oauth"},
		{"auth", "logout", "--provider", "codex-oauth"},
	} {
		if err := executeAuthCommand(t, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	want := []string{"codex-oauth login true", "anthropic-oauth login false", "anthropic-oauth status false", "codex-oauth logout false"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("official auth calls = %q, want %q", got, want)
	}
	if authProvider != "chatgpt" {
		t.Fatalf("provider flag leaked between commands: %q", authProvider)
	}
}

func TestAuthOfficialProvidersRejectChatGPTFlags(t *testing.T) {
	old := runOfficialAuth
	t.Cleanup(func() { runOfficialAuth = old })
	runOfficialAuth = func(context.Context, string, string, bool, io.Reader, io.Writer, io.Writer) error {
		t.Fatal("official CLI ran with a ChatGPT-only flag")
		return nil
	}
	for _, args := range [][]string{
		{"auth", "login", "--provider", "codex-oauth", "--enable-plan"},
		{"auth", "login", "--provider", "anthropic-oauth", "--no-browser"},
		{"auth", "login", "--provider", "codex-oauth", "--new-account"},
		{"auth", "status", "--provider", "codex-oauth", "--account", "oaiapp_one"},
	} {
		if err := executeAuthCommand(t, args...); err == nil || !strings.Contains(err.Error(), "applies only to --provider chatgpt") {
			t.Fatalf("%v: error = %v", args, err)
		}
	}
}

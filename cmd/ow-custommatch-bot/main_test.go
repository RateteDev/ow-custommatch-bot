package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeBotRunner struct {
	runFunc          func(token string) error
	setReadyNotifier func(fn func())
}

func (f *fakeBotRunner) Run(token string) error {
	if f.runFunc != nil {
		return f.runFunc(token)
	}
	return nil
}

func (f *fakeBotRunner) SetReadyNotifier(fn func()) {
	if f.setReadyNotifier != nil {
		f.setReadyNotifier(fn)
	}
}

func TestPromptBotToken(t *testing.T) {
	t.Run("正常入力", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got, err := promptBotToken(ui, strings.NewReader("mytoken\n"))
		if err != nil {
			t.Fatalf("promptBotToken returned error: %v", err)
		}
		if got != "mytoken" {
			t.Fatalf("promptBotToken = %q, want %q", got, "mytoken")
		}
		if !strings.Contains(out.String(), "BOT_TOKEN を入力してください") {
			t.Fatalf("prompt was not written: %q", out.String())
		}
	})

	t.Run("空入力", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("\n")); err == nil {
			t.Fatalf("expected error for empty input")
		}
	})

	t.Run("EOF", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("")); err == nil {
			t.Fatalf("expected error for EOF input")
		}
	})
}

func TestAppDataDir(t *testing.T) {
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
	})

	t.Run("windows with LOCALAPPDATA", func(t *testing.T) {
		runtimeGOOS = "windows"
		t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)

		got, err := appDataDir(appName)
		if err != nil {
			t.Fatalf("appDataDir returned error: %v", err)
		}
		want := filepath.Join(`C:\Users\tester\AppData\Local`, appName)
		if got != want {
			t.Fatalf("appDataDir = %q, want %q", got, want)
		}
	})

	t.Run("windows without LOCALAPPDATA", func(t *testing.T) {
		runtimeGOOS = "windows"
		t.Setenv("LOCALAPPDATA", "")

		if _, err := appDataDir(appName); err == nil {
			t.Fatalf("expected error when LOCALAPPDATA is empty")
		}
	})

	t.Run("non-windows", func(t *testing.T) {
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := appDataDir(appName)
		if err != nil {
			t.Fatalf("appDataDir returned error: %v", err)
		}
		want := filepath.Join(home, ".local", "share", appName)
		if got != want {
			t.Fatalf("appDataDir = %q, want %q", got, want)
		}
	})
}

func TestParseCLIArgs(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--help"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.showHelp {
			t.Fatalf("expected showHelp to be true")
		}
	})

	t.Run("version", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--version"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.showVersion {
			t.Fatalf("expected showVersion to be true")
		}
	})

	t.Run("test flag", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--test"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.testMode {
			t.Fatalf("expected testMode to be true")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		if _, err := parseCLIArgs([]string{"--unknown"}); err == nil {
			t.Fatalf("expected error for unknown flag")
		}
	})
}

func TestCLIUsageText(t *testing.T) {
	text := cliUsageText("ow-custommatch-bot")
	for _, want := range []string{"使い方", "--help", "--version", "--test", "BOT_TOKEN", "Credential Manager", guideURL} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage text missing %q: %s", want, text)
		}
	}
}

func TestRunShowVersion(t *testing.T) {
	origVersion := version
	t.Cleanup(func() {
		version = origVersion
	})

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "tag version", version: "v1.2.3", want: appName + " v1.2.3\n"},
		{name: "prerelease rc tag version", version: "v1.2.3-rc1", want: appName + " v1.2.3-rc1\n"},
		{name: "prerelease beta tag version", version: "v1.2.3-beta.1", want: appName + " v1.2.3-beta.1\n"},
		{name: "dev sha version", version: "dev-abcdef0", want: appName + " dev-abcdef0\n"},
		{name: "legacy git describe version", version: "v1.2.3-30-gabcdef0", want: appName + " v1.2.3\n"},
		{name: "raw short sha version", version: "abcdef0", want: appName + " dev-abcdef0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version = tt.version

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run(--version) exit code = %d, want 0", code)
			}
			if stdout.String() != tt.want {
				t.Fatalf("run(--version) output = %q, want %q", stdout.String(), tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("run(--version) stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "tag version", raw: "v1.2.3", want: "v1.2.3"},
		{name: "prerelease rc tag version", raw: "v1.2.3-rc1", want: "v1.2.3-rc1"},
		{name: "prerelease beta tag version", raw: "v1.2.3-beta.1", want: "v1.2.3-beta.1"},
		{name: "dev sha version", raw: "dev-abcdef0", want: "dev-abcdef0"},
		{name: "legacy git describe version", raw: "v1.2.3-30-gabcdef0", want: "v1.2.3"},
		{name: "legacy git describe dirty version", raw: "v1.2.3-30-gabcdef0-dirty", want: "v1.2.3"},
		{name: "raw short sha version", raw: "abcdef0", want: "dev-abcdef0"},
		{name: "empty version", raw: "", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayVersion(tt.raw); got != tt.want {
				t.Fatalf("displayVersion(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTokenResolution(t *testing.T) {
	origRead := readTokenFromStoreFn
	origSave := saveTokenToStoreFn
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		readTokenFromStoreFn = origRead
		saveTokenToStoreFn = origSave
		hasInteractiveConsole = origConsole
	})

	t.Run("store hit", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return false
		}

		got, err := resolveToken(strings.NewReader(""), startupUI{out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("resolveToken returned error: %v", err)
		}
		if got != "stored-token" {
			t.Fatalf("resolveToken = %q, want %q", got, "stored-token")
		}
	})

	t.Run("store miss prompts and saves", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saved := ""
		saveTokenToStoreFn = func(token string) error {
			saved = token
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return true
		}

		got, err := resolveToken(strings.NewReader("prompted-token\n"), startupUI{out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("resolveToken returned error: %v", err)
		}
		if got != "prompted-token" {
			t.Fatalf("resolveToken = %q, want %q", got, "prompted-token")
		}
		if saved != "prompted-token" {
			t.Fatalf("saved token = %q, want %q", saved, "prompted-token")
		}
	})

	t.Run("non interactive store miss", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return false
		}

		if _, err := resolveToken(strings.NewReader(""), startupUI{out: &bytes.Buffer{}}); !errors.Is(err, errTokenNotFound) {
			t.Fatalf("resolveToken error = %v, want errTokenNotFound", err)
		}
	})

	t.Run("save failure", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saveTokenToStoreFn = func(token string) error {
			return errTokenStoreUnsupported
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return true
		}

		if _, err := resolveToken(strings.NewReader("prompted-token\n"), startupUI{out: &bytes.Buffer{}}); err == nil || !strings.Contains(err.Error(), "保存") {
			t.Fatalf("expected save error, got %v", err)
		}
	})
}

func TestPromptStartupMode(t *testing.T) {
	t.Run("test mode selected", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got := promptStartupMode(ui, strings.NewReader("2\n"), 50*time.Millisecond)
		if !got {
			t.Fatalf("expected test mode to be selected")
		}
		output := out.String()
		for _, want := range []string{"通常運用", "動作確認用", "Enterキーで通常運用を開始できます"} {
			if !strings.Contains(output, want) {
				t.Fatalf("prompt output missing %q: %q", want, output)
			}
		}
		for _, unwanted := range []string{"自動で通常運用を開始します", "未入力の場合は"} {
			if strings.Contains(output, unwanted) {
				t.Fatalf("prompt output should not contain %q: %q", unwanted, output)
			}
		}
		for _, unwanted := range []string{"本番モード", "テストモード"} {
			if strings.Contains(output, unwanted) {
				t.Fatalf("prompt output should not contain %q: %q", unwanted, output)
			}
		}
	})

	t.Run("production mode selected", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}

		got := promptStartupMode(ui, strings.NewReader("1\n"), 50*time.Millisecond)
		if got {
			t.Fatalf("expected production mode to be selected")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}

		got := promptStartupMode(ui, strings.NewReader("\n"), 50*time.Millisecond)
		if got {
			t.Fatalf("expected empty input to select production mode")
		}
	})
}

func TestPromptStartupAction(t *testing.T) {
	t.Run("トークン上書きを選べる", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got := promptStartupAction(ui, strings.NewReader("3\n"), 50*time.Millisecond)
		if got != startupActionOverwriteToken {
			t.Fatalf("promptStartupAction() = %v, want %v", got, startupActionOverwriteToken)
		}
		output := out.String()
		for _, want := range []string{"[1] 通常運用", "[2] 動作確認用", "[3] トークンを上書きする"} {
			if !strings.Contains(output, want) {
				t.Fatalf("prompt output missing %q: %q", want, output)
			}
		}
	})

	t.Run("無効入力後に再入力できる", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out, err: &out}

		got := promptStartupAction(ui, strings.NewReader("4\nabc\n2\n"), 50*time.Millisecond)
		if got != startupActionStartTest {
			t.Fatalf("promptStartupAction() = %v, want %v", got, startupActionStartTest)
		}
		output := out.String()
		if strings.Count(output, "起動方法を選択してください") < 3 {
			t.Fatalf("prompt should be shown again after invalid input: %q", output)
		}
		if !strings.Contains(output, "1 / 2 / 3 / Enter のいずれかを入力してください") {
			t.Fatalf("prompt output missing invalid input guidance: %q", output)
		}
	})

	t.Run("自動起動しない", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		reader, writer := io.Pipe()
		defer reader.Close()
		done := make(chan startupAction, 1)

		go func() {
			done <- promptStartupAction(ui, reader, time.Millisecond)
		}()

		select {
		case got := <-done:
			t.Fatalf("promptStartupAction() returned unexpectedly: %v", got)
		case <-time.After(20 * time.Millisecond):
		}

		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}

		select {
		case got := <-done:
			if got != startupActionStartProd {
				t.Fatalf("promptStartupAction() after EOF = %v, want %v", got, startupActionStartProd)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("promptStartupAction() did not finish after EOF")
		}
	})
}

func TestStartupUIPrintStartupActionMenu(t *testing.T) {
	t.Run("非カラーでは従来どおり読める", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		ui.printStartupActionMenu()

		got := out.String()
		want := "\n" +
			"  起動方法を選択してください\n" +
			"  普段そのままお使いになる場合は [1] を選んでください。\n" +
			"  表示確認や試運転をしたい場合は [2] を選んでください。\n" +
			"  保存済みトークンを更新したい場合は [3] を選んでください。\n" +
			"\n" +
			"    [1] 通常運用\n" +
			"        実際の運用として起動します。\n" +
			"    [2] 動作確認用\n" +
			"        テスト用ダミーデータで画面や流れを確認できます。\n" +
			"    [3] トークンを上書きする\n" +
			"        保存先: " + tokenStorageLocationLabel() + "\n" +
			"\n" +
			"  Enterキーで通常運用を開始できます。> "
		if got != want {
			t.Fatalf("plain menu output mismatch:\n got: %q\nwant: %q", got, want)
		}
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("plain menu should not contain ANSI escape sequences: %q", got)
		}
	})

	t.Run("カラー有効時は選択肢ごとに色分けされる", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out, style: ansiStyle{enabled: true}}

		ui.printStartupActionMenu()

		got := out.String()
		for _, want := range []string{
			"    \x1b[32m[1] 通常運用\x1b[0m\n",
			"    \x1b[33m[2] 動作確認用\x1b[0m\n",
			"    \x1b[36m[3] トークンを上書きする\x1b[0m\n",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("colored menu output missing %q: %q", want, got)
			}
		}
	})
}

func TestStartupUIPrintBanner(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "tag version", version: "v1.2.3", want: "Version: v1.2.3"},
		{name: "dev sha version", version: "dev-abcdef0", want: "Version: dev-abcdef0"},
		{name: "legacy git describe version", version: "v1.2.3-30-gabcdef0", want: "Version: v1.2.3"},
		{name: "raw short sha version", version: "abcdef0", want: "Version: dev-abcdef0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			ui := startupUI{out: &out}

			ui.printBanner(tt.version)

			got := out.String()
			for _, want := range []string{"OW CUSTOMMATCH BOT", tt.want, "ow-custommatch-bot ガイド", "\x1b]8;;" + guideURL} {
				if !strings.Contains(got, want) {
					t.Fatalf("banner output missing %q: %q", want, got)
				}
			}
		})
	}
}

func TestStartupUIExternalLink(t *testing.T) {
	t.Run("色なしでもリンクラベルとURLを含む", func(t *testing.T) {
		ui := startupUI{style: ansiStyle{enabled: false}}

		got := ui.externalLink("Discord Developer Portal", portalURL)

		if !strings.Contains(got, "Discord Developer Portal") {
			t.Fatalf("link label missing: %q", got)
		}
		if !strings.Contains(got, "\x1b]8;;") {
			t.Fatalf("hyperlink sequence missing: %q", got)
		}
		if strings.Contains(got, "("+portalURL+")") {
			t.Fatalf("raw URL suffix should not be included: %q", got)
		}
	})

	t.Run("色ありならラベルを青で装飾する", func(t *testing.T) {
		ui := startupUI{style: ansiStyle{enabled: true}}

		got := ui.externalLink("Discord Developer Portal", portalURL)

		if !strings.Contains(got, "\x1b[34mDiscord Developer Portal\x1b[0m") {
			t.Fatalf("blue label missing: %q", got)
		}
	})
}

func TestStartupUIPrintPaths(t *testing.T) {
	var out bytes.Buffer
	ui := startupUI{out: &out}

	ui.printPaths("/var/lib/owcmb", "/var/log/owcmb.log", "/var/lib/owcmb/app.sqlite")

	got := out.String()
	for _, want := range []string{"ログファイル", "データベース"} {
		if !strings.Contains(got, want) {
			t.Fatalf("paths output missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "データ保存先") {
		t.Fatalf("paths output should not contain データ保存先: %q", got)
	}
	for _, unwanted := range []string{"\ndata ", "\nlog ", "\ndb "} {
		if strings.Contains("\n"+got, unwanted) {
			t.Fatalf("paths output should not contain %q label: %q", unwanted, got)
		}
	}
}

func TestStartupUIReady(t *testing.T) {
	var out bytes.Buffer
	ui := startupUI{out: &out}

	ui.ready()

	got := out.String()
	for _, want := range []string{"Discord との接続に成功しました", "/match", "/register_rank", "/my_rank", "Ctrl+C"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ready output missing %q: %q", want, got)
		}
	}
}

func TestStartupModeConfirmationMessage(t *testing.T) {
	tests := []struct {
		name string
		mode bool
		want string
	}{
		{name: "prod", mode: false, want: "通常運用で起動します。"},
		{name: "test", mode: true, want: "動作確認用で起動します。"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startupModeConfirmationMessage(tt.mode); got != tt.want {
				t.Fatalf("startupModeConfirmationMessage(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestIsAuthenticationFailureError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "discord auth failure", err: errors.New("websocket: close 4004: Authentication failed."), want: true},
		{name: "missing 4004", err: errors.New("websocket: close 1000: Authentication failed."), want: false},
		{name: "missing auth text", err: errors.New("websocket: close 4004: connection closed"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthenticationFailureError(tt.err); got != tt.want {
				t.Fatalf("isAuthenticationFailureError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRunAuthFailureRecovery(t *testing.T) {
	origRead := readTokenFromStoreFn
	origSave := saveTokenToStoreFn
	origConsole := hasInteractiveConsole
	origNewBot := newBotFn
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		readTokenFromStoreFn = origRead
		saveTokenToStoreFn = origSave
		hasInteractiveConsole = origConsole
		newBotFn = origNewBot
		runtimeGOOS = origGOOS
	})

	t.Setenv("HOME", t.TempDir())
	runtimeGOOS = "linux"
	hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
		return true
	}

	t.Run("認証失敗時にトークン再保存して1回だけ再試行する", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saved := ""
		saveTokenToStoreFn = func(token string) error {
			saved = token
			return nil
		}

		runTokens := make([]string, 0, 2)
		readyCalls := 0
		newBotFn = func(dbPath string) (botRunner, error) {
			return &fakeBotRunner{
				runFunc: func(token string) error {
					runTokens = append(runTokens, token)
					if len(runTokens) == 1 {
						return errors.New("websocket: close 4004: Authentication failed.")
					}
					return nil
				},
				setReadyNotifier: func(fn func()) {
					readyCalls++
					if fn != nil {
						fn()
					}
				},
			}, nil
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--test"}, strings.NewReader("new-token\n"), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run() exit code = %d, want 0, stderr=%q", code, stderr.String())
		}
		if saved != "new-token" {
			t.Fatalf("saved token = %q, want %q", saved, "new-token")
		}
		if got := strings.Join(runTokens, ","); got != "stored-token,new-token" {
			t.Fatalf("run tokens = %q, want %q", got, "stored-token,new-token")
		}
		if readyCalls == 0 {
			t.Fatalf("ready notifier was not set")
		}
		output := stdout.String() + "\n" + stderr.String()
		for _, want := range []string{
			"Discord Developer Portal",
			"\x1b]8;;" + portalURL,
			"BOT_TOKEN を入力してください",
			"Discord との接続に成功しました",
		} {
			if !strings.Contains(output, want) {
				t.Fatalf("output missing %q: %q", want, output)
			}
		}
	})

	t.Run("再入力が空なら終了理由を明示する", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		runCalls := 0
		newBotFn = func(dbPath string) (botRunner, error) {
			return &fakeBotRunner{
				runFunc: func(token string) error {
					runCalls++
					return errors.New("websocket: close 4004: Authentication failed.")
				},
			}, nil
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--test"}, strings.NewReader("\n"), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit code = %d, want 1", code)
		}
		if runCalls != 1 {
			t.Fatalf("run calls = %d, want 1", runCalls)
		}
		if !strings.Contains(stderr.String(), "トークンが入力されませんでした。") {
			t.Fatalf("stderr missing empty-token guidance: %q", stderr.String())
		}
	})

	t.Run("保存失敗なら終了する", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			return errTokenStoreUnsupported
		}
		runCalls := 0
		newBotFn = func(dbPath string) (botRunner, error) {
			return &fakeBotRunner{
				runFunc: func(token string) error {
					runCalls++
					return errors.New("websocket: close 4004: Authentication failed.")
				},
			}, nil
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--test"}, strings.NewReader("next-token\n"), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit code = %d, want 1", code)
		}
		if runCalls != 1 {
			t.Fatalf("run calls = %d, want 1", runCalls)
		}
		if !strings.Contains(stderr.String(), "トークンの保存に失敗しました") {
			t.Fatalf("stderr missing save failure guidance: %q", stderr.String())
		}
	})

	t.Run("再試行後も認証失敗なら明示して終了する", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			return nil
		}
		runCalls := 0
		newBotFn = func(dbPath string) (botRunner, error) {
			return &fakeBotRunner{
				runFunc: func(token string) error {
					runCalls++
					return errors.New("websocket: close 4004: Authentication failed.")
				},
			}, nil
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--test"}, strings.NewReader("next-token\n"), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit code = %d, want 1", code)
		}
		if runCalls != 2 {
			t.Fatalf("run calls = %d, want 2", runCalls)
		}
		if !strings.Contains(stderr.String(), "再試行") {
			t.Fatalf("stderr missing retry failure guidance: %q", stderr.String())
		}
	})

	t.Run("一般的な実行エラーでは再設定フローに入らない", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		runCalls := 0
		newBotFn = func(dbPath string) (botRunner, error) {
			return &fakeBotRunner{
				runFunc: func(token string) error {
					runCalls++
					return errors.New("network timeout")
				},
			}, nil
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--test"}, strings.NewReader("replacement-token\n"), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit code = %d, want 1", code)
		}
		if runCalls != 1 {
			t.Fatalf("run calls = %d, want 1", runCalls)
		}
		if strings.Contains(stdout.String()+stderr.String(), "Discord Developer Portal") {
			t.Fatalf("unexpected auth recovery guidance: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
}

func TestDescribeTokenError(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		msg := describeTokenError(errTokenNotFound)
		if !strings.Contains(msg, "BOT_TOKEN") {
			t.Fatalf("expected BOT_TOKEN hint: %s", msg)
		}
		if !strings.Contains(msg, guideURL) {
			t.Fatalf("expected guide URL hint: %s", msg)
		}
	})

	t.Run("unsupported store", func(t *testing.T) {
		msg := describeTokenError(errTokenStoreUnsupported)
		if !strings.Contains(msg, "OS") {
			t.Fatalf("expected OS hint: %s", msg)
		}
		if !strings.Contains(msg, guideURL) {
			t.Fatalf("expected guide URL hint: %s", msg)
		}
	})
}

func TestOverwriteTokenFlow(t *testing.T) {
	origSave := saveTokenToStoreFn
	t.Cleanup(func() {
		saveTokenToStoreFn = origSave
	})

	t.Run("保存先案内を出して上書きする", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}
		saved := ""
		saveTokenToStoreFn = func(token string) error {
			saved = token
			return nil
		}

		if err := overwriteStoredToken(ui, strings.NewReader("next-token\n")); err != nil {
			t.Fatalf("overwriteStoredToken returned error: %v", err)
		}
		if saved != "next-token" {
			t.Fatalf("saved token = %q, want %q", saved, "next-token")
		}
		for _, want := range []string{"保存先", tokenStorageLocationLabel(), "保存済みトークンを更新しました"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("overwrite output missing %q: %q", want, out.String())
			}
		}
	})

	t.Run("保存失敗を返す", func(t *testing.T) {
		saveTokenToStoreFn = func(token string) error {
			return errTokenStoreUnsupported
		}
		if err := overwriteStoredToken(startupUI{out: &bytes.Buffer{}}, strings.NewReader("next-token\n")); err == nil {
			t.Fatalf("expected overwriteStoredToken to fail")
		}
	})
}

func TestDetectColorEnabled(t *testing.T) {
	if detectColorEnabled(&bytes.Buffer{}) {
		t.Fatalf("expected color to be disabled for non-file writer")
	}
}

func TestStyleConsoleLogLine(t *testing.T) {
	line := "2026/02/25 20:00:00 [INFO] [2/3] トークン読込 ... OK\n"
	styled := styleConsoleLogLine(line, ansiStyle{enabled: true})
	if !strings.Contains(styled, "\x1b[") {
		t.Fatalf("expected ANSI escape sequence in styled line: %q", styled)
	}

	plain := styleConsoleLogLine(line, ansiStyle{enabled: false})
	if plain != line {
		t.Fatalf("expected plain line unchanged")
	}
}

func TestFormatErrorMessageText(t *testing.T) {
	msg := "起動に失敗しました: BOT_TOKEN が保存されていません。初回起動時は対話入力で保存してください。詳しい手順は https://ratetedev.github.io/ow-custommatch-bot/ をご確認ください。"
	got := formatErrorMessageText(msg)

	wants := []string{
		"保存されていません。\n",
		"保存してください。\n",
		"ご確認ください。",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted message missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "\n\n") {
		t.Fatalf("formatted message should not contain double newlines: %q", got)
	}
}

func TestStartupUIPrintErrorLineFormatsMessage(t *testing.T) {
	var errOut bytes.Buffer
	ui := newStartupUI(&bytes.Buffer{}, &errOut)

	ui.printErrorLine("Aです。Bです。")

	got := errOut.String()
	if !strings.Contains(got, "Aです。\nBです。") {
		t.Fatalf("unexpected error output: %q", got)
	}
}

func TestStartupUIPrintErrorLineLinkifiesKnownURL(t *testing.T) {
	var errOut bytes.Buffer
	ui := startupUI{err: &errOut, style: ansiStyle{enabled: true}}

	ui.printErrorLine("詳しい手順は " + guideURL + " をご確認ください。")

	got := errOut.String()
	if !strings.Contains(got, "使い方ページ") {
		t.Fatalf("link label missing: %q", got)
	}
	if !strings.Contains(got, "\x1b]8;;") {
		t.Fatalf("hyperlink sequence missing: %q", got)
	}
	if strings.Contains(got, "("+guideURL+")") {
		t.Fatalf("raw URL suffix should not be included: %q", got)
	}
}

func TestIsProgressToken(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  bool
	}{
		{token: "[2/4]", want: true},
		{token: "[INFO]", want: false},
		{token: "[2m", want: false},
		{token: "[abc/4]", want: false},
	} {
		if got := isProgressToken(tc.token); got != tc.want {
			t.Fatalf("isProgressToken(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

func TestShouldPauseOnErrorExit(t *testing.T) {
	origGOOS := runtimeGOOS
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		hasInteractiveConsole = origConsole
	})

	runtimeGOOS = "windows"
	hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
		return true
	}

	if !shouldPauseOnErrorExit(1, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("expected pause on windows error exit with interactive console")
	}
	if shouldPauseOnErrorExit(0, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("did not expect pause on success exit")
	}

	runtimeGOOS = "linux"
	if shouldPauseOnErrorExit(1, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("did not expect pause on non-windows")
	}
}

func TestPauseOnErrorExit(t *testing.T) {
	origGOOS := runtimeGOOS
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		hasInteractiveConsole = origConsole
	})

	runtimeGOOS = "windows"
	hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
		return true
	}

	var out bytes.Buffer
	pauseOnErrorExit(1, bytes.NewBufferString("\n"), &out)
	if !strings.Contains(out.String(), "Enterキー") {
		t.Fatalf("expected pause message, got: %q", out.String())
	}

	out.Reset()
	pauseOnErrorExit(0, bytes.NewBufferString("\n"), &out)
	if out.Len() != 0 {
		t.Fatalf("did not expect output on success exit: %q", out.String())
	}
}

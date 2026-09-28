package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const repo = "twinaapp/twina"

// selfUpdate reports whether the updater can replace the app in place. On
// Linux the app is installed from a .deb/.rpm into /usr/bin, so there we only
// point to the release page.
var selfUpdate = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

func updatesEnabled() bool { return version != "0.0.0" }

func setupUpdates(app *application.App) error {
	gh, err := github.New(github.Config{
		Repository:    repo,
		ChecksumAsset: "SHA256SUMS.txt",
		AssetMatcher:  matchAsset,
	})
	if err != nil {
		return err
	}
	return app.Updater.Init(updater.Config{
		CurrentVersion: version,
		Providers:      []updater.Provider{gh},
	})
}

// matchAsset picks the file the release workflow publishes for this platform.
func matchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	var suffix string
	switch req.Platform {
	case "darwin":
		suffix = "-macos-universal.zip"
	case "windows":
		suffix = "-windows-" + req.Arch + "-portable.exe"
	default:
		// Only used to confirm the release has a Linux build; see selfUpdate.
		suffix = "-" + req.Platform + "-" + req.Arch + ".deb"
	}
	for i, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			return i
		}
	}
	return -1
}

// checkForUpdates asks GitHub for a newer release. manual is set when the user
// picked the menu item, so "up to date" and errors are reported as well.
func checkForUpdates(app *application.App, manual bool) {
	ctx := context.Background()
	rel, err := app.Updater.Check(ctx)
	switch {
	case err != nil:
		if manual {
			app.Dialog.Error().SetTitle("Check for Updates").SetMessage(err.Error()).Show()
		}
	case rel == nil:
		if manual {
			app.Dialog.Info().SetTitle("Check for Updates").
				SetMessage("Twina " + version + " is the latest version.").Show()
		}
	default:
		action := "Download"
		if selfUpdate {
			action = "Install"
		}
		d := app.Dialog.Question().SetTitle("Update Available").
			SetMessage(fmt.Sprintf("Twina %s is available. You have %s.", strings.TrimPrefix(rel.Version, "v"), version))
		ok := d.AddButton(action).OnClick(func() {
			if selfUpdate {
				// The updater's window shows progress, errors and the restart button.
				go func() { _ = app.Updater.CheckAndInstall(ctx) }()
			} else {
				_ = app.Browser.OpenURL("https://github.com/" + repo + "/releases/latest")
			}
		})
		later := d.AddButton("Later")
		d.SetDefaultButton(ok).SetCancelButton(later).Show()
	}
}

package main

import (
	"embed"
	"log"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[Progress](progressEvent)
}

func main() {
	files := &FileService{}
	app := application.New(application.Options{
		Name:        "Twina",
		Description: "A dual-panel file manager",
		Services: []application.Service{
			application.NewService(files),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	updates := false
	if updatesEnabled() {
		if err := setupUpdates(app); err != nil {
			log.Printf("updates disabled: %v", err)
		} else {
			updates = true
			app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
				time.AfterFunc(5*time.Second, func() { checkForUpdates(app, false) })
			})
		}
	}
	checkNow := func(*application.Context) { go checkForUpdates(app, true) }

	// A minimal menu: the defaults bind ⌘W (close window) and ⌘R (reload),
	// which the panels use for closing tabs and refreshing.
	menu := app.Menu.New()
	if runtime.GOOS == "darwin" {
		// The AppMenu role, built by hand so Check for Updates can sit under About.
		appMenu := menu.AddSubmenu("Twina")
		appMenu.AddRole(application.About)
		if updates {
			appMenu.Add("Check for Updates…").OnClick(checkNow)
		}
		appMenu.AddSeparator()
		appMenu.AddRole(application.ServicesMenu)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Hide)
		appMenu.AddRole(application.HideOthers)
		appMenu.AddRole(application.UnHide)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Quit)
	}
	menu.AddRole(application.EditMenu)
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.WindowMenu)
	} else if updates {
		menu.AddSubmenu("Help").Add("Check for Updates…").OnClick(checkNow)
	}
	app.Menu.Set(menu)

	files.emit = func(p Progress) { app.Event.Emit(progressEvent, p) }

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Twina",
		Width:     1280,
		Height:    800,
		MinWidth:  720,
		MinHeight: 420,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 38,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(24, 26, 31),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

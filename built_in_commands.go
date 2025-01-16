package goapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/mitoteam/mttools"
	"github.com/spf13/cobra"
)

func (app *AppBase) buildRootCmd() {
	app.rootCmd = &cobra.Command{
		Version: app.Version,

		//disable default 'completion' subcommand
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},

		Run: func(cmd *cobra.Command, args []string) {
			//show help if no subcommand given
			cmd.Help()
		},

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			//Load Settings
			if mttools.IsFileExists(app.AppSettingsFilename) {
				if err := app.loadSettings(); err != nil {
					return err
				}
			} else {
				//do not require settings loading just for certain commands
				if cmd.Name() != "init" && cmd.Name() != "version" {
					log.Fatalf(
						"No "+app.AppSettingsFilename+" file found. Please create one or use `%s init` command.\n", app.ExecutableName,
					)
				}
			}

			if app.PreCmdF != nil {
				if err := app.PreCmdF(cmd); err != nil {
					return err
				}
			}

			return nil
		},

		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if app.PostCmdF != nil {
				if err := app.PostCmdF(cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func (app *AppBase) buildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints the raw version number of " + app.AppName + ".",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(app.Version)
		},
	}
}

func (app *AppBase) buildInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Creates system service to run " + app.AppName,

		Run: func(cmd *cobra.Command, args []string) {
			if mttools.IsSystemdAvailable() {
				unitData := &mttools.ServiceData{
					Name:      app.baseSettings.ServiceName,
					User:      app.baseSettings.ServiceUser,
					Group:     app.baseSettings.ServiceGroup,
					Autostart: app.serviceAutostart,
				}

				if err := unitData.InstallSystemdService(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatalf(
					"Directory %s does not exists. Only systemd based services supported for now.\n",
					mttools.SystemdServiceDirPath,
				)
			}
		},
	}

	cmd.PersistentFlags().BoolVar(
		&app.serviceAutostart,
		"autostart",
		true,
		"Set service to be auto started after boot. Please note: this option does not auto starts service after installation.",
	)

	return cmd
}

func (app *AppBase) buildUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove installed system service " + app.AppName,

		Run: func(cmd *cobra.Command, args []string) {
			if mttools.IsSystemdAvailable() {
				unitData := &mttools.ServiceData{
					Name: app.baseSettings.ServiceName,
				}

				if err := unitData.UninstallSystemdService(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatalf(
					"Directory %s does not exists. Only systemd based services supported for now.\n",
					mttools.SystemdServiceDirPath,
				)
			}
		},
	}

	return cmd
}

func (app *AppBase) buildInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates settings file with defaults in working directory.",

		RunE: func(cmd *cobra.Command, args []string) error {
			if mttools.IsFileExists(app.AppSettingsFilename) {
				return errors.New("Can not initialize existing file: " + app.AppSettingsFilename)
			}

			comment := `File was created automatically by '` + app.AppName + ` init' command. There are all
available options listed here with its default values. Recommendation is to edit options you
want to change and remove all others with default values to keep this as simple as possible.
`

			if err := app.saveSettings(comment); err != nil {
				return err
			}

			fmt.Println("Default app settings written to " + app.AppSettingsFilename)

			if app.InitF != nil {
				if err := app.InitF(); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func (app *AppBase) buildInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Prints info about app, settings, status etc.",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s\n", app.AppName)
			fmt.Print("================================\n")
			fmt.Printf("Version: %s\n", app.Version)
			fmt.Printf("Commit: %s\n", app.BuildCommit)
			fmt.Printf("Built: at %s with %s\n", app.BuildTime, app.BuildWith)

			// Settings
			fmt.Print("\n================================\n")
			fmt.Print("SETTINGS\n")
			fmt.Print("================================\n")
			app.printSettings()

			if app.PrintInfoF != nil {
				app.PrintInfoF()
			}
		},
	}

	return cmd
}

func (app *AppBase) buildRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs webserver",

		RunE: func(cmd *cobra.Command, args []string) error {
			address := app.baseSettings.WebserverHostname +
				":" + strconv.FormatUint(uint64(app.baseSettings.WebserverPort), 10)

			//Graceful shutdown according to https://github.com/gorilla/mux#graceful-shutdown
			httpSrv := &http.Server{
				Addr:         address,
				WriteTimeout: time.Second * 10,
				ReadTimeout:  time.Second * 20,
				IdleTimeout:  time.Second * 60,
				Handler:      app.Handler(),
				BaseContext:  func(l net.Listener) context.Context { return app.BaseContext },
			}

			log.Printf("Starting up web server at http://%s\nPress Ctrl + C to stop it.\n", address)

			go func() {
				if err := httpSrv.ListenAndServe(); err != nil {
					log.Println(err)
				}
			}()

			cancel_channel := make(chan os.Signal, 1)

			// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
			// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
			signal.Notify(cancel_channel, os.Interrupt, os.Kill)

			// Block execution until we receive our signal.
			<-cancel_channel

			log.Println("Shutting down web server")

			// Create a deadline to wait for (10s).
			ctx, cancel := context.WithTimeout(app.BaseContext, app.ShutdownTimeout)
			defer cancel()

			if err := httpSrv.Shutdown(ctx); err != nil {
				log.Fatal("Server forced to shutdown:", err)
			}

			return nil
		},

		// Do startup procedures
		PreRunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("%s version: %s\n", app.AppName, app.Version)

			var err error

			if app.PreRunF != nil {
				err = app.PreRunF()
			}

			return err
		},

		// Do shutdown procedures
		PostRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if app.PostRunF != nil {
				err = app.PostRunF()
			}

			log.Println("Shutdown complete")

			return err
		},
	}

	//Extended query log
	cmd.PersistentFlags().BoolVar(
		&app.WebRouterLogRequests,
		"log-requests",
		false,
		"Extended web router queries logging.",
	)

	// log SQL queries
	cmd.PersistentFlags().BoolVar(
		&app.baseSettings.LogSql,
		"log-sql",
		app.baseSettings.LogSql,
		"Log SQL queries.",
	)

	return cmd
}

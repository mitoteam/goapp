package goapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mitoteam/mttools"
	"github.com/spf13/cobra"
)

const DEV_MODE_LABEL = "DEV"

// Variables to be set by compiler
var (
	BuildVersion = DEV_MODE_LABEL
	BuildCommit  = DEV_MODE_LABEL
	BuildTime    = DEV_MODE_LABEL
)

type AppBase struct {
	ExecutableName  string //executable command name
	AppName         string //Long name
	LongDescription string //Long description

	Version         string //Version (auto set by compiler)
	BuildCommitFull string //Git full commit hash
	BuildCommit     string //Git short commit hash
	BuildTime       string //Build time
	BuildWith       string //Build information

	Global map[string]interface{} //some global application state values

	AppSettingsFilename string           // with .yml extension please
	AppSettings         interface{}      //pointer to struct embedding AppSettingsBase
	baseSettings        *AppSettingsBase //pointer to *AppSettingsBase, set in internalInit()

	serviceAutostart bool

	rootCmd *cobra.Command

	//contexts and timeout settings
	BaseContext     context.Context
	ShutdownTimeout time.Duration

	//web routers
	ginEngine           *gin.Engine
	WebRouterLogQueries bool                // true = extended query logging (--query-log option of `run`)
	BuildWebRouterF     func(r *gin.Engine) // function to build web router for `run` command
	handler             http.Handler

	//web api
	WebApiPathPrefix  string // usually "/api". Leave empty to disable web API at all.
	WebApiEnableGet   bool   // Serve both POST and GET methods. Default 'false' = POST-requests only.
	webApiHandlerList map[string]ApiRequestHandler

	//callbacks (aka event handlers)
	PreCmdF  func(cmd *cobra.Command) error // called before any subcommand. Stops executions if error returned.
	PostCmdF func(cmd *cobra.Command) error // called after any subcommand. Stops executions if error returned.

	PreRunF    func() error // called before starting `run` command. Stops executions if error returned.
	PostRunF   func() error // called after finishing `run` command. Stops executions if error returned.
	InitF      func() error // Additional code for `init` subcommand. Stops executions if error returned.
	PrintInfoF func()       // Prints additional information when `info` subcommand called.

	BuildCustomCommandsF func(rootCmd *cobra.Command) // Set this to add any custom subcommands
}

// Initializes new application.
// settings - application settings default values. Pointer to struct that embeds AppSettingsBase.
func NewAppBase(defaultSettings interface{}) *AppBase {
	app := AppBase{}

	//global app state values
	app.Global = make(map[string]interface{})

	//web api routes list
	app.webApiHandlerList = make(map[string]ApiRequestHandler)

	//default settings values
	app.AppSettingsFilename = ".settings.yml"
	if defaultSettings == nil {
		log.Fatalln("defaultSettings should not be empty")
	}

	base_settings_type := reflect.TypeFor[AppSettingsBase]()

	if !mttools.IsStructEmbeds(defaultSettings, base_settings_type) {
		log.Fatalln("settings structure should embed " + base_settings_type.Name())
	}

	app.AppSettings = defaultSettings

	v := reflect.ValueOf(app.AppSettings).Elem()
	app.baseSettings = v.FieldByName(base_settings_type.Name()).Addr().Interface().(*AppSettingsBase)

	app.baseSettings.checkDefaultValues(&AppSettingsBase{
		WebserverHostname:   "localhost",
		WebserverPort:       15115,
		ServiceName:         app.ExecutableName,
		ServiceUser:         "www-data",
		ServiceGroup:        "www-data",
		InitialRootPassword: mttools.RandomString(20),
	})

	//global application base context
	app.BaseContext = context.Background()

	//compilation data
	app.Version = BuildVersion
	app.BuildCommitFull = BuildCommit
	app.BuildCommit = app.BuildCommitFull[0:min(7, len(app.BuildCommitFull))]
	app.BuildTime = BuildTime
	app.BuildWith = runtime.Version()

	//set default values
	app.ExecutableName = "UNSET_ExecutableName"
	app.AppName = "UNSET_AppName"

	app.ShutdownTimeout = 10 * time.Second

	//build root cobra cmd
	app.buildRootCmd()

	return &app
}

func (app *AppBase) Handler() http.Handler {
	if app.handler == nil {
		//default
		return app.ginEngine.Handler()
	} else {
		return app.handler
	}
}

func (app *AppBase) SetHandler(h http.Handler) {
	app.handler = h
}

func (app *AppBase) Run() {
	app.internalInit()

	//cli application - we just let cobra to do its job
	if err := app.rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func (app *AppBase) internalInit() {
	//post-setup root cmd
	app.rootCmd.Use = app.ExecutableName
	app.rootCmd.Long = app.AppName

	if app.LongDescription != "" {
		app.rootCmd.Long += " - " + app.LongDescription

	}

	app.rootCmd.PersistentFlags().StringVar(
		&app.AppSettingsFilename,
		"settings",
		app.AppSettingsFilename,
		"Filename or full path bot settings file.",
	)

	//check app options
	if app.WebApiPathPrefix != "" {
		// no trailing slashes
		app.WebApiPathPrefix = strings.TrimSuffix(app.WebApiPathPrefix, "/")

		//should start from slash
		if !strings.HasPrefix(app.WebApiPathPrefix, "/") {
			app.WebApiPathPrefix += "/"
		}
	}

	//add built-in commands
	app.rootCmd.AddCommand(
		app.buildVersionCmd(),
		app.buildInstallCmd(),
		app.buildUninstallCmd(),
		app.buildInitCmd(),
		app.buildInfoCmd(),
		app.buildRunCmd(),
	)

	if app.BuildCustomCommandsF != nil {
		app.BuildCustomCommandsF(app.rootCmd)
	}
}

func (app *AppBase) loadSettings() error {
	if mttools.IsFileExists(app.AppSettingsFilename) {
		if err := mttools.LoadYamlSettingFromFile(app.AppSettingsFilename, app.AppSettings); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("File not found: %s", app.AppSettingsFilename)
	}

	// Settings post-processing

	if app.baseSettings.Production {
		// require some settings in PRODUCTION
		if app.baseSettings.BaseUrl == "" {
			return errors.New("base_url required in production")
		}

		if app.baseSettings.WebserverCookieSecret == "" {
			return errors.New("webserver_cookie_secret required in production")
		} else if len(app.baseSettings.WebserverCookieSecret) < 32 {
			return fmt.Errorf(
				"webserver_cookie_secret should be at least 32 characters long in production. You have %d.",
				len(app.baseSettings.WebserverCookieSecret),
			)
		}

	} else {
		// or use pre-defined values in DEV
		if app.baseSettings.BaseUrl == "" {
			app.baseSettings.BaseUrl = "http://" + app.baseSettings.WebserverHostname +
				":" + strconv.Itoa(int(app.baseSettings.WebserverPort))
		}

		if app.baseSettings.WebserverCookieSecret == "" {
			app.baseSettings.WebserverCookieSecret = "DEFAULT_DEV_SECRET"
		}
	}

	return nil
}

func (app *AppBase) saveSettings(comment string) error {
	return mttools.SaveYamlSettingToFile(app.AppSettingsFilename, comment, app.AppSettings)
}

func (app *AppBase) printSettings() {
	mttools.PrintYamlSettings(app.AppSettings)
}

func (app *AppBase) ApiHandler(path string, handler ApiRequestHandler) *AppBase {
	app.webApiHandlerList[path] = handler

	return app //for method chaining
}

func (app *AppBase) IsDevMode() bool {
	return app.Version == DEV_MODE_LABEL // && false //uncomment to debug production mode
}

package goapp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func (app *AppBase) buildWebRouter() {
	//no debug logging
	gin.SetMode(gin.ReleaseMode)

	//Initialize Cookie-based session store
	sessionStore := cookie.NewStore([]byte(app.baseSettings.WebserverCookieSecret))

	// Prepare router
	app.ginEngine = gin.New()

	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	app.ginEngine.Use(gin.Recovery())

	// use session store
	app.ginEngine.Use(sessions.Sessions(app.ExecutableName, sessionStore))

	//extended logging if requested
	if app.WebRouterLogQueries {
		app.ginEngine.Use(gin.Logger())
		log.Println("Extended queries logging enabled.")
	}

	//API routes
	if app.WebApiPathPrefix != "" {
		app.ginEngine.POST("/api/*any", (app).webApiRequestGinHandler)

		if app.WebApiEnableGet {
			app.ginEngine.GET("/api/*any", (app).webApiRequestGinHandler)
		}
	}

	// user provided routes
	if app.BuildWebRouterF != nil {
		app.BuildWebRouterF(app.ginEngine)
	}
}

func (app *AppBase) webApiRequestGinHandler(c *gin.Context) {
	var (
		api_request *ApiRequest
		err         error
	)

	path := strings.TrimPrefix(c.Request.URL.Path, app.WebApiPathPrefix)
	api_request, err = newApiRequest(c)

	if err == nil {
		if handler, ok := app.webApiHandlerList[path]; ok {
			err = handler(api_request)
		} else {
			err = fmt.Errorf("path '%s' not found", path)
		}
	}

	if err != nil {
		log.Println("API Request error: ", err)
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	// do not leave status unset
	if api_request.GetOutData("status") == "" {
		api_request.SetOkStatus(api_request.GetOutData("message"))
	}

	//prepare reply
	c.Writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(c.Writer).Encode(api_request.outData)
}

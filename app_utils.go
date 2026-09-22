package goapp

import (
	"log"
)

func IsDev() bool {
	return BuildVersion == DEV_MODE_LABEL // && false //uncomment to debug production mode
}

func PrintDev(msg string) {
	if IsDev() {
		log.Println("[DEV] " + msg)
	}
}

package main

import (
	"encoding/json"
	"os"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/swagger"
)

func main() {
	routes := server.OpenAPIRouteSupersetForBuild()
	apiVersion := config.DefaultApplicationConfig().Server.APIVersion
	spec := swagger.BuildSpecWithRoutes("dev", apiVersion, "en", routes)
	body, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		panic(err)
	}
	body = append(body, '\n')
	if err := os.WriteFile("src/swagger/openapi.json", body, 0o644); err != nil {
		panic(err)
	}
}

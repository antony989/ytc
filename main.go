package main

import (
	"github.com/kurosaki/l1/internal/routes"
)

func main() {
	router := routes.New()
	router.Logger.Fatal(router.Start(":9988"))
}

package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	v1 "github.com/jaskaranSM/transfer-service/api/v1"
	"github.com/jaskaranSM/transfer-service/config"
	"github.com/jaskaranSM/transfer-service/utils"
)

func main() {
	cfg := config.Get()

	// Parse command-line flags
	flag.Parse()

	// Create fiber app
	app := fiber.New(fiber.Config{Prefork: cfg.Prefork, ErrorHandler: utils.HandleError})

	// Add Middlewares
	app.Use(cors.New())
	app.Use(recover.New())

	// Create a /api/v1 endpoint
	v1endpoint := app.Group("/api/v1")

	// Bind handlers
	v1.AddRoutes(v1endpoint)

	// Listen on port 3000
	log.Fatal(app.Listen(fmt.Sprintf(":%d", cfg.Port))) // APP_PORT=3000 go run app.go
	//parent := "1MqwvxpG-mcKvLwayk2WMwnc_zdiQWtOx"
	//client := manager.NewGoogleDriveManager()
	// _, err := client.AddUpload(&manager.AddUploadOpts{
	// 	Path:               "D:/Evan Call - APPARE-RANMAN! ORIGINAL SOUNDTRACK Appare Tokidoki Kosame MP3/",
	// 	ParentId:           parent,
	// 	Size:               0,
	// 	Concurrency:        10,
	// 	CleanAfterComplete: false,
	// })

	// _, err := client.AddClone(&manager.AddCloneOpts{
	// 	FileId:      "1NG7lJjAb9BiQsM1EauHA6bUedUKYmGBW",
	// 	DesId:       "1MqwvxpG-mcKvLwayk2WMwnc_zdiQWtOx",
	// 	Concurrency: 20,
	// 	Size:        0,
	// })
	// _, err := client.AddDownload(&manager.AddDownloadOpts{
	// 	FileId:      "1MqwvxpG-mcKvLwayk2WMwnc_zdiQWtOx",
	// 	LocalDir:    ".",
	// 	Size:        0,
	// 	Concurrency: 10,
	// })
}

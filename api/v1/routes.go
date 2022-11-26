package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
)

func AddRoutes(router fiber.Router) {
	gdmanager := manager.NewGoogleDriveManager()
	router.Get(
		"/Hello",
		HelloHandler,
	)
	router.Post(
		"/clone",
		func(c *fiber.Ctx) error {
			return CloneHandler(c, gdmanager)
		},
	)
	router.Get(
		"/transferstatus/:gid",
		func(c *fiber.Ctx) error {
			return StatusHandler(c, gdmanager)
		},
	)
	router.Post(
		"/upload",
		func(c *fiber.Ctx) error {
			return UploadHandler(c, gdmanager)
		},
	)
	router.Post(
		"/download",
		func(c *fiber.Ctx) error {
			return DownloadHandler(c, gdmanager)
		},
	)

}

package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/service/gdrive"
)

func FileMetdataHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	fileId := ctx.Params("fileId")
	if fileId == "" {
		ctx.SendStatus(401)
		return ctx.JSON(fiber.Map{
			"error": "provide fileId in param, bad request",
		})
	}

	client := gdrive.NewGoogleDriveClient(1, 0, nil)
	err := client.Authorize()
	if err != nil {
		ctx.SendStatus(500)
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	file, err := client.GetFileMetadata(fileId)
	if err != nil {
		ctx.SendStatus(404)
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	rtr := fiber.Map{
		"file": file,
	}
	if err != nil {
		rtr["error"] = err.Error()
	}
	return ctx.JSON(rtr)
}

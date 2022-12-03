package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/service/gdrive"
	"github.com/jaskaranSM/transfer-service/types"
)

func ListFilesHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	var listFilesRequest types.ListFilesRequest
	err := ctx.BodyParser(&listFilesRequest)
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	client := gdrive.NewGoogleDriveClient(1, 0, nil)
	err = client.Authorize()
	if err != nil {
		ctx.SendStatus(500)
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	files, err := client.ListFilesByParentId(listFilesRequest.ParentID, listFilesRequest.Name, listFilesRequest.Count)
	if err != nil {
		ctx.SendStatus(404)
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	rtr := fiber.Map{
		"files": files,
	}
	if err != nil {
		rtr["error"] = err.Error()
	}
	return ctx.JSON(rtr)
}

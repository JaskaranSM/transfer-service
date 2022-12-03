package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/types"
)

func CloneHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	var cloneRequest types.CloneRequest
	err := ctx.BodyParser(&cloneRequest)
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	gid, err := gdmanager.AddClone(&manager.AddCloneOpts{
		FileId:      cloneRequest.FileId,
		DesId:       cloneRequest.DesId,
		Concurrency: cloneRequest.Concurrency,
		Size:        cloneRequest.Size,
	})
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{
		"gid": gid,
	})
}

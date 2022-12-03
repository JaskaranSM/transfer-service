package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/types"
)

func CancelHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	var cancelRequest types.CancelRequest
	err := ctx.BodyParser(&cancelRequest)
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	status := gdmanager.GetTransferStatusByGid(cancelRequest.Gid)
	if status == nil {
		ctx.SendStatus(404)
		return ctx.JSON(fiber.Map{
			"error": "gid not found in manager",
		})
	}
	status.Cancel()
	return ctx.JSON(fiber.Map{
		"gid": cancelRequest.Gid,
	})
}

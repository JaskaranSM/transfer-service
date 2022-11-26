package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
)

func StatusHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	gid := ctx.Params("gid")
	if gid == "" {
		ctx.SendStatus(401)
		return ctx.JSON(fiber.Map{
			"error": "provide gid in param, bad request",
		})
	}

	status := gdmanager.GetTransferStatusByGid(gid)
	if status == nil {
		ctx.SendStatus(404)
		return ctx.JSON(fiber.Map{
			"error": "gid not found in manager",
		})
	}
	err := status.GetFailureError()
	rtr := fiber.Map{
		"gid":              gid,
		"total_length":     status.TotalLength(),
		"completed_length": status.CompletedLength(),
		"is_completed":     status.IsCompleted(),
		"is_failed":        status.IsFailed(),
		"speed":            status.Speed(),
		"transfer_type":    status.GetTransferType(),
		"name":             status.Name(),
	}
	if err != nil {
		rtr["error"] = err.Error()
	}
	return ctx.JSON(rtr)
}

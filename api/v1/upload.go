package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/types"
)

func UploadHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	var uploadRequest types.UploadRequest
	err := ctx.BodyParser(&uploadRequest)
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	gid, err := gdmanager.AddUpload(&manager.AddUploadOpts{
		Path:        uploadRequest.Path,
		ParentId:    uploadRequest.ParentId,
		Concurrency: uploadRequest.Concurrency,
		Size:        uploadRequest.Size,
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

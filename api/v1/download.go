package v1

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jaskaranSM/transfer-service/manager"
	"github.com/jaskaranSM/transfer-service/types"
)

func DownloadHandler(ctx *fiber.Ctx, gdmanager *manager.GoogleDriveManager) error {
	var downloadRequest types.DownloadRequest
	err := ctx.BodyParser(&downloadRequest)
	if err != nil {
		return ctx.JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	gid, err := gdmanager.AddDownload(&manager.AddDownloadOpts{
		FileId:      downloadRequest.FileId,
		LocalDir:    downloadRequest.LocalDir,
		Concurrency: downloadRequest.Concurrency,
		Size:        downloadRequest.Size,
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

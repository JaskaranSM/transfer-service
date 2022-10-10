package utils

import (
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/jaskaranSM/transfer-service/logging"
)

func GetFileContentTypePath(file_path string) (string, error) {
	file, err := os.Open(file_path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return GetFileContentType(file)
}

func GetFileContentType(out *os.File) (string, error) {
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}

func HandleError(ctx *fiber.Ctx, err error) error {
	logger := logging.GetLogger()
	logger.Error("Error occurred", zap.Error(err))
	return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{"Detail": "internal server error"})
}

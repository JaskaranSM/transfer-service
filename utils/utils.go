package utils

import (
	"math/rand"
	"net/http"
	"os"
	"path/filepath"

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

func GetFolderSize(filePath string) (int64, error) {
	var size int64
	err := filepath.Walk(filePath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func GetPathSize(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if fileInfo.IsDir() {
		return GetFolderSize(filePath)
	}
	return fileInfo.Size(), nil
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

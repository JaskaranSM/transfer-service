package v1

import "github.com/gofiber/fiber/v2"

func HelloHandler(ctx *fiber.Ctx) error {
	return ctx.Status(200).JSON(fiber.Map{"detail": "Fuck you"})
}

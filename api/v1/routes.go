package v1

import "github.com/gofiber/fiber/v2"

func AddRoutes(router fiber.Router) {
	router.Get(
		"/Hello",
		HelloHandler,
	)
}

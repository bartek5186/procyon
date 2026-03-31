package controllers

import (
	"context"
	"net/http"

	"github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/models"
	"github.com/bartek5186/procyon/services"
	"github.com/labstack/echo/v4"
	ory "github.com/ory/client-go"
	"github.com/sirupsen/logrus"
)

type HelloController struct {
	appService *services.AppService
	logger     *logrus.Logger
}

func NewHelloController(appService *services.AppService, logger *logrus.Logger) *HelloController {
	return &HelloController{
		appService: appService,
		logger:     logger,
	}
}

func (c *HelloController) Health(ec echo.Context) error {
	ctx := c.withLanguage(ec)
	return ec.JSON(http.StatusOK, c.appService.Hello.Health(ctx))
}

func (c *HelloController) Hello(ec echo.Context) error {
	ctx := c.withLanguage(ec)

	var in models.HelloInput
	if err := ec.Bind(&in); err != nil {
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if err := ec.Validate(&in); err != nil {
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	out, err := c.appService.Hello.Greet(ctx, in, "")
	if err != nil {
		c.logger.WithError(err).Error("Hello")
		return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return ec.JSON(http.StatusOK, out)
}

func (c *HelloController) HelloAuthenticated(ec echo.Context) error {
	ctx := c.withLanguage(ec)

	var in models.HelloInput
	if err := ec.Bind(&in); err != nil {
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if err := ec.Validate(&in); err != nil {
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	sess := ec.Get(middleware.ContextKeySession).(*ory.Session)
	out, err := c.appService.Hello.Greet(ctx, in, sess.Identity.Id)
	if err != nil {
		c.logger.WithError(err).Error("HelloAuthenticated")
		return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return ec.JSON(http.StatusOK, out)
}

func (c *HelloController) withLanguage(ec echo.Context) context.Context {
	ctx := ec.Request().Context()

	if lang, ok := ec.Get("lang").(string); ok && lang != "" {
		ctx = context.WithValue(ctx, middleware.KeyLang{}, lang)
	}
	if cands, ok := ec.Get("langCandidates").([]string); ok && len(cands) > 0 {
		ctx = context.WithValue(ctx, middleware.KeyLangCandidates{}, cands)
	}

	return ctx
}

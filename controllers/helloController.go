package controllers

import (
	"context"
	"net/http"

	"github.com/bartek5186/procyon/internal/apierr"
	"github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/models"
	"github.com/bartek5186/procyon/services"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type HelloController struct {
	appService *services.AppService
	logger     *zap.Logger
}

func NewHelloController(appService *services.AppService, logger *zap.Logger) *HelloController {
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
		return apierr.ReplyBadRequest(ec, "invalid payload")
	}
	if err := ec.Validate(&in); err != nil {
		return apierr.ReplyValidation(ec, err)
	}

	out, err := c.appService.Hello.Greet(ctx, in, "")
	if err != nil {
		c.logger.Error("hello request failed", zap.Error(err))
		return apierr.Reply(ec, err)
	}

	return ec.JSON(http.StatusOK, out)
}

func (c *HelloController) HelloAuthenticated(ec echo.Context) error {
	out, err := c.greetAuthenticated(ec)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, out)
}

func (c *HelloController) HelloAdmin(ec echo.Context) error {
	out, err := c.greetAuthenticated(ec)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, map[string]any{
		"scope": "hello:manage",
		"data":  out,
	})
}

func (c *HelloController) greetAuthenticated(ec echo.Context) (*models.HelloResponse, error) {
	ctx := c.withLanguage(ec)

	var in models.HelloInput
	if err := ec.Bind(&in); err != nil {
		return nil, apierr.ReplyBadRequest(ec, "invalid payload")
	}
	if err := ec.Validate(&in); err != nil {
		return nil, apierr.ReplyValidation(ec, err)
	}

	sess, ok := middleware.SessionFromContext(ec)
	if !ok || sess == nil {
		return nil, apierr.ReplyUnauthorized(ec, "unauthorized")
	}

	out, err := c.appService.Hello.Greet(ctx, in, sess.Identity.Id)
	if err != nil {
		c.logger.Error("authenticated hello request failed", zap.Error(err))
		return nil, apierr.Reply(ec, err)
	}

	if role, ok := middleware.RoleFromContext(ec); ok {
		out.Role = role
	}

	return out, nil
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

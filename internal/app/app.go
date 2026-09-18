package app

import (
	"context"
	"wind-agent/internal/config"
	"wind-agent/internal/core"
	"wind-agent/internal/core/tools"
	"wind-agent/internal/data"
	"wind-agent/internal/service"
	"wind-agent/internal/session"
	"wind-agent/internal/transport/http_api"
	"wind-agent/internal/transport/qqbot"

	"golang.org/x/sync/errgroup"
)

type Application struct {
	AppCfg config.App
	Data   *data.Data
}

func New(appCfg config.App, d *data.Data) (*Application, error) {
	return &Application{AppCfg: appCfg, Data: d}, nil
}

func Run(ctx context.Context, cfgPath string) error {
	app, cleanup, err := InitApp(cfgPath)
	if err != nil {
		return err
	}
	defer cleanup()
	return app.run(ctx)
}

func (app *Application) run(ctx context.Context) error {
	registry := core.NewRegistry(app.AppCfg.Providers)

	// 工具注入
	w := app.AppCfg.Weather
	var toolList []core.Tool
	toolList = append(toolList, tools.Bash{})
	toolList = append(toolList, tools.WebSearch{})
	toolList = append(toolList, tools.Weather{
		Host:    w.Host,
		GeoHost: w.GeoHost,
		Key:     w.Key,
	})
	toolReg := core.NewToolRegistry(toolList...)

	ag := session.NewAgent(registry, toolReg, app.Data.Session)
	svc := service.NewSessionService(app.Data.Session, ag)

	srvList := app.buildTransport(svc)
	g, ctx := errgroup.WithContext(ctx)
	for _, srv := range srvList {
		g.Go(func() error {
			return srv.Run(ctx)
		})
	}
	return g.Wait()
}

type Server interface {
	Run(ctx context.Context) error
}

func (app *Application) buildTransport(svc *service.SessionService) []Server {
	var srv []Server
	tCfg := app.AppCfg.Transports
	if tCfg.HTTP.Enable {
		srv = append(srv, http_api.NewServer(svc, tCfg.HTTP.Address))
	}
	if tCfg.QQ.Enable {
		client := qqbot.NewClient(qqbot.ClientConfig{
			AppID:  tCfg.QQ.AppID,
			Secret: tCfg.QQ.Secret,
		})
		srv = append(srv, qqbot.NewGateway(svc, client, qqbot.Config{
			Enable: true,
			AppID:  tCfg.QQ.AppID,
			Secret: tCfg.QQ.Secret,
		}))
	}
	return srv
}

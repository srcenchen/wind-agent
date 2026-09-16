package app

import (
	"context"
	"wind-agent/internal/config"
	"wind-agent/internal/session"
	"wind-agent/internal/transport/http_api"

	"golang.org/x/sync/errgroup"
)

type Application struct {
	AppCfg *config.App
}

func New(appCfg *config.App) (*Application, error) {
	return &Application{AppCfg: appCfg}, nil
}

func Run(ctx context.Context, cfgPath string) error {
	app, cleanup, err := InitApp(cfgPath)
	defer cleanup()
	if err != nil {
		return err
	}
	return app.run(ctx)
}

func (app *Application) run(ctx context.Context) error {
	ag := session.NewAgent(app.AppCfg.Providers[0])
	srvList := app.buildTransport(ag)
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

func (app *Application) buildTransport(ag *session.Agent) []Server {
	var srv []Server
	tCfg := app.AppCfg.Transports
	if tCfg.HTTP.Enable {
		srv = append(srv, http_api.NewServer(ag, &tCfg.HTTP.Address))
	}
	return srv
}

//go:build wireinject

package app

import (
	"wind-agent/internal/config"
	"wind-agent/internal/data"

	"github.com/google/wire"
)

func InitApp(cfgPath string) (*Application, func(), error) {
	wire.Build(config.Load, data.NewData, New)
	return nil, nil, nil
}

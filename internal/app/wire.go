//go:build wireinject

package app

import (
	"wind-agent/internal/config"

	"github.com/google/wire"
)

func InitApp(cfgPath string) (*Application, func(), error) {
	wire.Build(config.Load, New)
	return nil, nil, nil
}

package config

import "github.com/spf13/viper"

type Configuration struct {
	App App `yaml:"app"`
}

type App struct {
	Providers  []Provider `yaml:"providers"`
	Transports Transport  `yaml:"transports"`
	Database   Database   `yaml:"database"`
}

func Load(cfgPath string) (App, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(cfgPath)
	cfg := &Configuration{}
	if err := v.ReadInConfig(); err != nil {
		return App{}, err
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg.App, err
	}
	return cfg.App, nil
}

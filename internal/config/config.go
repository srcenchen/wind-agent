package config

import "github.com/spf13/viper"

type Configuration struct {
	App App `yaml:"app"`
}

type App struct {
	Providers  []Provider `yaml:"providers"`
	Transports Transport  `yaml:"transports"`
}

type Provider struct {
	Name     string `yaml:"name"`
	Id       string `yaml:"id"`
	protocol string `yaml:"protocol"`
	Endpoint string `yaml:"endpoint"`
	Key      string `yaml:"key"`
}

type Transport struct {
	HTTP Entry `yaml:"http"`
}

type Entry struct {
	Enable  bool   `yaml:"enable"`
	Address string `yaml:"address"`
}

func Load(cfgPath string) (*App, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(cfgPath)
	cfg := &Configuration{}
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return &cfg.App, err
	}
	return &cfg.App, nil
}

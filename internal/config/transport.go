package config

type Transport struct {
	HTTP Entry `yaml:"http" mapstructure:"http"`
	QQ   QQ    `yaml:"qq" mapstructure:"qq"`
}

type Entry struct {
	Enable  bool   `yaml:"enable"`
	Address string `yaml:"address"`
}

type QQ struct {
	Enable bool   `yaml:"enable" mapstructure:"enable"`
	AppID  string `yaml:"app_id" mapstructure:"app_id"`
	Secret string `yaml:"secret" mapstructure:"secret"`
}

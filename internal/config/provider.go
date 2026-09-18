package config

type Provider struct {
	Name     string `yaml:"name"`
	Id       string `yaml:"id"`
	protocol string `yaml:"protocol"`
	Endpoint string `yaml:"endpoint"`
	Key      string `yaml:"key"`
	Model    string `yaml:"model"`
}

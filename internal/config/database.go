package config

type Database struct {
	Driver     string `yaml:"driver"`
	Connection string `yaml:"connection"`
}

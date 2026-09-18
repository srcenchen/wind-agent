package config

// Weather 和风天气（QWeather）配置。
type Weather struct {
	// Host 数据 API 主机，缺省 https://devapi.qweather.com；
	// 新版控制台分配了项目专属 API Host 时填完整地址，如 https://xxxx.re.qweatherapi.com
	Host string `yaml:"host"`
	// GeoHost 城市检索 API 主机，缺省 https://geoapi.qweather.com
	GeoHost string `yaml:"geo_host"`
	// Key 凭据 API KEY
	Key string `yaml:"key"`
}

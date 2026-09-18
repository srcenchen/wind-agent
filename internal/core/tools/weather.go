package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultWeatherHost = "https://devapi.qweather.com"
	defaultGeoHost     = "https://geoapi.qweather.com"
)

// Weather 查询和风天气（QWeather）实况与预报。
type Weather struct {
	Host    string
	GeoHost string
	Key     string
	Client  *http.Client
}

type weatherArgs struct {
	Location string `json:"location"`
	Type     string `json:"type"`
}

func (w Weather) Name() string { return "weather" }

func (w Weather) Description() string {
	return "查询城市天气：实况、未来3天或7天预报。location 支持城市名（如 北京）、城市ID 或经纬度（如 116.41,39.92）。"
}

func (w Weather) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"location": {
				"type": "string",
				"description": "城市名、城市ID 或经纬度（经度,纬度）"
			},
			"type": {
				"type": "string",
				"enum": ["now", "3d", "7d"],
				"description": "now=实况，3d=未来3天，7d=未来7天，默认 now"
			}
		},
		"required": ["location"]
	}`)
}

func (w Weather) Execute(ctx context.Context, args string) (string, error) {
	if w.Key == "" {
		return "", fmt.Errorf("weather: 未配置和风天气 key")
	}
	var in weatherArgs
	if err := json.Unmarshal([]byte(args), &in); err != nil {
		return "", fmt.Errorf("weather: 参数解析失败: %w", err)
	}
	loc := strings.TrimSpace(in.Location)
	if loc == "" {
		return "", fmt.Errorf("weather: location 不能为空")
	}
	kind := in.Type
	if kind == "" {
		kind = "now"
	}

	host := strings.TrimRight(w.Host, "/")
	if host == "" {
		host = defaultWeatherHost
	}
	geoHost := strings.TrimRight(w.GeoHost, "/")
	if geoHost == "" {
		geoHost = defaultGeoHost
		if !strings.Contains(host, "devapi") {
			geoHost = host
		}
	}

	// 天气接口的 location 只认城市ID或经纬度，城市名需先走 GeoAPI 检索。
	location := loc
	if !isCoord(loc) && !isLocationID(loc) {
		id, err := w.lookupCity(ctx, geoHost, loc)
		if err != nil {
			return "", err
		}
		location = id
	}

	switch kind {
	case "now":
		return w.queryNow(ctx, host, location)
	case "3d":
		return w.queryDaily(ctx, host, location, "3d")
	case "7d":
		return w.queryDaily(ctx, host, location, "7d")
	default:
		return "", fmt.Errorf("weather: 不支持的 type %q（可选 now/3d/7d）", kind)
	}
}

func (w Weather) lookupCity(ctx context.Context, geoHost, name string) (string, error) {
	var resp struct {
		Code     string `json:"code"`
		Location []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Adm1 string `json:"adm1"`
			Adm2 string `json:"adm2"`
		} `json:"location"`
	}
	if err := w.get(ctx, geoHost+"/v2/city/lookup", url.Values{
		"location": {name},
		"number":   {"1"},
	}, &resp); err != nil {
		return "", err
	}
	if resp.Code != "200" || len(resp.Location) == 0 {
		return "", fmt.Errorf("weather: 未找到城市 %q (code=%s)", name, resp.Code)
	}
	return resp.Location[0].ID, nil
}

func (w Weather) queryNow(ctx context.Context, host, location string) (string, error) {
	var resp struct {
		Code       string `json:"code"`
		UpdateTime string `json:"updateTime"`
		Now        struct {
			ObsTime   string `json:"obsTime"`
			Temp      string `json:"temp"`
			FeelsLike string `json:"feelsLike"`
			Text      string `json:"text"`
			WindDir   string `json:"windDir"`
			WindScale string `json:"windScale"`
			Humidity  string `json:"humidity"`
			Precip    string `json:"precip"`
			Pressure  string `json:"pressure"`
			Vis       string `json:"vis"`
		} `json:"now"`
	}
	if err := w.getWeather(ctx, host, "/v7/weather/now", location, &resp); err != nil {
		return "", err
	}
	if resp.Code != "200" {
		return "", weatherCodeErr(resp.Code)
	}
	n := resp.Now
	var b strings.Builder
	fmt.Fprintf(&b, "实况天气（观测时间 %s）\n", n.ObsTime)
	fmt.Fprintf(&b, "天气：%s\n", n.Text)
	fmt.Fprintf(&b, "气温：%s℃（体感 %s℃）\n", n.Temp, n.FeelsLike)
	fmt.Fprintf(&b, "风力：%s %s级\n", n.WindDir, n.WindScale)
	fmt.Fprintf(&b, "湿度：%s%%　气压：%shPa　能见度：%skm\n", n.Humidity, n.Pressure, n.Vis)
	fmt.Fprintf(&b, "降水量：%smm", n.Precip)
	return b.String(), nil
}

func (w Weather) queryDaily(ctx context.Context, host, location, kind string) (string, error) {
	var resp struct {
		Code  string `json:"code"`
		Daily []struct {
			FxDate       string `json:"fxDate"`
			TempMax      string `json:"tempMax"`
			TempMin      string `json:"tempMin"`
			TextDay      string `json:"textDay"`
			TextNight    string `json:"textNight"`
			WindDirDay   string `json:"windDirDay"`
			WindScaleDay string `json:"windScaleDay"`
			Humidity     string `json:"humidity"`
			Precip       string `json:"precip"`
			UvIndex      string `json:"uvIndex"`
		} `json:"daily"`
	}
	if err := w.getWeather(ctx, host, "/v7/weather/"+kind, location, &resp); err != nil {
		return "", err
	}
	if resp.Code != "200" {
		return "", weatherCodeErr(resp.Code)
	}
	if len(resp.Daily) == 0 {
		return "", fmt.Errorf("weather: 无预报数据")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "未来%d天预报\n", len(resp.Daily))
	for _, d := range resp.Daily {
		fmt.Fprintf(&b, "%s：%s转%s，%s~%s℃，%s%s级，湿度%s%%，降水%smm，紫外线%s\n",
			d.FxDate, d.TextDay, d.TextNight, d.TempMin, d.TempMax,
			d.WindDirDay, d.WindScaleDay, d.Humidity, d.Precip, d.UvIndex)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (w Weather) getWeather(ctx context.Context, host, path, location string, v any) error {
	return w.get(ctx, host+path, url.Values{"location": {location}}, v)
}

func (w Weather) get(ctx context.Context, rawURL string, query url.Values, v any) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("weather: URL 非法: %w", err)
	}
	q := u.Query()
	for k, vals := range query {
		for _, val := range vals {
			q.Set(k, val)
		}
	}
	q.Set("key", w.Key)
	u.RawQuery = q.Encode()

	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("weather: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("weather: 读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("weather: 服务返回 %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("weather: 解析响应失败: %w", err)
	}
	return nil
}

func weatherCodeErr(code string) error {
	switch code {
	case "204":
		return fmt.Errorf("weather: 该地区暂无数据 (204)")
	case "401":
		return fmt.Errorf("weather: API KEY 无效 (401)")
	case "402":
		return fmt.Errorf("weather: 超出访问量限额 (402)")
	case "403":
		return fmt.Errorf("weather: 无访问权限 (403)")
	case "404":
		return fmt.Errorf("weather: 查询的地区不存在 (404)")
	default:
		return fmt.Errorf("weather: 接口返回错误码 %s", code)
	}
}

func isCoord(s string) bool {
	i := strings.IndexByte(s, ',')
	if i <= 0 || i == len(s)-1 {
		return false
	}
	lon, lat := strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	return isNumber(lon) && isNumber(lat)
}

func isLocationID(s string) bool {
	if len(s) < 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	for _, r := range s {
		switch {
		case r == '.':
			if dot {
				return false
			}
			dot = true
		case r == '-' || (r >= '0' && r <= '9'):
		default:
			return false
		}
	}
	return true
}

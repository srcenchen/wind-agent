package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newWeatherServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/city/lookup", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			t.Errorf("missing key")
		}
		_, _ = w.Write([]byte(`{"code":"200","location":[{"id":"101010100","name":"北京","adm1":"北京市"}]}`))
	})
	mux.HandleFunc("/v7/weather/now", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("location"); got != "101010100" {
			t.Errorf("location = %q, want resolved id", got)
		}
		_, _ = w.Write([]byte(`{"code":"200","now":{"obsTime":"2026-09-18T11:48+08:00","temp":"26","feelsLike":"29","text":"多云","windDir":"北风","windScale":"3","humidity":"55","precip":"0.0","pressure":"1017","vis":"29"}}`))
	})
	mux.HandleFunc("/v7/weather/3d", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"200","daily":[{"fxDate":"2026-09-18","tempMax":"30","tempMin":"20","textDay":"晴","textNight":"多云","windDirDay":"西风","windScaleDay":"1-2","humidity":"40","precip":"0.0","uvIndex":"5"}]}`))
	})
	return httptest.NewServer(mux)
}

func TestWeatherCityNameResolvesThenQueriesNow(t *testing.T) {
	srv := newWeatherServer(t)
	defer srv.Close()

	w := Weather{Host: srv.URL, GeoHost: srv.URL, Key: "test"}
	out, err := w.Execute(context.Background(), `{"location":"北京","type":"now"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"多云", "26℃", "北风 3级"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWeatherCoordSkipsLookupAndDaily(t *testing.T) {
	srv := newWeatherServer(t)
	defer srv.Close()

	w := Weather{Host: srv.URL, GeoHost: srv.URL, Key: "test"}
	out, err := w.Execute(context.Background(), `{"location":"116.41,39.92","type":"3d"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "2026-09-18") || !strings.Contains(out, "晴转多云") {
		t.Errorf("unexpected daily output:\n%s", out)
	}
}

func TestWeatherDefaultsToNow(t *testing.T) {
	srv := newWeatherServer(t)
	defer srv.Close()

	w := Weather{Host: srv.URL, GeoHost: srv.URL, Key: "test"}
	out, err := w.Execute(context.Background(), `{"location":"101010100"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "实况天气") {
		t.Errorf("expected now output:\n%s", out)
	}
}

func TestWeatherRejectsEmptyLocation(t *testing.T) {
	w := Weather{Key: "test"}
	if _, err := w.Execute(context.Background(), `{"location":"  "}`); err == nil {
		t.Fatal("expected error for empty location")
	}
}

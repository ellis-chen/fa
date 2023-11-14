package fsutils

import (
	"encoding/json"
	"net/http"
)

// H is a shortcut for map[string]interface{}
type H map[string]interface{}

func (h H) ToJson() string {
	bs, _ := json.Marshal(h)
	return string(bs)
}

func ReadUserIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-Ip")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

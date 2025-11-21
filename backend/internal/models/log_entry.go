package models

// CaddyAccessLog represents a structured log entry from Caddy's JSON access logs.
type CaddyAccessLog struct {
	Level   string  `json:"level"`
	Ts      float64 `json:"ts"`
	Logger  string  `json:"logger"`
	Msg     string  `json:"msg"`
	Request struct {
		RemoteIP   string              `json:"remote_ip"`
		RemotePort string              `json:"remote_port"`
		ClientIP   string              `json:"client_ip"`
		Proto      string              `json:"proto"`
		Method     string              `json:"method"`
		Host       string              `json:"host"`
		URI        string              `json:"uri"`
		Headers    map[string][]string `json:"headers"`
		TLS        struct {
			Resumed     bool   `json:"resumed"`
			Version     int    `json:"version"`
			CipherSuite int    `json:"cipher_suite"`
			Proto       string `json:"proto"`
			ServerName  string `json:"server_name"`
		} `json:"tls"`
	} `json:"request"`
	BytesRead   int                 `json:"bytes_read"`
	UserID      string              `json:"user_id"`
	Duration    float64             `json:"duration"`
	Size        int                 `json:"size"`
	Status      int                 `json:"status"`
	RespHeaders map[string][]string `json:"resp_headers"`
}

// LogFilter defines criteria for filtering logs.
type LogFilter struct {
	Search string `form:"search"`
	Host   string `form:"host"`
	Status string `form:"status"` // e.g., "200", "4xx", "5xx"
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

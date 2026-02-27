package notifications

import "net/http"

func executeNotifyRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req)
}

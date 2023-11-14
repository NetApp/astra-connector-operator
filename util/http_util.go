package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPClient interface used for request and to facilitate testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HeaderMap User specific details required for the http header
type HeaderMap struct {
	AccountId     string
	Authorization string
}

// DoRequest Makes http request with the given parameters
func DoRequest(ctx context.Context, client HTTPClient, method, url string, body io.Reader, headerMap HeaderMap, retryCount ...int) (*http.Response, error, context.CancelFunc) {
	// Default retry count
	retries := 1
	if len(retryCount) > 0 {
		retries = retryCount[0]
	}

	var httpResponse *http.Response
	var err error

	// Child context that can't exceed a deadline specified
	childCtx, cancel := context.WithTimeout(ctx, 3*time.Minute) // TODO : Update timeout here

	req, _ := http.NewRequestWithContext(childCtx, method, url, body)

	req.Header.Add("Content-Type", "application/json")

	if headerMap.Authorization != "" {
		req.Header.Add("authorization", headerMap.Authorization)
	}

	for i := 0; i < retries; i++ {
		httpResponse, err = client.Do(req)
		if err == nil {
			break
		}
	}

	return httpResponse, err, cancel
}

func SetHttpClient(disableTls bool, astraHost, hostAliasIP string, log logr.Logger) (*http.Client, error) {
	if disableTls {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.WithValues("disableTls", disableTls).Info("TLS Validation Disabled! Not for use in production!")
	}

	if hostAliasIP != "" {
		log.WithValues("HostAliasIP", hostAliasIP).Info("Using the HostAlias IP")
		cloudBridgeHost, err := getAstraHostFromURL(astraHost)
		if err != nil {
			return &http.Client{}, err
		}

		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = hostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = hostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	return &http.Client{}, nil
}

func getAstraHostFromURL(astraHostURL string) (string, error) {
	cloudBridgeURLSplit := strings.Split(astraHostURL, "://")
	if len(cloudBridgeURLSplit) != 2 {
		errStr := fmt.Sprintf("invalid cloudBridgeURL provided: %s, format - https://hostname", astraHostURL)
		return "", errors.New(errStr)
	}
	return cloudBridgeURLSplit[1], nil
}

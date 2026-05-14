package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type RESTClient struct {
	client *http.Client
}

func NewRESTClient(client *http.Client) *RESTClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &RESTClient{client: client}
}

func (c *RESTClient) Call(ctx context.Context, target Upstream, inbound *http.Request) (*http.Response, error) {
	if target.Protocol != ProtocolREST {
		return nil, fmt.Errorf("unsupported upstream protocol %q", target.Protocol)
	}

	targetURL, err := buildTargetURL(target.BaseURL, inbound.URL)
	if err != nil {
		return nil, err
	}

	outbound, err := http.NewRequestWithContext(ctx, inbound.Method, targetURL, inbound.Body)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	copyForwardHeaders(outbound.Header, inbound.Header)
	outbound.Host = outbound.URL.Host

	return c.client.Do(outbound)
}

func buildTargetURL(baseURL string, inboundURL *url.URL) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse upstream base url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("upstream base url must include scheme and host")
	}

	base.Path = joinPath(base.Path, inboundURL.Path)
	base.RawQuery = inboundURL.RawQuery
	base.Fragment = ""

	return base.String(), nil
}

func joinPath(basePath string, requestPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	if requestPath == "" {
		requestPath = "/"
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	return basePath + requestPath
}

func copyForwardHeaders(dst http.Header, src http.Header) {
	for name, values := range src {
		if !isForwardedRequestHeader(name) {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func CopyResponse(w http.ResponseWriter, resp *http.Response) error {
	defer resp.Body.Close()

	for name, values := range resp.Header {
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, err := io.Copy(w, resp.Body)
	return err
}

func isForwardedRequestHeader(name string) bool {
	if isHopByHopHeader(name) {
		return false
	}

	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Proxy-Authorization", "X-Api-Key":
		return false
	default:
		return true
	}
}

func isHopByHopHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade":
		return true
	default:
		return false
	}
}

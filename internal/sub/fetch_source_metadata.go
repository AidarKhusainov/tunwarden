package sub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// FetchResult contains subscription response bytes plus safe response metadata
// that can be used for provider display-name detection. Only response headers
// are retained; request URLs and provider tokens are not copied into the result.
type FetchResult struct {
	Content []byte
	Header  http.Header
}

// FetchSourceWithMetadata reads subscription content and preserves bounded,
// provider-supplied response metadata. file:// URLs have no response headers;
// HTTP(S) response headers are cloned after a successful status response.
func FetchSourceWithMetadata(ctx context.Context, source Source) (FetchResult, error) {
	return fetchSource(ctx, source)
}

func fetchSource(ctx context.Context, source Source) (FetchResult, error) {
	if err := ValidateSource(source); err != nil {
		return FetchResult{}, err
	}
	u, err := url.Parse(source.URL)
	if err != nil {
		return FetchResult{}, fmt.Errorf("parse subscription URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "file":
		path, err := url.PathUnescape(u.Path)
		if err != nil {
			return FetchResult{}, fmt.Errorf("read subscription %s: file path is not valid percent-encoding", source.ID)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return FetchResult{}, fmt.Errorf("read subscription %s: %w", source.ID, err)
		}
		return FetchResult{Content: data}, nil
	case "http", "https":
		return fetchHTTPSource(ctx, source)
	default:
		return FetchResult{}, fmt.Errorf("unsupported subscription URL scheme %q", u.Scheme)
	}
}

func fetchHTTPSource(ctx context.Context, source Source) (FetchResult, error) {
	requestURL, clientID, err := subscriptionRequestURL(source.URL)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetch subscription %s: %w", source.ID, err)
	}
	if clientID == "" {
		clientID, err = LoadOrCreateClientID("")
		if err != nil {
			return FetchResult{}, fmt.Errorf("fetch subscription %s: prepare subscription client identity: %w", source.ID, err)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: sameOriginRedirectPolicy}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return FetchResult{}, fetchSubscriptionError(source.ID, err, clientID)
	}
	req.Header.Set("User-Agent", subscriptionUserAgent)
	req.Header.Set(subscriptionClientHeader, clientID)
	res, err := client.Do(req)
	if err != nil {
		return FetchResult{}, fetchSubscriptionError(source.ID, err, clientID)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf("fetch subscription %s: unexpected HTTP status %d", source.ID, res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024+1))
	if err != nil {
		return FetchResult{}, fmt.Errorf("read subscription %s response: %w", source.ID, err)
	}
	if len(data) > 4*1024*1024 {
		return FetchResult{}, fmt.Errorf("read subscription %s response: content exceeds 4 MiB limit", source.ID)
	}
	return FetchResult{Content: data, Header: res.Header.Clone()}, nil
}

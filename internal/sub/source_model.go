package sub

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AidarKhusainov/podlaz/internal/profile"
)

const (
	subscriptionsFileName    = "subscriptions.json"
	subscriptionUserAgent    = "podlaz"
	subscriptionClientHeader = "x-hwid"
)

var subscriptionUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Format string

const (
	FormatUnknown  Format = "unknown"
	FormatURIList  Format = "uri-list"
	FormatBase64   Format = "base64"
	FormatXrayJSON Format = "xray-json"
	FormatSingBox  Format = "sing-box"
	FormatMihomo   Format = "mihomo"
)

var ErrNotFound = errors.New("subscription not found")
var ErrAlreadyExists = errors.New("subscription already exists")

type Source struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Format        Format    `json:"format"`
	ProfileIDs    []string  `json:"profile_ids,omitempty"`
	LastUpdatedAt time.Time `json:"last_updated_at,omitempty"`
}

type Issue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type Parsed struct {
	Profiles    []profile.Profile
	Unsupported []Issue
	Warnings    []Issue
}

type UpdateResult struct {
	Subscription Source
	Imported     int
	Updated      int
	Unchanged    int
	Removed      int
	Unsupported  int
	Warnings     []Issue
	Issues       []Issue
}

type Store struct {
	path string
}

func NewStore(path string) (Store, error) {
	if path == "" {
		defaultPath, err := DefaultStorePath()
		if err != nil {
			return Store{}, err
		}
		path = defaultPath
	}
	return Store{path: path}, nil
}

func DefaultStorePath() (string, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" || !filepath.IsAbs(stateHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve podlaz state directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "podlaz", subscriptionsFileName), nil
}

func (s Store) Path() string { return s.path }

func NewSource(name, sourceURL string) Source {
	sourceURL = strings.TrimSpace(sourceURL)
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = fallbackSubscriptionDisplayName(sourceURL)
	}
	return Source{
		ID:     profile.NormalizeID(displayName),
		Name:   displayName,
		URL:    sourceURL,
		Format: FormatBase64,
	}
}

func fallbackSubscriptionDisplayName(sourceURL string) string {
	candidate := "subscription"
	if u, err := url.Parse(strings.TrimSpace(sourceURL)); err == nil {
		host := strings.TrimSpace(u.Hostname())
		base := safeHumanPathBase(u.Path)
		switch {
		case host != "" && base != "":
			candidate = host + " " + base
		case host != "":
			candidate = host
		case base != "":
			candidate = base
		}
	}
	if name, ok := profile.SanitizeDisplayName(candidate); ok {
		return name
	}
	return "subscription"
}

func safeHumanPathBase(rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || rawPath == "/" {
		return ""
	}
	if suspiciousSubscriptionPathContext(rawPath) {
		return ""
	}
	base, err := url.PathUnescape(pathpkg.Base(rawPath))
	if err != nil {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		return ""
	}
	if unsafeSubscriptionPathBase(base) {
		return ""
	}
	name, ok := profile.SanitizeDisplayName(base)
	if !ok {
		return ""
	}
	return name
}

func suspiciousSubscriptionPathContext(rawPath string) bool {
	dir := pathpkg.Dir(rawPath)
	if dir == "." || dir == "/" {
		return false
	}
	for _, segment := range strings.Split(dir, "/") {
		segment = normalizeSubscriptionPathContextSegment(segment)
		switch {
		case segment == "sub",
			segment == "subs",
			segment == "subscription",
			segment == "subscriptions",
			segment == "subscribe",
			segment == "invite",
			segment == "link",
			segment == "token",
			segment == "key":
			return true
		case strings.HasPrefix(segment, "sub") && len(segment) >= len("sub")+4:
			return true
		}
	}
	return false
}

func normalizeSubscriptionPathContextSegment(segment string) string {
	segment = strings.ToLower(strings.TrimSpace(segment))
	replacer := strings.NewReplacer(
		"0", "o",
		"1", "i",
		"3", "e",
		"4", "a",
		"5", "s",
		"7", "t",
		"-", "",
		"_", "",
		".", "",
	)
	return replacer.Replace(segment)
}

func unsafeSubscriptionPathBase(base string) bool {
	trimmed := strings.TrimSpace(base)
	lower := strings.ToLower(trimmed)
	return subscriptionUUIDPattern.MatchString(trimmed) ||
		looksSubscriptionSecretLike(lower) ||
		looksSubscriptionPathTokenLike(trimmed) ||
		looksNumericOrHashLike(lower)
}

func looksSubscriptionSecretLike(value string) bool {
	for _, marker := range []string{"tok" + "en", "pass" + "word", "pass" + "wd", "sec" + "ret", "priv" + "ate", "author" + "ization", "api" + "_key", "api" + "key"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func looksSubscriptionPathTokenLike(value string) bool {
	if len(value) < 12 || strings.ContainsAny(value, " -_") {
		return false
	}
	hasLower, hasUpper, hasDigit := false, false, false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return false
		}
	}
	return hasLower && hasUpper && hasDigit
}

func looksNumericOrHashLike(value string) bool {
	if len(value) >= 6 && allDigits(value) {
		return true
	}
	return len(value) >= 8 && allHex(value)
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func allHex(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' {
			continue
		}
		return false
	}
	return true
}

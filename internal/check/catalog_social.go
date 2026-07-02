package check

func init() {
	targetCatalog = append(targetCatalog,
		Target{
			ID:               "instagram",
			DisplayName:      "Instagram",
			Category:         "social-service",
			ProbeType:        ProbeTypeHTTPS,
			URL:              "https://" + "www.instagram.com/robots.txt",
			Timeout:          DefaultTimeout,
			SuccessCondition: "HTTP status below 500",
			ProxyDNS:         true,
			PrivacyNote:      "Instagram receives a single HTTPS request from the selected proxy path.",
		},
		Target{
			ID:               "youtube",
			DisplayName:      "YouTube",
			Category:         "video-service",
			ProbeType:        ProbeTypeHTTPS,
			URL:              "https://" + "www.youtube.com/generate_204",
			Timeout:          DefaultTimeout,
			SuccessCondition: "HTTP status 204 or any non-server-error status",
			ProxyDNS:         true,
			PrivacyNote:      "YouTube receives a single HTTPS connectivity-check request from the selected proxy path.",
		},
	)
}

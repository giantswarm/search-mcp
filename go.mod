module github.com/giantswarm/search-mcp

go 1.25.5

require (
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2
	github.com/PuerkitoBio/goquery v1.13.0
	github.com/mark3labs/mcp-go v1.0.0
	github.com/prometheus/client_golang v1.24.1
	github.com/spf13/cobra v1.10.2
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/andybalholm/cascadia v1.3.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// Pin transitive modules flagged by the OSS Index scan (nancy) in CI.
replace golang.org/x/mod => golang.org/x/mod v0.40.0

replace golang.org/x/crypto => golang.org/x/crypto v0.55.0

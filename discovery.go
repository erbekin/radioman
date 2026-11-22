package radiobrowser

import (
	"fmt"
	"net"
	"slices"
	"strings"
)

const lookupDomain = "radio-browser.info"

func discoverHosts() ([]string, error) {
	_, records, err := net.LookupSRV("api", "tcp", lookupDomain)
	if err != nil {
		return nil, fmt.Errorf("discover RadioBrowser hosts: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("discover RadioBrowser hosts: no hosts returned")
	}

	slices.SortFunc(records, func(a, b *net.SRV) int {
		if a.Priority != b.Priority {
			return int(a.Priority) - int(b.Priority)
		}
		return int(a.Weight) - int(b.Weight)
	})

	hosts := make([]string, 0, len(records))
	for _, record := range records {
		host := strings.TrimSuffix(record.Target, ".")
		hosts = append(hosts, fmt.Sprintf("https://%s:%d", host, record.Port))
	}

	return hosts, nil
}

package validate

import (
	"net"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
)

func AlertmanagerUrl(url string) error {
	if url == "" {
		return nil
	}

	err := config.CheckTargetAddress(model.LabelValue(url))
	if err != nil {
		return err
	}

	host, _, err := net.SplitHostPort(url)
	if err != nil {
		_, err = net.LookupHost(url)
		if err != nil {
			return err
		}

		return nil
	}

	_, err = net.LookupHost(host)
	if err != nil {
		return err
	}

	return nil
}

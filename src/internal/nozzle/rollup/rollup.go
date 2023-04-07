package rollup

import (
	"encoding/csv"
	"strings"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

const (
	GorouterHttpMetricName = "http"
	GorouterSourceId       = "gorouter"
)

type PointsBatch struct {
	Points []*rpc.Point
	Size   int
}

type Rollup interface {
	Record(sourceId string, tags map[string]string, value int64)
	Rollup(timestamp int64) []*PointsBatch
}

func keyFromTags(rollupTags []string, sourceId string, tags map[string]string) string {
	filteredTags := []string{sourceId}

	for _, tag := range rollupTags {
		filteredTags = append(filteredTags, tags[tag])
	}

	csvOutput := &strings.Builder{}
	csvWriter := csv.NewWriter(csvOutput)
	_ = csvWriter.Write(filteredTags)
	csvWriter.Flush()
	return csvOutput.String()
}

func labelsFromKey(key, nodeIndex string, rollupTags []string, log *logger.Logger) (map[string]string, error) {
	keyParts, err := csv.NewReader(strings.NewReader(key)).Read()

	if err != nil {
		log.Error(
			"skipping rollup metric",
			err,
			logger.String("reason", "failed to decode"),
			logger.String("key", key),
		)
		return nil, err
	}

	// if we can't parse the key, there's probably some garbage in one
	// of the tags, so let's skip it
	if len(keyParts) != len(rollupTags)+1 {
		log.Info(
			"skipping rollup metric",
			logger.String("reason", "wrong number of parts"),
			logger.String("key", key),
			logger.Count(len(keyParts)),
		)
		return nil, err
	}

	labels := make(map[string]string)
	for index, tagName := range rollupTags {
		if value := keyParts[index+1]; value != "" {
			labels[tagName] = value
		}
	}

	labels["source_id"] = keyParts[0]
	labels["node_index"] = nodeIndex

	return labels, nil
}

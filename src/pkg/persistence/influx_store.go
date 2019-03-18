package persistence

import (
	"path/filepath"

	"github.com/influxdata/influxdb/tsdb"
)

func OpenTsStore(storagePath string) (*tsdb.Store, error) {
	baseDir := filepath.Join(storagePath, "influxdb")
	dataDir := filepath.Join(baseDir, "data")
	walDir := filepath.Join(baseDir, "wal")

	tsStore := tsdb.NewStore(dataDir)
	tsStore.EngineOptions.WALEnabled = true
	tsStore.EngineOptions.Config.WALDir = walDir
	tsStore.EngineOptions.Config.MaxSeriesPerDatabase = 0
	tsStore.EngineOptions.Config.MaxValuesPerTag = 0

	if err := tsStore.Open(); err != nil {
		return nil, err
	}

	return tsStore, nil
}

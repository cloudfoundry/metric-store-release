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

	// These might be redundant, but for now it seems safer to set both of
	// them to the new TSI index
	tsStore.EngineOptions.IndexVersion = tsdb.TSI1IndexName
	tsStore.EngineOptions.Config.Index = tsdb.TSI1IndexName

	if err := tsStore.Open(); err != nil {
		return nil, err
	}

	return tsStore, nil
}

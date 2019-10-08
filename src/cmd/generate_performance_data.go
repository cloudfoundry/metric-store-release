package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	shared "github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	"github.com/prometheus/prometheus/pkg/labels"
)

func main() {
	numberOfPoints := 1000000

	if len(os.Args) == 1 {
		panic("You must provide the storage path as an arg")
	}

	fmt.Println(" - storing data in", os.Args[1])

	spyPersistentStoreMetrics := shared.NewSpyMetricRegistrar()
	persistentStore := persistence.NewStore(
		os.Args[1],
		spyPersistentStoreMetrics,
	)

	appender, _ := persistentStore.Appender()

	testLabels := labels.Labels{
		{Name: labels.MetricName, Value: "bigmetric"},
		{Name: "app_id", Value: "bde5831e-a819-4a34-9a46-012fd2e821e6b"},
		{Name: "app_name", Value: "bblog"},
		{Name: "bosh_environment", Value: "vpc-bosh-run-pivotal-io"},
		{Name: "deployment", Value: "pws-diego-cellblock-09"},
		{Name: "index", Value: "9b74a5b1-9af9-4715-a57a-bd28ad7e7f1b"},
		{Name: "instance_id", Value: "0"},
		{Name: "ip", Value: "10.10.148.146"},
		{Name: "job", Value: "diego-cell"},
		{Name: "organization_id", Value: "ab2de77b-a484-4690-9201-b8eaf707fd87"},
		{Name: "organization_name", Value: "blars"},
		{Name: "origin", Value: "rep"},
		{Name: "process_id", Value: "328de02b-79f1-4f8d-b3b2-b81112809603"},
		{Name: "process_instance_id", Value: "2652349b-4d40-4b51-4165-7129"},
		{Name: "process_type", Value: "web"},
		{Name: "source_id", Value: "5ee5831e-a819-4a34-9a46-012fd2e821e7"},
		{Name: "space_id", Value: "eb94778d-66b5-4804-abcb-e9efd7f725aa"},
		{Name: "space_name", Value: "bblog"},
		{Name: "unit", Value: "percentage"},
		// {Name: "uri", Value: "https://google.ca"},
	}

	fmt.Printf(" - generating %d points\n", numberOfPoints)
	for i := 0; i < numberOfPoints; i++ {
		appender.Add(testLabels, time.Now().Add(time.Duration(-i)*time.Second).UnixNano(), .5)
	}
	appender.Commit()

	fmt.Println(" - compacting data")
	persistentStore.Compact()

	fmt.Println(" - waiting on compaction")
	time.Sleep(60 * time.Second)

	fmt.Println(" - DONE!")
}

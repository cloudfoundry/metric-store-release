module github.com/cloudfoundry/metric-store-release

require (
	cloud.google.com/go/bigtable v1.3.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee
	code.cloudfoundry.org/go-envstruct v1.5.0
	code.cloudfoundry.org/go-loggregator v0.0.0-20190725203007-b8d176783c8a
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	collectd.org v0.5.0 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20200619233522-341c6c546fe1 // indirect
	github.com/benbjohnson/jmphash v0.0.0-20141216154655-2d58f234cd86
	github.com/cespare/xxhash v1.1.0
	github.com/dvsekhvalnov/jose2go v0.0.0-20180829124132-7f401d37b68a
	github.com/emirpasic/gods v1.12.0
	github.com/fortytw2/leaktest v1.3.0
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-kit/kit v0.10.0
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/flux v0.69.2 // indirect
	github.com/influxdata/influxdb v1.8.4
	github.com/influxdata/influxql v1.1.1-0.20200828144457-65d3ef77d385
	github.com/json-iterator/go v1.1.10
	github.com/jsternberg/zap-logfmt v1.2.0 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.20.0
	github.com/prometheus/prometheus v1.8.2-0.20200724121523-657ba532e42f
	github.com/spf13/cast v1.3.1 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/willf/bitset v1.1.10 // indirect
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae
	google.golang.org/grpc v1.32.0
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/apimachinery v0.18.5 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/influxdata/influxdb => github.com/attack/influxdb v1.8.5-0.20210209205548-2c317a0836d6

go 1.15

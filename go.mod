module github.com/cloudfoundry/metric-store-release

require (
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee
	code.cloudfoundry.org/go-envstruct v1.5.0
	code.cloudfoundry.org/go-loggregator v0.0.0-20190725203007-b8d176783c8a
	collectd.org v0.3.0 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20191031071000-627c72987925 // indirect
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/dgryski/go-bitstream v0.0.0-20180413035011-3522498ce2c8 // indirect
	github.com/dvsekhvalnov/jose2go v0.0.0-20180829124132-7f401d37b68a
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/glycerine/goconvey v0.0.0-20190410193231-58a59202ab31 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/mux v1.7.3
	github.com/influxdata/flux v0.52.0 // indirect
	github.com/influxdata/influxdb v1.7.9
	github.com/influxdata/influxql v1.0.1
	github.com/influxdata/roaring v0.4.12 // indirect
	github.com/influxdata/usage-client v0.0.0-20160829180054-6d3895376368 // indirect
	github.com/jsternberg/zap-logfmt v1.2.0 // indirect
	github.com/jwilder/encoding v0.0.0-20170811194829-b4e1701a28ef // indirect
	github.com/mdempsky/unconvert v0.0.0-20190921185256-3ecd357795af // indirect
	github.com/mschoch/smat v0.0.0-20160514031455-90eadee771ae // indirect
	github.com/niubaoshu/gotiny v0.0.3
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.7.0
	github.com/prometheus/prometheus v1.8.2-0.20191017095924-6f92ce560538
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/willf/bitset v1.1.10 // indirect
	go.uber.org/zap v1.12.0
	golang.org/x/net v0.0.0-20191109021931-daa7c04131f5
	golang.org/x/sys v0.0.0-20191110163157-d32e6e3b99c4
	golang.org/x/tools v0.0.0-20191111200310-9d59ce8a7f66 // indirect
	google.golang.org/grpc v1.24.0
)

replace github.com/influxdata/influxdb => github.com/attack/influxdb v1.7.9-0.20191029173138-5bd71457cbd5

go 1.13

module github.com/cloudfoundry/metric-store-release

require (
	cloud.google.com/go v0.44.1 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee
	code.cloudfoundry.org/go-envstruct v1.5.0
	code.cloudfoundry.org/go-loggregator v0.0.0-20190725203007-b8d176783c8a
	collectd.org v0.3.0 // indirect
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20191216154348-860796e0b024 // indirect
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878 // indirect
	github.com/benbjohnson/jmphash v0.0.0-20141216154655-2d58f234cd86
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/dgryski/go-bitstream v0.0.0-20180413035011-3522498ce2c8 // indirect
	github.com/dvsekhvalnov/jose2go v0.0.0-20180829124132-7f401d37b68a
	github.com/emirpasic/gods v1.12.0
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/glycerine/goconvey v0.0.0-20190410193231-58a59202ab31 // indirect
	github.com/go-kit/kit v0.9.0
	github.com/go-openapi/analysis v0.19.4 // indirect
	github.com/go-openapi/runtime v0.19.3 // indirect
	github.com/go-openapi/swag v0.19.4 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-rootcerts v1.0.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/hashicorp/memberlist v0.1.4 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/influxdata/flux v0.57.0 // indirect
	github.com/influxdata/influxdb v1.7.9
	github.com/influxdata/influxql v1.0.1
	github.com/influxdata/roaring v0.4.12 // indirect
	github.com/influxdata/usage-client v0.0.0-20160829180054-6d3895376368 // indirect
	github.com/json-iterator/go v1.1.8
	github.com/jsternberg/zap-logfmt v1.2.0 // indirect
	github.com/jwilder/encoding v0.0.0-20170811194829-b4e1701a28ef // indirect
	github.com/mschoch/smat v0.0.0-20160514031455-90eadee771ae // indirect
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.1.0
	github.com/prometheus/common v0.7.0
	github.com/prometheus/prometheus v1.8.2-0.20191111142012-edeb7a44cbf7
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/tinylib/msgp v1.1.1 // indirect
	github.com/willf/bitset v1.1.10 // indirect
	go.mongodb.org/mongo-driver v1.0.4 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
	google.golang.org/grpc v1.24.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/kube-openapi v0.0.0-20190722073852-5e22f3d471e6 // indirect
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a // indirect
)

replace github.com/influxdata/influxdb => github.com/attack/influxdb v1.7.9-0.20191029173138-5bd71457cbd5

go 1.13

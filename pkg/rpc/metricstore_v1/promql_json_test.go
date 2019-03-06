package metricstore_v1_test

import (
	"io"
	"time"

	rpc "github.com/cloudfoundry/metric-store/pkg/rpc/metricstore_v1"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PromQL JSON", func() {
	Context("Marshal()", func() {
		It("handles a scalar instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Scalar{
					Scalar: &rpc.PromQL_Point{
						Time:  1234,
						Value: 2.5,
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1.234, "2.5"]
				}
			}`))
		})

		It("handles a vector instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Vector{
					Vector: &rpc.PromQL_Vector{
						Samples: []*rpc.PromQL_Sample{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Point: &rpc.PromQL_Point{
									Time:  1000,
									Value: 2.5,
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Point: &rpc.PromQL_Point{
									Time:  2000,
									Value: 3.5,
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"value": [ 1.000, "2.5" ]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"value": [ 2.000, "3.5" ]
						}
					]
				}
			}`))
		})

		It("handles an empty vector instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Vector{
					Vector: &rpc.PromQL_Vector{
						Samples: []*rpc.PromQL_Sample{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": []
				}
			}`))
		})

		It("handles a vector instant query result with no tags", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Vector{
					Vector: &rpc.PromQL_Vector{
						Samples: []*rpc.PromQL_Sample{
							{
								Point: &rpc.PromQL_Point{
									Time:  1000,
									Value: 2.5,
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [ 1.000, "2.5" ]
						}
					]
				}
			}`))
		})

		It("handles a matrix instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 2.5,
									},
									{
										Time:  2000,
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 4.5,
									},
									{
										Time:  2000,
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles an empty matrix instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": []
				}
			}`))
		})

		It("handles a matrix instant query result with no tags", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 2.5,
									},
									{
										Time:  2000,
										Value: 3.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles a matrix range query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_RangeQueryResult{
				Result: &rpc.PromQL_RangeQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 2.5,
									},
									{
										Time:  2000,
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 4.5,
									},
									{
										Time:  2000,
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles an empty matrix range query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_RangeQueryResult{
				Result: &rpc.PromQL_RangeQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": []
				}
			}`))
		})

		It("handles a matrix range query result with no tags", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_RangeQueryResult{
				Result: &rpc.PromQL_RangeQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Points: []*rpc.PromQL_Point{
									{
										Time:  1000,
										Value: 2.5,
									},
									{
										Time:  2000,
										Value: 3.5,
									},
								},
							},
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {},
							"values": [
								[ 1, "2.5" ],
								[ 2, "3.5" ]
							]
						}
					]
				}
			}`))
		})

		It("handles a series query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			result, err := marshaler.Marshal(&rpc.PromQL_SeriesQueryResult{
				Series: []*rpc.PromQL_SeriesInfo{
					{
						Info: map[string]string{
							"__name__": "up",
							"job":      "prometheus",
							"instance": "localhost:9090",
						},
					},
					{
						Info: map[string]string{
							"__name__": "up",
							"job":      "node",
							"instance": "localhost:9091",
						},
					},
					{
						Info: map[string]string{
							"__name__": "process_start_time_seconds",
							"job":      "prometheus",
							"instance": "localhost:9090",
						},
					},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchJSON(`{
				"status": "success",
				"data": [
					{
						"__name__" : "up",
						"job" : "prometheus",
						"instance" : "localhost:9090"
					},
					{
						"__name__" : "up",
						"job" : "node",
						"instance" : "localhost:9091"
					},
					{
						"__name__" : "process_start_time_seconds",
						"job" : "prometheus",
						"instance" : "localhost:9090"
					}
				]
			}`))
		})
	})

	Context("Unmarshal()", func() {
		It("handles a scalar instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "scalar",
					"result": [1.777, "2.5"]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Scalar{
					Scalar: &rpc.PromQL_Point{
						Time:  convertToMilliseconds(1.777),
						Value: 2.5,
					},
				},
			}))
		})

		It("handles a vector instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"value": [ 1.456, "2.5" ]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"value": [ 2, "3.5" ]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Vector{
					Vector: &rpc.PromQL_Vector{
						Samples: []*rpc.PromQL_Sample{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Point: &rpc.PromQL_Point{
									Time:  convertToMilliseconds(1.456),
									Value: 2.5,
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Point: &rpc.PromQL_Point{
									Time:  convertToMilliseconds(2.000),
									Value: 3.5,
								},
							},
						},
					},
				},
			}))
		})

		It("handles a matrix instant query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1.987, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(rpc.PromQL_InstantQueryResult{
				Result: &rpc.PromQL_InstantQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  convertToMilliseconds(1.987),
										Value: 2.5,
									},
									{
										Time:  convertToMilliseconds(2.000),
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  convertToMilliseconds(1.000),
										Value: 4.5,
									},
									{
										Time:  convertToMilliseconds(2.000),
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			}))
		})

		It("handles a matrix range query result", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_RangeQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name":   "tag-value"
							},
							"values": [
								[ 1.987, "2.5" ],
								[ 2, "3.5" ]
							]
						},
						{
							"metric": {
								"__name__": "metric-name",
								"deployment": "cf",
								"tag-name2":   "tag-value2"
							},
							"values": [
								[ 1, "4.5" ],
								[ 2, "6.5" ]
							]
						}
					]
				}
			}`), &result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(rpc.PromQL_RangeQueryResult{
				Result: &rpc.PromQL_RangeQueryResult_Matrix{
					Matrix: &rpc.PromQL_Matrix{
						Series: []*rpc.PromQL_Series{
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name":   "tag-value",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  convertToMilliseconds(1.987),
										Value: 2.5,
									},
									{
										Time:  convertToMilliseconds(2.000),
										Value: 3.5,
									},
								},
							},
							{
								Metric: map[string]string{
									"__name__":   "metric-name",
									"deployment": "cf",
									"tag-name2":  "tag-value2",
								},
								Points: []*rpc.PromQL_Point{
									{
										Time:  convertToMilliseconds(1.000),
										Value: 4.5,
									},
									{
										Time:  convertToMilliseconds(2.000),
										Value: 6.5,
									},
								},
							},
						},
					},
				},
			}))
		})

		It("returns an error for points with unparsable values", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [ 1.456, "potato" ]
						}
					]
				}
			}`), &result)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for points of the wrong length", func() {
			marshaler := &runtime.JSONPb{OrigName: true, EmitDefaults: true}

			var result rpc.PromQL_InstantQueryResult
			err := marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [ 1.456, "2.5", 2 ]
						}
					]
				}
			}`), &result)
			Expect(err).To(HaveOccurred())

			err = marshaler.Unmarshal([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [ 1.456 ]
						}
					]
				}
			}`), &result)
			Expect(err).To(HaveOccurred())
		})
	})
})

type mockMarshaler struct {
	marshalError   error
	unmarshalError error
	encodeError    error
	decodeError    error
}

func (m *mockMarshaler) Marshal(v interface{}) ([]byte, error) {
	return []byte("mock marshaled result"), m.marshalError
}

func (m *mockMarshaler) Unmarshal(data []byte, v interface{}) error {
	*(v.(*string)) = "mock unmarshaled result"

	return m.unmarshalError
}

func (m *mockMarshaler) NewEncoder(w io.Writer) runtime.Encoder {
	return runtime.EncoderFunc(func(interface{}) error {
		w.Write([]byte("mock encoded result"))

		return m.encodeError
	})
}

func (m *mockMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(v interface{}) error {
		*(v.(*string)) = "mock decoded result"

		return m.decodeError
	})
}

func (m *mockMarshaler) ContentType() string {
	panic("not implemented")
}

func convertToMilliseconds(floatTime float64) int64 {
	return int64(floatTime * float64(time.Second/time.Millisecond))
}

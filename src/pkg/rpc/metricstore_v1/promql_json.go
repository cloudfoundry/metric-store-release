package metricstore_v1

import (
	"errors"
	fmt "fmt"
	"strconv"
	"time"

	"github.com/francoispqt/gojay"
	"github.com/golang/protobuf/jsonpb"
)

func (sqr *PromQL_SeriesQueryResult) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return gojay.MarshalJSONObject(sqr)
}

func (sqr *PromQL_SeriesQueryResult) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("status", "success")
	enc.ArrayKey("data", sqr)
}

func (sqr *PromQL_SeriesQueryResult) IsNil() bool {
	return sqr == nil
}

func (sqr *PromQL_SeriesQueryResult) NKeys() int {
	return 2
}

func (sqr *PromQL_SeriesQueryResult) MarshalJSONArray(enc *gojay.Encoder) {
	for _, series := range sqr.Series {
		enc.Object(series)
	}
}

func (si *PromQL_SeriesInfo) MarshalJSONObject(enc *gojay.Encoder) {
	for k, v := range si.Info {
		enc.StringKey(k, v)
	}
}

func (si *PromQL_SeriesInfo) IsNil() bool {
	return si == nil
}

func (lr *PromQL_LabelsQueryResult) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return gojay.MarshalJSONObject(lr)
}

func (lr *PromQL_LabelsQueryResult) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("status", "success")
	enc.ArrayKey("data", lr)
}

func (lr *PromQL_LabelsQueryResult) IsNil() bool {
	return lr == nil
}

func (lr *PromQL_LabelsQueryResult) MarshalJSONArray(enc *gojay.Encoder) {
	for _, label := range lr.Labels {
		enc.String(label)
	}
}

func (lnr *PromQL_LabelValuesQueryResult) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return gojay.MarshalJSONObject(lnr)
}

func (lnr *PromQL_LabelValuesQueryResult) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("status", "success")
	enc.ArrayKey("data", lnr)
}

func (lnr *PromQL_LabelValuesQueryResult) IsNil() bool {
	return lnr == nil
}

func (lnr *PromQL_LabelValuesQueryResult) MarshalJSONArray(enc *gojay.Encoder) {
	for _, label := range lnr.Values {
		enc.String(label)
	}
}

func (ir *PromQL_InstantQueryResult) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return gojay.MarshalJSONObject(ir)
}

func (ir *PromQL_InstantQueryResult) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("status", "success")

	switch data := ir.Result.(type) {
	case *PromQL_InstantQueryResult_Scalar:
		enc.ObjectKey("data", data)
	case *PromQL_InstantQueryResult_Vector:
		enc.ObjectKey("data", data)
	case *PromQL_InstantQueryResult_Matrix:
		enc.ObjectKey("data", data)
	}
}

func (ir *PromQL_InstantQueryResult) IsNil() bool {
	return ir == nil
}

func (ir *PromQL_InstantQueryResult) UnmarshalJSONPB(m *jsonpb.Unmarshaler, val []byte) error {
	return gojay.UnmarshalJSONObject(val, ir)
}

func (ir *PromQL_InstantQueryResult) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "status":
		return nil
	case "data":
		dataJSON, resultType, err := getDataWithResultType(dec)
		if err != nil {
			return nil
		}

		switch resultType {
		case "scalar":
			data := &PromQL_InstantQueryResult_Scalar{}
			ir.Result = data

			return gojay.UnmarshalJSONObject(dataJSON, data)
		case "vector":
			data := &PromQL_InstantQueryResult_Vector{}
			ir.Result = data

			return gojay.UnmarshalJSONObject(dataJSON, data)
		case "matrix":
			data := &PromQL_InstantQueryResult_Matrix{}
			ir.Result = data

			return gojay.UnmarshalJSONObject(dataJSON, data)
		default:
			return fmt.Errorf(`unknown "resultType": %s`, resultType)
		}
	}

	return nil
}

func (ir *PromQL_InstantQueryResult) NKeys() int {
	return 2
}

type resultType string

func (rt *resultType) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	if key == "resultType" {
		return dec.String((*string)(rt))
	}

	return nil
}

func (rt *resultType) NKeys() int {
	return 1
}

func (rr *PromQL_RangeQueryResult) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return gojay.MarshalJSONObject(rr)
}

func (rr *PromQL_RangeQueryResult) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("status", "success")

	switch data := rr.Result.(type) {
	case *PromQL_RangeQueryResult_Matrix:
		enc.ObjectKey("data", data)
	}
}

func (rr *PromQL_RangeQueryResult) IsNil() bool {
	return rr == nil
}

func (rr *PromQL_RangeQueryResult) UnmarshalJSONPB(m *jsonpb.Unmarshaler, val []byte) error {
	return gojay.UnmarshalJSONObject(val, rr)
}

func (rr *PromQL_RangeQueryResult) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "status":
		return nil
	case "data":
		dataJSON, resultType, err := getDataWithResultType(dec)
		if err != nil {
			return nil
		}

		switch resultType {
		case "matrix":
			data := &PromQL_RangeQueryResult_Matrix{}
			rr.Result = data

			return gojay.UnmarshalJSONObject(dataJSON, data)
		default:
			return fmt.Errorf(`unknown "resultType": %s`, resultType)
		}
	}

	return nil
}

func getDataWithResultType(dec *gojay.Decoder) (dataJSON gojay.EmbeddedJSON, resultType resultType, err error) {
	if err = dec.EmbeddedJSON(&dataJSON); err != nil {
		return
	}

	if err = gojay.UnmarshalJSONObject(dataJSON, &resultType); err != nil {
		return
	}

	return
}

func (rr *PromQL_RangeQueryResult) NKeys() int {
	return 1
}

func (irs *PromQL_InstantQueryResult_Scalar) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resultType", "scalar")
	enc.ArrayKey("result", irs.Scalar)
}

func (irs *PromQL_InstantQueryResult_Scalar) IsNil() bool {
	return irs == nil
}

func (irs *PromQL_InstantQueryResult_Scalar) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	if key == "result" {
		var err error
		irs.Scalar, err = decodePoint(dec)
		return err
	}

	return nil
}

func (irs *PromQL_InstantQueryResult_Scalar) NKeys() int {
	return 1
}

func (irv *PromQL_InstantQueryResult_Vector) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resultType", "vector")
	enc.ArrayKey("result", irv.Vector)
}

func (irv *PromQL_InstantQueryResult_Vector) IsNil() bool {
	return irv == nil
}

func (irv *PromQL_InstantQueryResult_Vector) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	if key == "result" {
		irv.Vector = &PromQL_Vector{}
		return dec.Array(irv.Vector)
	}

	return nil
}

func (irv *PromQL_InstantQueryResult_Vector) NKeys() int {
	return 1
}

func (v *PromQL_Vector) MarshalJSONArray(enc *gojay.Encoder) {
	for _, s := range v.Samples {
		enc.Object(s)
	}
}

func (v *PromQL_Vector) IsNil() bool {
	return v == nil
}

func (v *PromQL_Vector) UnmarshalJSONArray(dec *gojay.Decoder) error {
	sample := &PromQL_Sample{}

	if err := dec.Object(sample); err != nil {
		return err
	}

	v.Samples = append(v.Samples, sample)

	return nil
}

func (s *PromQL_Sample) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("metric", metricMap(s.Metric))
	enc.ArrayKey("value", s.Point)
}

func (s *PromQL_Sample) IsNil() bool {
	return s == nil
}

func (s *PromQL_Sample) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "metric":
		s.Metric = make(map[string]string)
		return dec.Object(metricMap(s.Metric))
	case "value":
		var err error
		s.Point, err = decodePoint(dec)
		return err
	}

	return nil
}

func (s *PromQL_Sample) NKeys() int {
	return 2
}

func (irm *PromQL_InstantQueryResult_Matrix) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resultType", "matrix")
	enc.ArrayKey("result", irm.Matrix)
}

func (irm *PromQL_InstantQueryResult_Matrix) IsNil() bool {
	return irm == nil
}

func (irm *PromQL_InstantQueryResult_Matrix) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	if key == "result" {
		irm.Matrix = &PromQL_Matrix{}
		return dec.Array(irm.Matrix)
	}

	return nil
}

func (irm *PromQL_InstantQueryResult_Matrix) NKeys() int {
	return 1
}

func (rrm *PromQL_RangeQueryResult_Matrix) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resultType", "matrix")
	enc.ArrayKey("result", rrm.Matrix)
}

func (rrm *PromQL_RangeQueryResult_Matrix) IsNil() bool {
	return rrm == nil
}

func (rrm *PromQL_RangeQueryResult_Matrix) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	if key == "result" {
		rrm.Matrix = &PromQL_Matrix{}
		return dec.Array(rrm.Matrix)
	}

	return nil
}

func (rrm *PromQL_RangeQueryResult_Matrix) NKeys() int {
	return 1
}

func (m *PromQL_Matrix) MarshalJSONArray(enc *gojay.Encoder) {
	for _, s := range m.Series {
		enc.Object(s)
	}
}

func (m *PromQL_Matrix) IsNil() bool {
	return m == nil
}

func (m *PromQL_Matrix) UnmarshalJSONArray(dec *gojay.Decoder) error {
	series := &PromQL_Series{}
	if err := dec.Object(series); err != nil {
		return err
	}

	m.Series = append(m.Series, series)

	return nil
}

func (s *PromQL_Series) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("metric", metricMap(s.Metric))
	enc.ArrayKey("values", s)
}

func (s *PromQL_Series) MarshalJSONArray(enc *gojay.Encoder) {
	for _, p := range s.Points {
		enc.Array(p)
	}
}

func (s *PromQL_Series) IsNil() bool {
	return s == nil
}

func (s *PromQL_Series) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "metric":
		s.Metric = make(map[string]string)
		return dec.Object(metricMap(s.Metric))
	case "values":
		return dec.Array(s)
	}

	return nil
}

func (s *PromQL_Series) UnmarshalJSONArray(dec *gojay.Decoder) error {
	point, err := decodePoint(dec)

	if err != nil {
		return err
	}

	s.Points = append(s.Points, point)
	return nil
}

func (s *PromQL_Series) NKeys() int {
	return 2
}

type metricMap map[string]string

func (m metricMap) MarshalJSONObject(enc *gojay.Encoder) {
	for k, v := range m {
		enc.StringKey(k, v)
	}
}

func (m metricMap) IsNil() bool {
	return m == nil
}

func (m metricMap) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	var value string

	if err := dec.String(&value); err != nil {
		return err
	}

	m[key] = value
	return nil
}

func (m metricMap) NKeys() int {
	return 0
}

func (p *PromQL_Point) MarshalJSONArray(enc *gojay.Encoder) {
	enc.Float64(float64(p.Time) / float64(time.Second/time.Millisecond))
	enc.String(strconv.FormatFloat(p.Value, 'f', -1, 64))
}

func (p *PromQL_Point) IsNil() bool {
	return p == nil
}

func decodePoint(dec *gojay.Decoder) (*PromQL_Point, error) {
	scalar := pointWithPosition{}
	if err := dec.Array(&scalar); err != nil {
		return nil, err
	}

	if scalar.position != 2 {
		return nil, errors.New("point too short, expected numeric time and string value")
	}

	return &scalar.PromQL_Point, nil
}

type pointWithPosition struct {
	PromQL_Point
	position int
}

func (pp *pointWithPosition) UnmarshalJSONArray(dec *gojay.Decoder) error {
	switch pp.position {
	case 0:
		var decimalTime float64
		if err := dec.Float64(&decimalTime); err != nil {
			return err
		}
		pp.Time = int64(decimalTime * float64(time.Second/time.Millisecond))
		pp.position++
	case 1:
		var stringValue string
		if err := dec.String(&stringValue); err != nil {
			return err
		}

		var err error
		pp.Value, err = strconv.ParseFloat(stringValue, 64)
		if err != nil {
			return fmt.Errorf("failed to parse value: %q", err)
		}
		pp.position++
	default:
		return errors.New("point too long, expected only numeric time and string value")
	}

	return nil
}

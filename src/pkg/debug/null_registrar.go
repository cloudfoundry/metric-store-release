package debug

type NullRegistrar struct {
}

func (*NullRegistrar) Set(string, float64) {
}

func (*NullRegistrar) Inc(string) {
}

func (*NullRegistrar) Add(string, float64) {
}

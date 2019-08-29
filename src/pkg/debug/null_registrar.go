package debug

type NullRegistrar struct {
}

func (*NullRegistrar) Set(string, float64, ...string) {
}

func (*NullRegistrar) Inc(string, ...string) {
}

func (*NullRegistrar) Add(string, float64, ...string) {
}

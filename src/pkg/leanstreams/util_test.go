package leanstreams

import "testing"

func TestMessageBytesToInt(t *testing.T) {
	cases := []struct {
		input, output int64
	}{
		{1, 1},
		{2, 2},
		{4, 4},
		{16, 16},
		{32, 32},
		{64, 64},
		{128, 128},
		{256, 256},
		{1024, 1024},
		{2048, 2048},
		{4096, 4096},
		{8192, 8192},
		{17, 17},
		{456, 456},
		{24569045, 24569045},
	}

	for _, c := range cases {
		bytes := int64ToByteArray(c.input, headerByteSize)
		result, _ := byteArrayToInt64(bytes)
		if result != c.output {
			t.Errorf("Conversion between bytes incorrect. Original value %d, got %d", c.input, result)
		}
	}
}

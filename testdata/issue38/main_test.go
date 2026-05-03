package issue38

import "testing"

type Framework struct {
	m *testing.M
}

func (f *Framework) EtcdMain(action func() int) {
	action()
}

func TestMain(m *testing.M) {
	var framework Framework
	framework.EtcdMain(m.Run)
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name string
		arg  int
		want uint
	}{
		{"positive", 1, 1},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Abs(tt.arg); got != tt.want {
				t.Errorf("Abs() = %v, want %v", got, tt.want)
			}
		})
	}
}

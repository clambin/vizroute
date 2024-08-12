package ui

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGradient(t *testing.T) {
	type args struct {
		value   float64
		maximum float64
		length  int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "minimum",
			args: args{value: 0, maximum: 10, length: 12},
			want: "|----------|",
		},
		{
			name: "maximum",
			args: args{value: 10, maximum: 10, length: 12},
			want: "|**********|",
		},
		{
			name: "ceiling",
			args: args{value: 0.1, maximum: 10, length: 12},
			want: "|*---------|",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Gradient(tt.args.value, tt.args.maximum, tt.args.length))
		})
	}
}

func TestGradient_length(t *testing.T) {
	const maxValue float64 = 100
	const resolution float64 = 10000
	const length = 15
	for v := range int(maxValue * resolution) {
		assert.Len(t, Gradient(float64(v)/resolution, maxValue, length), length)
	}
}

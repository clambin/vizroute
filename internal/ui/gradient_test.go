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

package utils

import "testing"

func TestGorillaPathToOasPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/transactions/get/{tx_id:[0-9a-zA-Z]+}", "/transactions/get/{tx_id}"},
		{"/transactions/list/{epoch:[0-9]+}", "/transactions/list/{epoch}"},
		{"/mirroring/tx_data/{tx_id:[0-9a-zA-Z]+}", "/mirroring/tx_data/{tx_id}"},
		{"/a/{b}/c", "/a/{b}/c"},
		{"/a/{b:[0-9]{3}}/{c}", "/a/{b}/{c}"},
		{"/a/{b:[0-9]+}/{c:[a-z]+}", "/a/{b}/{c}"},
		{"/validators/list", "/validators/list"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := gorillaPathToOasPath(tt.in); got != tt.want {
			t.Errorf("gorillaPathToOasPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

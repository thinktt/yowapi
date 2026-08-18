package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/thinktt/yowapi/pkg/utils"
)

func TestMoveResponseRetriesOnlyDatabaseHTTPError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "database error",
			err:  utils.NewHTTPError(http.StatusInternalServerError, "DB Error: unavailable"),
			want: true,
		},
		{
			name: "invalid move",
			err:  utils.NewHTTPError(http.StatusBadRequest, "invalid move index"),
			want: false,
		},
		{
			name: "malformed stored game",
			err:  utils.NewHTTPError(http.StatusInternalServerError, "Error parsing db game"),
			want: false,
		},
		{
			name: "raw infrastructure error",
			err:  errors.New("mongo unavailable"),
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isRetryableMoveResponseError(test.err)
			if got != test.want {
				t.Fatalf("isRetryableMoveResponseError() = %t, want %t", got, test.want)
			}
		})
	}
}

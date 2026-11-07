package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"chameth.com/goplum/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResultStream struct {
	api.GoPlum_ResultsClient
	results []*api.Result
	err     error
}

func (f *fakeResultStream) Recv() (*api.Result, error) {
	if len(f.results) == 0 {
		return nil, f.err
	}
	res := f.results[0]
	f.results = f.results[1:]
	return res, nil
}

func TestPrintResults_endOfStreamIsNotAnError(t *testing.T) {
	result := &api.Result{Check: "web"}
	stream := &fakeResultStream{results: []*api.Result{result}, err: io.EOF}

	out := &bytes.Buffer{}
	require.NoError(t, printResults(stream, out))
	assert.Equal(t, fmt.Sprintf("%v\n", result), out.String())
}

func TestPrintResults_returnsStreamErrors(t *testing.T) {
	failure := errors.New("connection reset")
	stream := &fakeResultStream{err: failure}

	require.ErrorIs(t, printResults(stream, &bytes.Buffer{}), failure)
}

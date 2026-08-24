package httpui

import "io"

func copyResponse(destination io.Writer, source io.Reader) (int64, error) {
	return io.Copy(destination, source)
}

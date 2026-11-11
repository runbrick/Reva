package request

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/runbrick/reqo/internal/model"
)

type bodyFactory func() (io.ReadCloser, string, error)

func newBodyFactory(body model.Body) bodyFactory {
	boundary := ""
	if body.Mode == model.BodyFormData {
		boundary = multipart.NewWriter(io.Discard).Boundary()
	}
	return func() (io.ReadCloser, string, error) {
		switch body.Mode {
		case "", model.BodyNone:
			return nil, "", nil
		case model.BodyJSON:
			return io.NopCloser(strings.NewReader(body.Text)), "application/json", nil
		case model.BodyRaw:
			contentType := body.ContentType
			if contentType == "" {
				contentType = "text/plain"
			}
			return io.NopCloser(strings.NewReader(body.Text)), contentType, nil
		case model.BodyURLEncoded:
			values := make(url.Values)
			for _, row := range body.URLEncoded {
				if !row.Disabled {
					values.Add(row.Key, row.Value)
				}
			}
			return io.NopCloser(strings.NewReader(values.Encode())), "application/x-www-form-urlencoded", nil
		case model.BodyBinary:
			file, err := openUpload(body.FilePath)
			if err != nil {
				return nil, "", err
			}
			return file, "application/octet-stream", nil
		case model.BodyFormData:
			return multipartBody(body.Form, boundary)
		default:
			return nil, "", fmt.Errorf("unsupported body mode %q", body.Mode)
		}
	}
}

func openUpload(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open upload file %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("stat upload file %q: %w", path, err)
		}
		return nil, fmt.Errorf("upload file %q is not a regular file", path)
	}
	return file, nil
}

// multipartBody buffers only the multipart framing and text fields. File
// contents are read directly by the transport, without a producer goroutine.
func multipartBody(fields []model.FormField, boundary string) (io.ReadCloser, string, error) {
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, "", err
	}
	stream := &uploadStream{}
	var readers []io.Reader
	flush := func() {
		if framing.Len() > 0 {
			readers = append(readers, strings.NewReader(framing.String()))
			framing.Reset()
		}
	}
	for _, field := range fields {
		if field.Disabled {
			continue
		}
		if !field.File {
			if err := writer.WriteField(field.Key, field.Value); err != nil {
				stream.Close()
				return nil, "", err
			}
			continue
		}
		file, err := openUpload(field.Value)
		if err != nil {
			stream.Close()
			return nil, "", err
		}
		stream.files = append(stream.files, file)
		if _, err := writer.CreateFormFile(field.Key, filepath.Base(field.Value)); err != nil {
			stream.Close()
			return nil, "", err
		}
		flush()
		readers = append(readers, file)
	}
	if err := writer.Close(); err != nil {
		stream.Close()
		return nil, "", err
	}
	flush()
	stream.Reader = io.MultiReader(readers...)
	return stream, writer.FormDataContentType(), nil
}

type uploadStream struct {
	io.Reader
	files []*os.File
}

func (s *uploadStream) Close() error {
	var err error
	for _, file := range s.files {
		err = errors.Join(err, file.Close())
	}
	return err
}

package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/catalog"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
)

func productThumbnail(data []byte) ([]byte, uint32, uint32, error) {
	if err := catalog.ValidateImage(data); err != nil {
		return nil, 0, 0, err
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	tw, th := w, h
	if w >= h && w > 320 {
		tw, th = 320, max(1, h*320/w)
	} else if h > 320 {
		tw, th = max(1, w*320/h), 320
	}
	thumb := image.NewRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			r, g, b, a := source.At(bounds.Min.X+x*w/tw, bounds.Min.Y+y*h/th).RGBA()
			// Composite transparent PNG pixels over white before JPEG encoding.
			thumb.SetRGBA(x, y, color.RGBA{uint8((r + 65535 - a) >> 8), uint8((g + 65535 - a) >> 8), uint8((b + 65535 - a) >> 8), 255})
		}
	}
	var output bytes.Buffer
	err = jpeg.Encode(&output, thumb, &jpeg.Options{Quality: 75})
	return output.Bytes(), uint32(w), uint32(h), err
}

type mediaLimitKey struct{}
type boundedMediaTransport struct{ base http.RoundTripper }

func (t boundedMediaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	limit, limited := req.Context().Value(mediaLimitKey{}).(int64)
	if !limited {
		return resp, nil
	}
	if resp.ContentLength > limit {
		resp.Body.Close()
		return nil, errors.New("catalog media exceeds limit")
	}
	resp.Body = &boundedMediaBody{ReadCloser: resp.Body, remaining: limit}
	return resp, nil
}

type boundedMediaBody struct {
	io.ReadCloser
	remaining int64
}

func (r *boundedMediaBody) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		var extra [1]byte
		n, err := r.ReadCloser.Read(extra[:])
		if n != 0 {
			return 0, errors.New("catalog media exceeds limit")
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func (s *Service) downloadProductImage(ctx context.Context, product *waE2E.ProductMessage) ([]byte, error) {
	image := product.GetProduct().GetProductImage()
	if image == nil || image.GetFileLength() == 0 || image.GetFileLength() > catalog.MaxImageBytes {
		return nil, errors.New("missing or oversized catalog image")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// AES padding and MAC add a small amount to the encrypted wire payload.
	ctx = context.WithValue(ctx, mediaLimitKey{}, int64(catalog.MaxImageBytes+64))
	download := s.downloadOverride
	if download == nil {
		download = s.client.Download
	}
	data, err := download(ctx, image)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > catalog.MaxImageBytes {
		return nil, errors.New("catalog image exceeds limit")
	}
	return data, nil
}

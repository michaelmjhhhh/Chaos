package desktop

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func coreProviderLabel(provider, model string) string {
	p := core.ProviderByID(provider)
	if model == "" {
		model = p.Model
	}
	return p.Name + " · " + model
}

func (d *Desktop) requestThumbnail(path string) {
	if path == "" || d.actions.Dispatch == nil || d.thumbnailPending[path] || d.thumbnails[path] != nil {
		return
	}
	d.thumbnailPending[path] = true
	dispatch, loader := d.actions.Dispatch, d.actions.LoadThumbnail
	go func() {
		d.thumbnailSlots <- struct{}{}
		defer func() { <-d.thumbnailSlots }()
		var data []byte
		var err error
		if loader != nil {
			data, err = loader(path)
		} else {
			data, err = readThumbnail(path)
		}
		var bitmap *ui.Bitmap
		if err == nil {
			bitmap, err = ui.DecodeBitmap(data)
		}
		dispatch(func() {
			// Remember failed decodes too: unsupported/missing files must not retry every frame.
			if err == nil {
				d.thumbnails[path] = bitmap
			}
			d.thumbnailOrder = append(d.thumbnailOrder, path)
			if len(d.thumbnailOrder) > 64 {
				old := d.thumbnailOrder[0]
				d.thumbnailOrder = d.thumbnailOrder[1:]
				delete(d.thumbnails, old)
				delete(d.thumbnailPending, old)
			}
		})
	}()
}
func readThumbnail(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prepared, err := core.PrepareImage(ctx, path)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(prepared.Base64)
}

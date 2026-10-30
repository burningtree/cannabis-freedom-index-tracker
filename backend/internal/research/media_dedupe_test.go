package research

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// a test picture: a gradient with a bright block, at a size; pattern changes the scene
func picture(w, h, pattern int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*255/w + y*80/h*pattern) % 256)
			if pattern == 2 {
				v = uint8(255 - (y*255/h+x*40/w)%256)
			}
			if x > w/3 && x < w/2 && y > h/4 && y < h*3/4 {
				v = 250
			}
			img.Set(x, y, color.RGBA{v, v / 2, 255 - v, 255})
		}
	}
	return img
}

func TestDedupeMediaDropsTheSamePictureFromAnotherOutlet(t *testing.T) {
	encode := func(img image.Image, asPNG bool) []byte {
		var b bytes.Buffer
		if asPNG {
			png.Encode(&b, img)
		} else {
			jpeg.Encode(&b, img, &jpeg.Options{Quality: 60})
		}
		return b.Bytes()
	}
	files := map[string][]byte{
		"https://a.example/police.jpg":     encode(picture(1200, 800, 1), false),
		"https://b.example/police-640.png": encode(picture(640, 427, 1), true), // the same photo, smaller and another format
		"https://c.example/field.jpg":      encode(picture(1200, 800, 2), false),
		"https://d.example/existing.jpg":   encode(picture(800, 600, 2), false),
	}
	fetch := func(_ context.Context, u string) ([]byte, error) {
		if b, ok := files[u]; ok {
			return b, nil
		}
		return nil, errors.New("not found")
	}
	media := []Media{
		{Kind: "photo", MediaURL: "https://a.example/police.jpg", Caption: "Seized drugs"},
		{Kind: "photo", MediaURL: "https://b.example/police-640.png", Caption: "Drugs seized"},
		{Kind: "photo", MediaURL: "https://a.example/police.jpg", Caption: "Same address again"},
		{Kind: "photo", MediaURL: "https://c.example/field.jpg", Caption: "The field"},
		{Kind: "photo", MediaURL: "https://e.example/unreachable.jpg", Caption: "Cannot be compared"},
		{Kind: "video", MediaURL: "https://youtu.be/dQw4w9WgXcQ", Caption: "Video"},
	}
	kept, dropped := DedupeMedia(context.Background(), media, nil, fetch)
	if len(kept) != 4 || len(dropped) != 2 || kept[0].Caption != "Seized drugs" || kept[1].Caption != "The field" {
		t.Fatal(kept, dropped)
	}
	// a photo the existing report already shows is not added again
	kept, dropped = DedupeMedia(context.Background(), media[3:4], []string{"https://d.example/existing.jpg"}, fetch)
	if len(kept) != 0 || len(dropped) != 1 {
		t.Fatal("existing photo added again", kept)
	}
}

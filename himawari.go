// Package himawari is a Go port of boramalper/himawaripy - a client for downloading
// Himawari-8 geo-stationary satellite images.
package himawari

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"sync"
	"time"
)

const (
	// Default zoom level.
	DefaultLevel = 4
	// Width of each chunk.
	Width = 550
	// Height of each chunk.
	Height = Width
)

// Supported zoom levels. Higher level increases quality and size of the image.
// Size of the resulting image is expected to be level*W x level*H.
var Levels = []int{1, 2, 4, 8, 16, 20}

// Latest returns a timestamp of latest image available.
func Latest() (time.Time, error) {
	resp, err := http.Get("http://himawari8-dl.nict.go.jp/himawari8/img/D531106/latest.json")
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	var r struct {
		Date string `json:"date"`
		//File string `json:"file"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return time.Time{}, err
	}
	latest, err := time.Parse("2006-01-02 15:04:05", r.Date)
	if err != nil {
		return time.Time{}, err
	}
	return latest, nil
}

func TimeWithOffset(latest time.Time) time.Time {
	local := time.Now()
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		panic(err)
	}
	hima := local.In(loc)
	_, localOff := local.Zone()
	_, himaOff := hima.Zone()
	offset := localOff - himaOff
	return latest.Add(time.Second * time.Duration(offset))
}

// ChunkUrl returns an URL for a given chunk at specific time.
func ChunkUrl(t time.Time, level int, width int, x, y int) string {
	return fmt.Sprintf("http://himawari8.nict.go.jp/img/D531106/%dd/%d/%s_%d_%d.png",
		level, width, t.UTC().Format("2006/01/02/150405"), x, y)
}

// Chunk returns a decoded chunk image at specific time.
func Chunk(t time.Time, level int, x, y int) (image.Image, error) {
	resp, err := http.Get(ChunkUrl(t, level, Width, x, y))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return png.Decode(resp.Body)
}

// Workers is a number of goroutines that Image will use to load chunks concurrently.
var Workers = 5

// Image loads a whole satellite image for a given time and zoom level.
//
// If level = 0, default zoom level will be used.
func Image(t time.Time, level int) (image.Image, error) {
	if level <= 0 {
		level = DefaultLevel
	}
	if level == 1 {
		return Chunk(t, level, 0, 0)
	}
	workers := Workers
	if workers <= 0 {
		workers = 1
	} else if total := level * level; workers > total {
		workers = total
	}
	canvas := image.NewRGBA(image.Rect(0, 0, level*Width, level*Height))
	draw := func(x, y int, img image.Image) {
		draw.Draw(canvas, image.Rect(x*Width, y*Height, (x+1)*Width, (y+1)*Height), img, image.ZP, draw.Src)
	}

	if workers == 1 {
		for y := 0; y < level; y++ {
			for x := 0; x < level; x++ {
				img, err := Chunk(t, level, x, y)
				if err != nil {
					return nil, err
				}
				draw(x, y, img)
			}
		}
		return canvas, nil
	}
	chunks := make(chan [2]int)
	errc := make(chan error, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range chunks {
				x, y := c[0], c[1]
				img, err := Chunk(t, level, x, y)
				if err != nil {
					errc <- err
					return
				}
				draw(x, y, img)
			}
		}()
	}
	for y := 0; y < level; y++ {
		for x := 0; x < level; x++ {
			select {
			case chunks <- [2]int{x, y}:
			case err := <-errc:
				close(chunks)
				return canvas, err
			}
		}
	}
	close(chunks)
	wg.Wait()
	close(errc)
	return canvas, <-errc
}

// LatestImage loads most recent satellite image for a given zoom level.
//
// If level = 0, default zoom level will be used.
//
// offsetTime parameter can be set to true to correct time to local time zone.
func LatestImage(level int, offsetTime bool) (image.Image, error) {
	if level <= 0 {
		level = DefaultLevel
	}
	t, err := Latest()
	if err != nil {
		return nil, err
	}
	if offsetTime {
		t = TimeWithOffset(t)
	}
	return Image(t, level)
}
